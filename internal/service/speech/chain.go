package speech

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/zhouzirui/z-tavern/backend/internal/model/speech"
)

// SpeechChain 语音处理链，集成ASR、AI模型和TTS
type SpeechChain struct {
	speechSvc *Service
	chatModel model.ChatModel
}

// NewSpeechChain 创建语音处理链
func NewSpeechChain(speechSvc *Service, chatModel model.ChatModel) *SpeechChain {
	return &SpeechChain{
		speechSvc: speechSvc,
		chatModel: chatModel,
	}
}

// VoiceToVoiceInput 语音到语音的输入
type VoiceToVoiceInput struct {
	SessionID    string `json:"sessionId"`
	AudioData    []byte `json:"-"`
	AudioFormat  string `json:"audioFormat"`
	Language     string `json:"language"`
	PersonaID    string `json:"personaId"`
	SystemPrompt string `json:"systemPrompt"`
}

// VoiceToVoiceOutput 语音到语音的输出
type VoiceToVoiceOutput struct {
	SessionID     string `json:"sessionId"`
	InputText     string `json:"inputText"`
	OutputText    string `json:"outputText"`
	OutputAudio   []byte `json:"-"`
	AudioFormat   string `json:"audioFormat"`
	ASRConfidence float64 `json:"asrConfidence"`
	ProcessTime   int64  `json:"processTime"`
}

// ProcessVoiceToVoice 处理语音到语音的完整流程
func (sc *SpeechChain) ProcessVoiceToVoice(ctx context.Context, input *VoiceToVoiceInput) (*VoiceToVoiceOutput, error) {
	// 步骤1: ASR - 语音转文本
	asrResp, err := sc.speechSvc.TranscribeBuffer(ctx, input.SessionID, input.AudioData, input.AudioFormat, input.Language)
	if err != nil {
		return nil, fmt.Errorf("ASR failed: %w", err)
	}

	// 步骤2: 构建AI模型的消息
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: input.SystemPrompt,
		},
		{
			Role:    schema.User,
			Content: asrResp.Text,
		},
	}

	// 步骤3: AI模型生成回复
	aiResp, err := sc.chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	// 步骤4: TTS - 文本转语音
	ttsResp, err := sc.speechSvc.SynthesizeToBuffer(ctx, input.SessionID, aiResp.Content, "", input.Language)
	if err != nil {
		return nil, fmt.Errorf("TTS failed: %w", err)
	}

	return &VoiceToVoiceOutput{
		SessionID:     input.SessionID,
		InputText:     asrResp.Text,
		OutputText:    aiResp.Content,
		OutputAudio:   ttsResp.AudioData,
		AudioFormat:   ttsResp.Format,
		ASRConfidence: asrResp.Confidence,
		ProcessTime:   ttsResp.Duration,
	}, nil
}

// StreamingVoiceProcessor 流式语音处理器
type StreamingVoiceProcessor struct {
	speechSvc *Service
	chatModel model.ChatModel
}

// NewStreamingVoiceProcessor 创建流式语音处理器
func NewStreamingVoiceProcessor(speechSvc *Service, chatModel model.ChatModel) *StreamingVoiceProcessor {
	return &StreamingVoiceProcessor{
		speechSvc: speechSvc,
		chatModel: chatModel,
	}
}

// StreamingVoiceInput 流式语音输入
type StreamingVoiceInput struct {
	SessionID     string
	AudioStream   <-chan []byte
	SystemPrompt  string
	Language      string
	PersonaID     string
}

// StreamingVoiceOutput 流式语音输出
type StreamingVoiceOutput struct {
	SessionID      string
	TextChunk      string
	AudioChunk     []byte
	IsTextFinal    bool
	IsAudioFinal   bool
	ASRConfidence  float64
}

// ProcessStreamingVoice 处理流式语音交互
func (svp *StreamingVoiceProcessor) ProcessStreamingVoice(ctx context.Context, input *StreamingVoiceInput, output chan<- *StreamingVoiceOutput) error {
	defer close(output)

	// 创建ASR流式识别结果通道
	asrResults := make(chan *speech.StreamingASRChunk, 10)

	// 启动流式ASR
	go func() {
		defer close(asrResults)
		if err := svp.speechSvc.TranscribeStream(ctx, input.SessionID, input.AudioStream, asrResults); err != nil {
			return
		}
	}()

	var fullText string
	var textChunks []string

	// 处理ASR结果并进行AI生成
	for asrChunk := range asrResults {
		// 发送ASR中间结果
		output <- &StreamingVoiceOutput{
			SessionID:     input.SessionID,
			TextChunk:     asrChunk.Text,
			IsTextFinal:   asrChunk.IsFinal,
			ASRConfidence: asrChunk.Confidence,
		}

		if asrChunk.IsFinal {
			fullText += asrChunk.Text
			textChunks = append(textChunks, asrChunk.Text)

			// 当有完整句子时，进行AI处理
			if len(fullText) > 0 {
				messages := []*schema.Message{
					{
						Role:    schema.System,
						Content: input.SystemPrompt,
					},
					{
						Role:    schema.User,
						Content: fullText,
					},
				}

				// AI生成回复
				aiResp, err := svp.chatModel.Generate(ctx, messages)
				if err != nil {
					continue
				}

				// TTS生成语音
				ttsResp, err := svp.speechSvc.SynthesizeToBuffer(ctx, input.SessionID, aiResp.Content, "", input.Language)
				if err != nil {
					continue
				}

				// 发送最终结果
				output <- &StreamingVoiceOutput{
					SessionID:    input.SessionID,
					TextChunk:    aiResp.Content,
					AudioChunk:   ttsResp.AudioData,
					IsTextFinal:  true,
					IsAudioFinal: true,
				}

				fullText = "" // 重置文本，准备处理下一轮
			}
		}
	}

	return nil
}