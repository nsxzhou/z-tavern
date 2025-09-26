package speech

import (
	"bytes"
	"context"
	"time"

	"github.com/zhouzirui/z-tavern/backend/internal/model/speech"
)

// Service 语音服务核心业务逻辑
type Service struct {
	config         *speech.SpeechConfig
	ttsClient      *VolcengineTTSClient
	asrClient      *VolcengineASRClient
	connectionPool *ConnectionPool
	errorHandler   *ErrorHandler
}

// NewService 创建语音服务实例
func NewService(config *speech.SpeechConfig) *Service {
	connectionPool := NewConnectionPool(DefaultConnectionPoolOptions())
	errorHandler := NewErrorHandler()

	return &Service{
		config:         config,
		ttsClient:      NewVolcengineTTSClient(config),
		asrClient:      NewVolcengineASRClient(config),
		connectionPool: connectionPool,
		errorHandler:   errorHandler,
	}
}

// Cleanup 清理资源
func (s *Service) Cleanup() {
	if s.connectionPool != nil {
		s.connectionPool.Cleanup()
	}
}

// TranscribeAudio 语音转文字 - 使用WebSocket协议
func (s *Service) TranscribeAudio(ctx context.Context, req *speech.ASRRequest) (*speech.ASRResponse, error) {
	// 使用新的WebSocket ASR客户端
	return s.asrClient.TranscribeAudioWS(ctx, req)
}

// SynthesizeSpeech 文字转语音 - 使用WebSocket协议
func (s *Service) SynthesizeSpeech(ctx context.Context, req *speech.TTSRequest) (*speech.TTSResponse, error) {
	// 使用新的WebSocket TTS客户端
	return s.ttsClient.SynthesizeSpeechWS(ctx, req)
}

// TranscribeBuffer 语音转文字（使用字节数组）
func (s *Service) TranscribeBuffer(ctx context.Context, sessionID string, audioData []byte, format, language string) (*speech.ASRResponse, error) {
	// 创建ASR请求
	req := &speech.ASRRequest{
		SessionID: sessionID,
		AudioData: bytes.NewReader(audioData),
		Format:    format,
		Language:  language,
	}

	return s.TranscribeAudio(ctx, req)
}

// SynthesizeToBuffer 文字转语音（返回字节数组）
func (s *Service) SynthesizeToBuffer(ctx context.Context, sessionID, text, voice, language string) (*speech.TTSResponse, error) {
	// 创建TTS请求
	req := &speech.TTSRequest{
		SessionID: sessionID,
		Text:      text,
		Voice:     voice,
		Language:  language,
	}

	return s.SynthesizeSpeech(ctx, req)
}

// TranscribeStream 流式语音识别
func (s *Service) TranscribeStream(ctx context.Context, sessionID string, audioStream <-chan []byte, results chan<- *speech.StreamingASRChunk) error {
	// 这是一个简化的实现，实际的流式识别需要WebSocket或类似的长连接
	// 这里我们模拟流式处理，将音频流缓冲后批量处理

	var buffer []byte
	for audioChunk := range audioStream {
		buffer = append(buffer, audioChunk...)

		// 当缓冲区达到一定大小时进行识别
		if len(buffer) >= 16000 { // 假设16KB为一个处理单位
			asrResp, err := s.TranscribeBuffer(ctx, sessionID, buffer, "pcm", "zh-CN")
			if err != nil {
				continue // 忽略错误，继续处理
			}

			// 发送流式结果
			chunk := &speech.StreamingASRChunk{
				SessionID:  sessionID,
				Text:       asrResp.Text,
				IsFinal:    true,
				Confidence: asrResp.Confidence,
				StartTime:  0,
				EndTime:    asrResp.Duration,
				RequestID:  asrResp.RequestID,
				CreatedAt:  time.Now(),
			}

			select {
			case results <- chunk:
			case <-ctx.Done():
				return ctx.Err()
			}

			buffer = buffer[:0] // 清空缓冲区
		}
	}

	// 处理剩余的音频数据
	if len(buffer) > 0 {
		asrResp, err := s.TranscribeBuffer(ctx, sessionID, buffer, "pcm", "zh-CN")
		if err == nil {
			chunk := &speech.StreamingASRChunk{
				SessionID:  sessionID,
				Text:       asrResp.Text,
				IsFinal:    true,
				Confidence: asrResp.Confidence,
				StartTime:  0,
				EndTime:    asrResp.Duration,
				RequestID:  asrResp.RequestID,
				CreatedAt:  time.Now(),
			}

			select {
			case results <- chunk:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return nil
}
