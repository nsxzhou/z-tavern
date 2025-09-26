package speech

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/zhouzirui/z-tavern/backend/internal/model/speech"
)

// VolcengineTTSClient 火山引擎TTS WebSocket客户端
type VolcengineTTSClient struct {
	config *speech.SpeechConfig
	dialer *websocket.Dialer
}

type ttsServerMessage struct {
	ReqID    string `json:"reqid"`
	Code     int    `json:"code"`
	Message  string `json:"message"`
	Sequence int    `json:"sequence"`
	Data     string `json:"data"`
	Addition struct {
		Duration string `json:"duration,omitempty"`
	} `json:"addition,omitempty"`
}

// NewVolcengineTTSClient 创建火山引擎TTS客户端
func NewVolcengineTTSClient(config *speech.SpeechConfig) *VolcengineTTSClient {
	return &VolcengineTTSClient{
		config: config,
		dialer: &websocket.Dialer{
			HandshakeTimeout: 30 * time.Second,
		},
	}
}

type volcengineTTSRequest struct {
	User struct {
		UID string `json:"uid"`
	} `json:"user"`
	ReqParams struct {
		Speaker     string                   `json:"speaker"`
		Text        string                   `json:"text"`
		AudioParams volcengineTTSAudioParams `json:"audio_params"`
		Additions   string                   `json:"additions,omitempty"`
		Language    string                   `json:"language,omitempty"`
	} `json:"req_params"`
}

type volcengineTTSAudioParams struct {
	Format          string  `json:"format"`
	SampleRate      int     `json:"sample_rate"`
	EnableTimestamp bool    `json:"enable_timestamp"`
	SpeedRatio      float32 `json:"speed_ratio,omitempty"`
	VolumeRatio     float32 `json:"volume_ratio,omitempty"`
}

// SynthesizeSpeechWS 使用WebSocket协议进行语音合成
func (c *VolcengineTTSClient) SynthesizeSpeechWS(ctx context.Context, req *speech.TTSRequest) (*speech.TTSResponse, error) {
	const wsURL = "wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream"

	if strings.TrimSpace(req.Text) == "" {
		return nil, fmt.Errorf("TTS text is empty")
	}

	appKey, accessKey, err := resolveCredentials(c.config)
	if err != nil {
		return nil, err
	}

	encoding := strings.TrimSpace(req.Format)
	if encoding == "" {
		encoding = "mp3"
	}
	if encoding == "wav" {
		encoding = "mp3"
	}

	speakers := resolveTTSSpeakerCandidates(strings.TrimSpace(req.Voice), strings.TrimSpace(c.config.TTSVoice))
	var lastMismatch error

	for speakerIdx, speaker := range speakers {
		resourceCandidates := resolveTTSResourceCandidates(speaker)
		var mismatchErr error

		for resourceIdx, resourceID := range resourceCandidates {
			resp, attemptErr := c.synthesizeSpeechWithResource(ctx, wsURL, req, appKey, accessKey, speaker, encoding, resourceID)
			if attemptErr == nil {
				if resourceIdx > 0 {
					log.Printf("[TTS] voice %s succeeded with fallback resource %s", speaker, resourceID)
				}
				if speakerIdx > 0 {
					log.Printf("[TTS] fallback voice %s succeeded", speaker)
				}
				return resp, nil
			}

			if isResourceMismatchError(attemptErr) {
				log.Printf("[TTS] voice %s resource %s mismatch: %v", speaker, resourceID, attemptErr)
				mismatchErr = attemptErr
				continue
			}

			return nil, attemptErr
		}

		if mismatchErr != nil {
			lastMismatch = mismatchErr
			continue
		}
	}

	if lastMismatch != nil {
		return nil, lastMismatch
	}

	return nil, fmt.Errorf("TTS synthesis failed: no compatible resource id or speaker for voice candidates %v", speakers)
}

func (c *VolcengineTTSClient) synthesizeSpeechWithResource(
	ctx context.Context,
	wsURL string,
	req *speech.TTSRequest,
	appKey, accessKey, speaker, encoding, resourceID string,
) (*speech.TTSResponse, error) {
	connectID := uuid.New().String()

	header := http.Header{}
	header.Set("X-Api-App-Key", appKey)
	header.Set("X-Api-Access-Key", accessKey)
	header.Set("X-Api-Resource-Id", resourceID)
	header.Set("X-Api-Connect-Id", connectID)

	conn, resp, err := c.dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to TTS WebSocket: %w", err)
	}
	defer conn.Close()

	if resp != nil {
		if logid := resp.Header.Get("X-Tt-Logid"); logid != "" {
			log.Printf("[TTS] connected with logid: %s", logid)
		}
	}

	ttsReq, userUID := c.buildTTSRequest(req, speaker, encoding)

	payloadData, err := json.Marshal(ttsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal TTS request: %w", err)
	}

	message := CreateFullClientRequest(payloadData, NoCompression)

	messageBytes, err := EncodeMessage(message)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, messageBytes); err != nil {
		return nil, fmt.Errorf("failed to send TTS request: %w", err)
	}

	var (
		audioBuffer bytes.Buffer
		reqID       string
		duration    int64
	)

	responseSessionID := strings.TrimSpace(req.SessionID)
	if responseSessionID == "" {
		responseSessionID = userUID
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			_, data, err := conn.ReadMessage()
			if err != nil {
				return nil, fmt.Errorf("failed to read TTS response: %w", err)
			}

			msg, err := DecodeMessage(bytes.NewReader(data))
			if err != nil {
				return nil, fmt.Errorf("failed to decode TTS message: %w", err)
			}

			switch msg.Header.MessageType {
			case ErrorMessage:
				payload, err := DecompressPayload(msg.Payload, msg.Header.CompressionMethod)
				if err != nil {
					return nil, fmt.Errorf("TTS error message decode failed: %w", err)
				}
				return nil, fmt.Errorf("TTS error: %s", string(payload))

			case AudioOnlyServerResponse:
				chunk, err := DecompressPayload(msg.Payload, msg.Header.CompressionMethod)
				if err != nil {
					return nil, fmt.Errorf("failed to decompress audio chunk: %w", err)
				}
				audioBuffer.Write(chunk)

			case FullServerResponse:
				payload, err := DecompressPayload(msg.Payload, msg.Header.CompressionMethod)
				if err != nil {
					return nil, fmt.Errorf("failed to decompress TTS response payload: %w", err)
				}

				if msg.Header.MessageFlags == WithEvent && msg.EventType != EventTypeSessionFinished {
					log.Printf("[TTS] server event: %d", msg.EventType)
				}

				var serverResp ttsServerMessage
				if len(payload) > 0 {
					if err := json.Unmarshal(payload, &serverResp); err != nil {
						log.Printf("[TTS] failed to unmarshal response payload: %v", err)
					} else {
						if serverResp.Code != 0 && serverResp.Code != 3000 {
							return nil, fmt.Errorf("TTS API error %d: %s", serverResp.Code, serverResp.Message)
						}

						if serverResp.ReqID != "" {
							reqID = serverResp.ReqID
						}

						if serverResp.Addition.Duration != "" {
							if parsed, err := parseDuration(serverResp.Addition.Duration); err == nil {
								duration = parsed
							}
						}

						if serverResp.Data != "" {
							if chunk, err := decodeBase64Audio(serverResp.Data); err == nil {
								audioBuffer.Write(chunk)
							} else {
								return nil, fmt.Errorf("failed to decode base64 audio chunk: %w", err)
							}
						}
					}
				}

				finalizedByEvent := msg.Header.MessageFlags == WithEvent && msg.EventType == EventTypeSessionFinished
				finalizedBySequence := msg.IsLastPacket() || serverResp.Sequence < 0

				if finalizedByEvent || finalizedBySequence {
					if audioBuffer.Len() == 0 {
						return nil, fmt.Errorf("TTS audio is empty")
					}
					if reqID == "" {
						reqID = connectID
					}
					return &speech.TTSResponse{
						SessionID: responseSessionID,
						AudioData: audioBuffer.Bytes(),
						Duration:  duration,
						Format:    encoding,
						RequestID: reqID,
						CreatedAt: time.Now(),
					}, nil
				}

			default:
				log.Printf("[TTS] unexpected message type: %d", msg.Header.MessageType)
			}
		}
	}
}

// buildTTSRequest 构建符合火山引擎API格式的TTS请求
func (c *VolcengineTTSClient) buildTTSRequest(req *speech.TTSRequest, speaker, encoding string) (*volcengineTTSRequest, string) {
	ttsReq := &volcengineTTSRequest{}

	userUID := strings.TrimSpace(req.SessionID)
	if userUID == "" {
		userUID = uuid.New().String()
	}
	ttsReq.User.UID = userUID

	ttsReq.ReqParams.Speaker = speaker
	if ttsReq.ReqParams.Speaker == "" {
		ttsReq.ReqParams.Speaker = strings.TrimSpace(c.config.TTSVoice)
	}

	ttsReq.ReqParams.Text = req.Text

	format := encoding
	if format == "" {
		format = "mp3"
	}
	if format == "wav" {
		format = "mp3"
	}
	ttsReq.ReqParams.AudioParams.Format = format
	ttsReq.ReqParams.AudioParams.SampleRate = 24000
	ttsReq.ReqParams.AudioParams.EnableTimestamp = true

	speed := req.Speed
	if speed <= 0 && c.config.TTSSpeed > 0 {
		speed = c.config.TTSSpeed
	}
	if speed > 0 && speed != 1.0 {
		ttsReq.ReqParams.AudioParams.SpeedRatio = speed
	}

	volume := req.Volume
	if volume <= 0 && c.config.TTSVolume > 0 {
		volume = c.config.TTSVolume
	}
	if volume > 0 && volume != 1.0 {
		ttsReq.ReqParams.AudioParams.VolumeRatio = volume
	}

	language := strings.TrimSpace(req.Language)
	if language == "" {
		language = strings.TrimSpace(c.config.TTSLanguage)
	}
	if language != "" {
		ttsReq.ReqParams.Language = language
	}

	ttsReq.ReqParams.Additions = buildAdditionsPayload()

	return ttsReq, userUID
}

func buildAdditionsPayload() string {
	additions := map[string]any{
		"disable_markdown_filter": false,
	}

	data, err := json.Marshal(additions)
	if err != nil {
		return "{}"
	}

	return string(data)
}

func resolveTTSResourceCandidates(voice string) []string {
	const (
		defaultResource = "volc.service_type.10029"
		megaResource    = "volc.megatts.default"
		seedResource    = "seed-tts-2.0"
	)

	voice = strings.TrimSpace(voice)
	if voice == "" {
		return []string{defaultResource, seedResource}
	}

	if strings.HasPrefix(voice, "S_") {
		return []string{megaResource}
	}

	normalized := strings.ToLower(voice)
	seedHints := []string{
		"bigtts",
		"seed",
		"megatts",
		"uranus",
		"venus",
		"jupiter",
		"saturn",
		"neptune",
		"mercury",
		"pluto",
		"mars",
	}

	for _, hint := range seedHints {
		if strings.Contains(normalized, hint) {
			return []string{seedResource, defaultResource}
		}
	}

	return []string{defaultResource, seedResource}
}

func resolveTTSSpeakerCandidates(requested, fallback string) []string {
	aliasMap := map[string]string{
		"hogwarts-young-hero":                   "zh_male_M392_conversation_wvae_bigtts",
		"athens-wise-mentor":                    "zh_male_M392_conversation_wvae_bigtts",
		"stark-industries":                      "zh_male_M392_conversation_wvae_bigtts",
		"tavern-guide":                          "zh_female_vv_venus_bigtts",
		"default":                               fallback,
		"en_default":                            "en_female_amy_jupiter_bigtts",
		"zh_female_vv_uranus_bigtts":            "zh_female_vv_uranus_bigtts",
		"zh_male_m392_conversation":             "zh_male_M392_conversation_wvae_bigtts",
		"zh_male_m392_conversation_wvae_bigtts": "zh_male_M392_conversation_wvae_bigtts",
	}

	var candidates []string

	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if mapped, ok := aliasMap[strings.ToLower(s)]; ok {
			s = mapped
		}
		for _, existing := range candidates {
			if strings.EqualFold(existing, s) {
				return
			}
		}
		candidates = append(candidates, s)
	}

	add(requested)
	add(fallback)

	if len(candidates) == 0 {
		return []string{fallback}
	}

	return candidates
}

func isResourceMismatchError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	return strings.Contains(errMsg, "resource ID is mismatched with speaker related resource")
}

// decodeBase64Audio 解码base64音频数据
func decodeBase64Audio(base64Data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(base64Data)
}

// parseDuration 解析时长字符串（毫秒）
func parseDuration(durationStr string) (int64, error) {
	if durationStr == "" {
		return 0, nil
	}
	return strconv.ParseInt(durationStr, 10, 64)
}
