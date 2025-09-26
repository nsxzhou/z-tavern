package speech

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zhouzirui/z-tavern/backend/internal/model/speech"
)

// VolcengineASRClient 火山引擎ASR WebSocket客户端
type VolcengineASRClient struct {
	config *speech.SpeechConfig
	dialer *websocket.Dialer
}

type asrServerMessage struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	Sequence int    `json:"sequence"`
	Result   struct {
		Text       string `json:"text"`
		Utterances []struct {
			Text      string `json:"text"`
			StartTime int64  `json:"start_time"`
			EndTime   int64  `json:"end_time"`
			Definite  bool   `json:"definite"`
		} `json:"utterances,omitempty"`
	} `json:"result,omitempty"`
	AudioInfo struct {
		Duration int64 `json:"duration"`
	} `json:"audio_info,omitempty"`
}

// NewVolcengineASRClient 创建火山引擎ASR客户端
func NewVolcengineASRClient(config *speech.SpeechConfig) *VolcengineASRClient {
	return &VolcengineASRClient{
		config: config,
		dialer: &websocket.Dialer{
			HandshakeTimeout: 30 * time.Second,
		},
	}
}

// ASRRequest 火山引擎ASR请求结构（按文档格式）
type ASRRequest struct {
	User struct {
		UID        string `json:"uid,omitempty"`
		DID        string `json:"did,omitempty"`
		Platform   string `json:"platform,omitempty"`
		SDKVersion string `json:"sdk_version,omitempty"`
		AppVersion string `json:"app_version,omitempty"`
	} `json:"user,omitempty"`
	Audio struct {
		Language string `json:"language,omitempty"`
		Format   string `json:"format"`
		Codec    string `json:"codec,omitempty"`
		Rate     int    `json:"rate,omitempty"`
		Bits     int    `json:"bits,omitempty"`
		Channel  int    `json:"channel,omitempty"`
	} `json:"audio"`
	Request struct {
		ModelName            string `json:"model_name"`
		EnableITN            bool   `json:"enable_itn,omitempty"`
		EnablePunc           bool   `json:"enable_punc,omitempty"`
		EnableDDC            bool   `json:"enable_ddc,omitempty"`
		ShowUtterances       bool   `json:"show_utterances,omitempty"`
		ResultType           string `json:"result_type,omitempty"`
		EnableAccelerateText bool   `json:"enable_accelerate_text,omitempty"`
		AccelerateScore      int    `json:"accelerate_score,omitempty"`
		VADSegmentDuration   int    `json:"vad_segment_duration,omitempty"`
		EndWindowSize        int    `json:"end_window_size,omitempty"`
		ForceToSpeechTime    int    `json:"force_to_speech_time,omitempty"`
		EnableNonstream      bool   `json:"enable_nonstream,omitempty"`
	} `json:"request"`
}

// ASRResponse 火山引擎ASR响应结构
type ASRResponse struct {
	Result struct {
		Text       string `json:"text"`
		Utterances []struct {
			Text      string `json:"text"`
			StartTime int64  `json:"start_time"`
			EndTime   int64  `json:"end_time"`
			Definite  bool   `json:"definite"`
			Words     []struct {
				Text          string `json:"text"`
				StartTime     int64  `json:"start_time"`
				EndTime       int64  `json:"end_time"`
				BlankDuration int64  `json:"blank_duration"`
			} `json:"words,omitempty"`
		} `json:"utterances,omitempty"`
	} `json:"result,omitempty"`
	AudioInfo struct {
		Duration int64 `json:"duration"`
	} `json:"audio_info,omitempty"`
}

// TranscribeAudioWS 使用WebSocket协议进行语音识别
func (c *VolcengineASRClient) TranscribeAudioWS(ctx context.Context, req *speech.ASRRequest) (*speech.ASRResponse, error) {
	// 选择合适的WebSocket端点
	var wsURL string
	if c.isStreamingMode() {
		// 双向流式模式（优化版本）
		wsURL = "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_async"
	} else {
		// 流式输入模式
		wsURL = "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_nostream"
	}

	appID, token, err := resolveCredentials(c.config)
	if err != nil {
		return nil, err
	}

	// 设置请求头（按ASR API文档格式）
	header := http.Header{}
	header.Set("X-Api-App-Key", appID)
	header.Set("X-Api-Access-Key", token)

	// 设置资源ID
	resourceID := "volc.bigasr.sauc.duration" // 小时版
	if c.config.ConcurrentMode {
		resourceID = "volc.bigasr.sauc.concurrent" // 并发版
	}
	header.Set("X-Api-Resource-Id", resourceID)
	header.Set("X-Api-Connect-Id", req.SessionID) // 使用sessionID作为连接ID

	// 建立WebSocket连接
	conn, resp, err := c.dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ASR WebSocket: %w", err)
	}
	defer conn.Close()

	// 检查连接头中的logid
	if logid := resp.Header.Get("X-Tt-Logid"); logid != "" {
		log.Printf("[ASR] Connected with logid: %s", logid)
	}

	// 构建ASR请求
	asrReq := c.buildASRRequest(req)

	// 序列化请求为JSON
	payloadData, err := json.Marshal(asrReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ASR request: %w", err)
	}

	// 压缩payload
	compressedPayload, err := CompressPayload(payloadData, GzipCompression)
	if err != nil {
		return nil, fmt.Errorf("failed to compress payload: %w", err)
	}

	// 发送full client request
	message := CreateFullClientRequest(compressedPayload, GzipCompression)
	messageBytes, err := EncodeMessage(message)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, messageBytes); err != nil {
		return nil, fmt.Errorf("failed to send ASR request: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 并发接收识别结果，确保实时消费服务端反馈
	respCh := make(chan *speech.ASRResponse, 1)
	recvErrCh := make(chan error, 1)
	go func() {
		resp, err := c.receiveASRResults(ctx, conn, req.SessionID)
		if err != nil {
			recvErrCh <- err
			return
		}
		respCh <- resp
	}()

	// 并发发送音频流，这样当服务端提前返回错误时可以及时取消发送
	sendErrCh := make(chan error, 1)
	go func() {
		sendErrCh <- c.sendAudioData(ctx, conn, req)
	}()

	var sendDone bool

	for {
		select {
		case err := <-sendErrCh:
			sendDone = true
			if err != nil {
				cancel()
				return nil, fmt.Errorf("failed to send audio data: %w", err)
			}
		case resp := <-respCh:
			cancel()
			return resp, nil
		case err := <-recvErrCh:
			cancel()
			return nil, err
		case <-ctx.Done():
			if !sendDone {
				return nil, ctx.Err()
			}
		}
	}
}

// buildASRRequest 构建符合火山引擎API格式的ASR请求
func (c *VolcengineASRClient) buildASRRequest(req *speech.ASRRequest) *ASRRequest {
	asrReq := &ASRRequest{}

	// User配置
	asrReq.User.UID = req.SessionID

	// Audio配置
	asrReq.Audio.Format = req.Format
	if asrReq.Audio.Format == "" {
		asrReq.Audio.Format = "wav"
	}

	asrReq.Audio.Language = req.Language
	if asrReq.Audio.Language == "" {
		asrReq.Audio.Language = "zh-CN"
	}

	asrReq.Audio.Codec = "raw" // 默认为raw (PCM)
	asrReq.Audio.Rate = 16000  // 默认采样率
	asrReq.Audio.Bits = 16     // 默认位数
	asrReq.Audio.Channel = 1   // 默认单声道

	// Request配置
	asrReq.Request.ModelName = "bigmodel"
	asrReq.Request.EnableITN = true      // 启用文本规范化
	asrReq.Request.EnablePunc = true     // 启用标点
	asrReq.Request.ShowUtterances = true // 显示分句信息
	asrReq.Request.ResultType = "full"   // 全量返回结果
	asrReq.Request.EndWindowSize = 800   // 强制判停时间800ms

	return asrReq
}

// sendAudioData 发送音频数据
func (c *VolcengineASRClient) sendAudioData(ctx context.Context, conn *websocket.Conn, req *speech.ASRRequest) error {
	// 读取所有音频数据
	audioData := make([]byte, 0)
	buf := make([]byte, 1024)
	for {
		n, err := req.AudioData.Read(buf)
		if n > 0 {
			audioData = append(audioData, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	if len(audioData) == 0 {
		return fmt.Errorf("no audio data to send")
	}

	// 将音频数据分包发送（每包约200ms的音频）
	chunkSize := 6400    // 16kHz, 16bit, mono, 200ms = 6400 bytes
	sequence := int32(2) // 服务端FullClientRequest占用序号1，音频从2开始

	for i := 0; i < len(audioData); i += chunkSize {
		end := i + chunkSize
		if end > len(audioData) {
			end = len(audioData)
		}

		chunk := audioData[i:end]
		isLast := (end >= len(audioData))

		// 创建audio only request
		compressedChunk, err := CompressPayload(chunk, GzipCompression)
		if err != nil {
			return fmt.Errorf("failed to compress audio chunk: %w", err)
		}

		audioMsg := CreateAudioOnlyRequest(compressedChunk, sequence, isLast, GzipCompression)
		msgBytes, err := EncodeMessage(audioMsg)
		if err != nil {
			return fmt.Errorf("failed to encode audio message: %w", err)
		}

		if err := conn.WriteMessage(websocket.BinaryMessage, msgBytes); err != nil {
			return fmt.Errorf("failed to send audio chunk: %w", err)
		}

		sequence++

		// 如果是最后一包，退出循环
		if isLast {
			break
		}

		// 控制发送速率，模拟实时音频流
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond): // 200ms间隔
		}
	}

	return nil
}

// receiveASRResults 接收ASR识别结果
func (c *VolcengineASRClient) receiveASRResults(ctx context.Context, conn *websocket.Conn, sessionID string) (*speech.ASRResponse, error) {
	var (
		finalText string
		duration  int64
	)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			_, data, err := conn.ReadMessage()
			if err != nil {
				return nil, fmt.Errorf("failed to read ASR response: %w", err)
			}

			msg, err := DecodeMessage(bytes.NewReader(data))
			if err != nil {
				return nil, fmt.Errorf("failed to decode ASR message: %w", err)
			}

			switch msg.Header.MessageType {
			case ErrorMessage:
				payload, err := DecompressPayload(msg.Payload, msg.Header.CompressionMethod)
				if err != nil {
					return nil, fmt.Errorf("ASR error message decode failed: %w", err)
				}
				return nil, fmt.Errorf("ASR error: %s", string(payload))

			case FullServerResponse:
				payload, err := DecompressPayload(msg.Payload, msg.Header.CompressionMethod)
				if err != nil {
					return nil, fmt.Errorf("failed to decompress ASR payload: %w", err)
				}

				var serverResp asrServerMessage
				if err := json.Unmarshal(payload, &serverResp); err != nil {
					log.Printf("[ASR] failed to unmarshal response: %v", err)
					continue
				}

				if serverResp.Code != 0 && serverResp.Code != 20000000 {
					return nil, fmt.Errorf("ASR API error %d: %s", serverResp.Code, serverResp.Message)
				}

				textCandidate := serverResp.Result.Text
				if textCandidate == "" && len(serverResp.Result.Utterances) > 0 {
					textCandidate = joinUtterances(serverResp.Result.Utterances)
				}
				if textCandidate != "" {
					finalText = textCandidate
				}

				if serverResp.AudioInfo.Duration > 0 {
					duration = serverResp.AudioInfo.Duration
				}

				if msg.IsLastPacket() || serverResp.Sequence < 0 {
					if finalText == "" {
						log.Printf("[ASR] empty transcript for session %s", sessionID)
					}
					return &speech.ASRResponse{
						SessionID:  sessionID,
						Text:       finalText,
						Confidence: estimateASRConfidence(finalText),
						Duration:   duration,
						RequestID:  sessionID,
						CreatedAt:  time.Now(),
					}, nil
				}

			default:
				// 其他类型（如音频ACK）直接忽略
			}
		}
	}
}

func joinUtterances(utterances []struct {
	Text      string `json:"text"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
	Definite  bool   `json:"definite"`
}) string {
	var builder strings.Builder
	for _, u := range utterances {
		if builder.Len() > 0 {
			builder.WriteString(" ")
		}
		builder.WriteString(u.Text)
	}
	return builder.String()
}

func estimateASRConfidence(text string) float64 {
	if strings.TrimSpace(text) == "" {
		return 0
	}
	return 0.95
}

// isStreamingMode 判断是否使用流式模式
func (c *VolcengineASRClient) isStreamingMode() bool {
	// 根据配置决定使用哪种模式，这里默认使用流式输入模式（准确率更高）
	return false
}
