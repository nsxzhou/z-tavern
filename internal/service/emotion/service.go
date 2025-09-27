package emotion

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	analysis "github.com/zhouzirui/z-tavern/backend/internal/analysis/emotion"
	"github.com/zhouzirui/z-tavern/backend/internal/model/chat"
	"github.com/zhouzirui/z-tavern/backend/internal/model/persona"
)

// Config 控制情绪分析服务的行为。
type Config struct {
	Enabled      bool
	HistoryLimit int
}

// Guidance 表示情绪分析的结果以及对回复语气的建议。
type Guidance struct {
	Decision   analysis.Decision
	Style      string
	Confidence float32
	Reason     string
}

// Service 使用大模型对会话情绪进行分析，并在必要时回退到启发式规则。
type Service struct {
	enabled      bool
	classifier   compose.Runnable[map[string]any, *schema.Message]
	fallback     func(user, assistant string) analysis.Decision
	historyLimit int
}

// NewService 创建情绪分析服务。chatModel 可重用现有的大模型实例。
func NewService(ctx context.Context, chatModel model.ChatModel, cfg Config) (*Service, error) {
	historyLimit := cfg.HistoryLimit
	if historyLimit <= 0 {
		historyLimit = 6
	}

	svc := &Service{
		enabled:      cfg.Enabled && chatModel != nil,
		fallback:     analysis.Analyze,
		historyLimit: historyLimit,
	}

	if !svc.enabled {
		return svc, nil
	}

	promptTemplate := prompt.FromMessages(
		schema.FString,
		schema.SystemMessage(emotionSystemPrompt),
		schema.UserMessage(emotionUserPrompt),
	)

	chain := compose.NewChain[map[string]any, *schema.Message]()
	chain.AppendChatTemplate(promptTemplate)
	chain.AppendChatModel(chatModel)

	runnable, err := chain.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to compile emotion classifier chain: %w", err)
	}

	svc.classifier = runnable
	return svc, nil
}

// Enabled 返回情绪分析服务是否启用。
func (s *Service) Enabled() bool {
	return s != nil && s.enabled && s.classifier != nil
}

// Analyze 根据会话上下文与回复预测情绪。assistantMessage 为空时同样运行，以便在回复前获取语气建议。
func (s *Service) Analyze(ctx context.Context, personaObj *persona.Persona, history []chat.Message, userMessage, assistantMessage string) Guidance {
	if !s.Enabled() {
		return s.fallbackGuidance(personaObj, userMessage, assistantMessage)
	}

	input := map[string]any{
		"persona":         summarizePersona(personaObj),
		"history":         formatHistory(history, s.historyLimit),
		"user_message":    strings.TrimSpace(userMessage),
		"assistant_draft": strings.TrimSpace(assistantMessage),
	}

	msg, err := s.classifier.Invoke(ctx, input)
	if err != nil {
		log.Printf("[emotion] classifier invoke failed, use fallback: %v", err)
		return s.fallbackGuidance(personaObj, userMessage, assistantMessage)
	}
	if msg == nil || strings.TrimSpace(msg.Content) == "" {
		return s.fallbackGuidance(personaObj, userMessage, assistantMessage)
	}

	result, err := parseClassifierOutput(msg.Content)
	if err != nil {
		log.Printf("[emotion] classifier output parse failed, use fallback: %v", err)
		return s.fallbackGuidance(personaObj, userMessage, assistantMessage)
	}

	label, ok := parseEmotionLabel(result.Emotion)
	if !ok {
		return s.fallbackGuidance(personaObj, userMessage, assistantMessage)
	}

	scale := clampScale(result.Scale)
	decision := analysis.Decision{
		Emotion: label,
		Scale:   scale,
		Score:   int(scale * 2),
	}

	style := strings.TrimSpace(result.Style)
	if style == "" {
		style = defaultStyleByEmotion[decision.Emotion]
	}

	confidence := result.Confidence
	if confidence <= 0 {
		confidence = 0.6
	}
	if confidence > 1 {
		confidence = 1
	}

	return Guidance{
		Decision:   decision,
		Style:      style,
		Confidence: confidence,
		Reason:     strings.TrimSpace(result.Reason),
	}
}

func (s *Service) fallbackGuidance(personaObj *persona.Persona, userMessage, assistantMessage string) Guidance {
	decision := s.fallback(userMessage, assistantMessage)
	style := defaultStyleByEmotion[decision.Emotion]
	if style == "" {
		style = "保持自然友好的语气。"
	}

	confidence := float32(0.3)
	if decision.Score > 0 {
		confidence = 0.55
	}

	return Guidance{
		Decision:   decision,
		Style:      style,
		Confidence: confidence,
		Reason:     "fallback",
	}
}

// parseClassifierOutput 解析大模型返回的 JSON。
func parseClassifierOutput(content string) (*classifierPayload, error) {
	trimmed := strings.TrimSpace(content)
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("missing json object")
	}

	payload := &classifierPayload{}
	if err := json.Unmarshal([]byte(trimmed[start:end+1]), payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func summarizePersona(p *persona.Persona) string {
	if p == nil {
		return "无特定角色设定。"
	}

	sections := []string{
		fmt.Sprintf("名字:%s", strings.TrimSpace(p.Name)),
		fmt.Sprintf("称号:%s", strings.TrimSpace(p.Title)),
	}
	if tone := strings.TrimSpace(p.Tone); tone != "" {
		sections = append(sections, fmt.Sprintf("既有语气:%s", tone))
	}
	return strings.Join(sections, " | ")
}

func formatHistory(messages []chat.Message, limit int) string {
	if len(messages) == 0 {
		return "无历史对话"
	}
	if limit < 1 {
		limit = 1
	}
	start := len(messages) - limit
	if start < 0 {
		start = 0
	}

	var builder strings.Builder
	for i := start; i < len(messages); i++ {
		msg := messages[i]
		role := "用户"
		if strings.EqualFold(msg.Sender, "assistant") {
			role = "AI"
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		builder.WriteString(role)
		builder.WriteString(": ")
		builder.WriteString(content)
		if i < len(messages)-1 {
			builder.WriteString("\n")
		}
	}
	if builder.Len() == 0 {
		return "无历史对话"
	}
	return builder.String()
}

func parseEmotionLabel(raw string) (analysis.Label, bool) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "neutral":
		return analysis.Neutral, true
	case "happy":
		return analysis.Happy, true
	case "sad":
		return analysis.Sad, true
	case "angry":
		return analysis.Angry, true
	case "excited":
		return analysis.Excited, true
	case "tender":
		return analysis.Tender, true
	case "comfort":
		return analysis.Comfort, true
	case "magnetic":
		return analysis.Magnetic, true
	default:
		return "", false
	}
}

func clampScale(val float32) float32 {
	if val <= 0 {
		return 3
	}
	if val < 1 {
		return 1
	}
	if val > 5 {
		return 5
	}
	return val
}

type classifierPayload struct {
	Emotion    string  `json:"emotion"`
	Scale      float32 `json:"scale"`
	Confidence float32 `json:"confidence"`
	Style      string  `json:"style"`
	Reason     string  `json:"reason"`
}

const emotionSystemPrompt = "你是一名情绪与语气的分析师。请阅读提供的角色设定、历史对话、用户输入以及（可选的）AI 草稿，推断用户当前情绪，并给出 AI 回复应该采用的语气建议。\n输出要求：只返回一个 JSON 对象，字段如下：emotion (必须是 neutral/happy/sad/angry/excited/tender/comfort/magnetic 之一)、scale (1~5 之间的数字，可有小数)、confidence (0~1 之间的小数)、style (一句话描述建议的语气)、reason (简要中文理由)。不得输出多余文本。"

const emotionUserPrompt = "角色信息：\n{persona}\n\n最近对话：\n{history}\n\n用户最新输入：\n{user_message}\n\nAI 预期回复草稿（可能为空）：\n{assistant_draft}\n\n请基于这些信息给出 JSON。"

var defaultStyleByEmotion = map[analysis.Label]string{
	analysis.Neutral:  "语气平和、耐心，确保信息清晰。",
	analysis.Happy:    "语气轻快且充满正能量，适度赞美与鼓励。",
	analysis.Sad:      "语气柔和、富有同理心，适当安慰。",
	analysis.Angry:    "语气沉稳、理性，先理解情绪再帮助纾解。",
	analysis.Excited:  "语气热情、积极，与用户一起保持兴奋。",
	analysis.Tender:   "语气温柔细腻，放慢节奏给予陪伴。",
	analysis.Comfort:  "语气温暖，传递安全感与支持。",
	analysis.Magnetic: "语气稳重有力，条理清晰并传递信任。",
}
