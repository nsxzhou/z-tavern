package ai

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/zhouzirui/z-tavern/backend/internal/config"
	"github.com/zhouzirui/z-tavern/backend/internal/model/persona"
	"github.com/zhouzirui/z-tavern/backend/internal/model/chat"
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
func (s *Service) GenerateResponse(ctx context.Context, sessionID string, persona *persona.Persona, messages []chat.Message, userMessage string) (*schema.Message, error) {
	input := s.buildChainInput(persona, messages, userMessage)

	response, err := s.chain.Invoke(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to run AI chain: %w", err)
	}

	log.Printf("[ai] generated response for session=%s, persona=%s, length=%d", sessionID, persona.ID, len(response.Content))
	return response, nil
}

// StreamResponse streams AI response chunks via the configured chain.
func (s *Service) StreamResponse(ctx context.Context, persona *persona.Persona, messages []chat.Message, userMessage string) (*schema.StreamReader[*schema.Message], error) {
	if !s.StreamingEnabled() {
		return nil, fmt.Errorf("streaming disabled in configuration")
	}

	input := s.buildChainInput(persona, messages, userMessage)

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
func (s *Service) buildChainInput(persona *persona.Persona, messages []chat.Message, userMessage string) map[string]any {
	return map[string]any{
		"system":  s.buildSystemPrompt(persona),
		"history": s.buildHistoryMessages(messages),
		"query":   userMessage,
	}
}

// buildSystemPrompt creates a comprehensive system prompt for the persona
func (s *Service) buildSystemPrompt(persona *persona.Persona) string {
	// Use the enhanced prompt manager
	promptManager := NewPersonaPromptManager()
	return promptManager.BuildSystemPrompt(persona)
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
