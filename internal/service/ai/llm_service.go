package ai

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/zhouzirui/z-tavern/backend/internal/analysis/emotion"
	"github.com/zhouzirui/z-tavern/backend/internal/config"
	"github.com/zhouzirui/z-tavern/backend/internal/model/chat"
	"github.com/zhouzirui/z-tavern/backend/internal/model/persona"
	emotionservice "github.com/zhouzirui/z-tavern/backend/internal/service/emotion"
)

// Service encapsulates AI-powered chat functionality
type Service struct {
	chatModel model.ChatModel
	personas  persona.Store
	cfg       config.AIConfig
	chain     compose.Runnable[map[string]any, *schema.Message]
}

// NewService creates a new AI service instance
func NewService(ctx context.Context, personas persona.Store, cfg config.AIConfig) (*Service, error) {
	chatModel, err := cfg.NewChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model: %w", err)
	}

	promptTemplate := prompt.FromMessages(
		schema.FString,
		schema.SystemMessage("{system}"),
		schema.MessagesPlaceholder("history", true),
		schema.UserMessage("{query}"),
	)

	chain := compose.NewChain[map[string]any, *schema.Message]()
	chain.AppendChatTemplate(promptTemplate)
	chain.AppendChatModel(chatModel)

	runnable, err := chain.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to compile chat chain: %w", err)
	}

	return &Service{
		chatModel: chatModel,
		personas:  personas,
		cfg:       cfg,
		chain:     runnable,
	}, nil
}

// StreamingEnabled 指示是否开启 SSE 流式输出。
func (s *Service) StreamingEnabled() bool {
	return s.cfg.StreamResponse
}

// GenerateResponse generates AI response for a persona-based conversation
func (s *Service) GenerateResponse(ctx context.Context, sessionID string, persona *persona.Persona, messages []chat.Message, userMessage string, guidance *emotionservice.Guidance) (*schema.Message, error) {
	input := s.buildChainInput(persona, messages, userMessage, guidance)

	response, err := s.chain.Invoke(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to run AI chain: %w", err)
	}

	log.Printf("[ai] generated response for session=%s, persona=%s, length=%d", sessionID, persona.ID, len(response.Content))
	return response, nil
}

// StreamResponse streams AI response chunks via the configured chain.
func (s *Service) StreamResponse(ctx context.Context, persona *persona.Persona, messages []chat.Message, userMessage string, guidance *emotionservice.Guidance) (*schema.StreamReader[*schema.Message], error) {
	if !s.StreamingEnabled() {
		return nil, fmt.Errorf("streaming disabled in configuration")
	}

	input := s.buildChainInput(persona, messages, userMessage, guidance)

	stream, err := s.chain.Stream(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to stream AI chain output: %w", err)
	}

	return stream, nil
}

// GetChatModel 返回底层的聊天模型
func (s *Service) GetChatModel() model.ChatModel {
	return s.chatModel
}

// buildConversationContext creates the message context for the AI model
func (s *Service) buildChainInput(persona *persona.Persona, messages []chat.Message, userMessage string, guidance *emotionservice.Guidance) map[string]any {
	return map[string]any{
		"system":  s.buildSystemPrompt(persona, guidance),
		"history": s.buildHistoryMessages(messages),
		"query":   userMessage,
	}
}

// buildSystemPrompt creates a comprehensive system prompt for the persona
func (s *Service) buildSystemPrompt(persona *persona.Persona, guidance *emotionservice.Guidance) string {
	promptManager := NewPersonaPromptManager()
	base := promptManager.BuildSystemPrompt(persona)

	if guidance == nil {
		return base
	}

	decision := guidance.Decision
	if decision.Emotion == "" {
		return base
	}

	var builder strings.Builder
	builder.WriteString(base)
	builder.WriteString("\n\n基于用户当前状态的情绪分析：")
	emotionDesc := describeEmotion(decision.Emotion)
	if emotionDesc != "" {
		builder.WriteString(emotionDesc)
	} else {
		builder.WriteString(fmt.Sprintf("情绪标签=%s", string(decision.Emotion)))
	}
	builder.WriteString(fmt.Sprintf("，强度约 %.1f。", decision.Scale))
	if guidance.Style != "" {
		builder.WriteString("\n回复建议：")
		builder.WriteString(guidance.Style)
	}
	if guidance.Reason != "" {
		builder.WriteString("\n情绪推断理由：")
		builder.WriteString(guidance.Reason)
	}

	builder.WriteString("\n请在保证角色一致性的前提下，优先照顾上述情绪，用更贴切的语气与表达方式回应用户。")
	return builder.String()
}

func (s *Service) buildHistoryMessages(messages []chat.Message) []*schema.Message {
	const historyLimit = 10

	if len(messages) == 0 {
		return nil
	}

	startIdx := 0
	if len(messages) > historyLimit {
		startIdx = len(messages) - historyLimit
	}

	history := make([]*schema.Message, 0, len(messages)-startIdx)
	for _, msg := range messages[startIdx:] {
		switch msg.Sender {
		case "user":
			history = append(history, schema.UserMessage(msg.Content))
		case "assistant":
			history = append(history, schema.AssistantMessage(msg.Content, nil))
		}
	}

	return history
}

func describeEmotion(label emotion.Label) string {
	switch label {
	case emotion.Happy:
		return "用户情绪积极、快乐，需要保持轻快和赞许。"
	case emotion.Sad:
		return "用户情绪低落或伤感，需要温柔安慰与理解。"
	case emotion.Angry:
		return "用户情绪易怒或不满，需要稳重、理性地回应并帮助平复。"
	case emotion.Excited:
		return "用户情绪高涨、兴奋，可以保持热情并延续这种积极能量。"
	case emotion.Tender:
		return "用户渴望温柔细腻的互动，应当放慢节奏、保持柔和。"
	case emotion.Comfort:
		return "用户需要被安抚或陪伴，应当传递安全感与支撑。"
	case emotion.Magnetic:
		return "用户期望坚定可靠的指引，应当展现专业与自信。"
	case emotion.Neutral:
		return "用户情绪平和，请保持清晰、礼貌且自然的语气。"
	default:
		return ""
	}
}
