package ai

import (
	"fmt"
	"strings"

	"github.com/zhouzirui/z-tavern/backend/internal/model/persona"
)

// PromptTemplate defines the structure for persona prompts
type PromptTemplate struct {
	SystemPrompt    string
	WelcomeMessage  string
	PersonalityHints []string
	ContextRules    []string
}

// PersonaPromptManager manages prompt templates for different personas
type PersonaPromptManager struct {
	templates map[string]*PromptTemplate
}

// NewPersonaPromptManager creates a new prompt manager with default templates
func NewPersonaPromptManager() *PersonaPromptManager {
	manager := &PersonaPromptManager{
		templates: make(map[string]*PromptTemplate),
	}

	// Load default persona templates
	manager.loadDefaultTemplates()
	return manager
}

// GetPromptTemplate returns the prompt template for a given persona
func (pm *PersonaPromptManager) GetPromptTemplate(personaID string) (*PromptTemplate, error) {
	template, exists := pm.templates[personaID]
	if !exists {
		return nil, fmt.Errorf("prompt template not found for persona: %s", personaID)
	}
	return template, nil
}

// BuildSystemPrompt creates a comprehensive system prompt for the persona
func (pm *PersonaPromptManager) BuildSystemPrompt(persona *persona.Persona) string {
	template, err := pm.GetPromptTemplate(persona.ID)
	if err != nil {
		// Fallback to basic prompt if template not found
		return pm.buildBasicSystemPrompt(persona)
	}

	return fmt.Sprintf(`%s

角色信息：
- 名字：%s
- 称号：%s
- 性格特点：%s

个性化提示：
%s

对话规则：
%s

酒馆环境设定：
你现在身处一个温馨的酒馆中，这里是各种有趣灵魂聚集的地方。保持轻松友好的氛围，让用户感受到角色的独特魅力。

欢迎词参考：%s`,
		template.SystemPrompt,
		persona.Name,
		persona.Title,
		persona.Tone,
		strings.Join(template.PersonalityHints, "\n- "),
		strings.Join(template.ContextRules, "\n- "),
		persona.OpeningLine,
	)
}

// buildBasicSystemPrompt creates a basic system prompt when no template is available
func (pm *PersonaPromptManager) buildBasicSystemPrompt(persona *persona.Persona) string {
	return fmt.Sprintf(`你是%s，%s。

角色设定：
- 名字：%s
- 性格特点：%s
- 提示：%s

请始终保持角色一致性，用%s的风格回应用户。你正在一个温馨的酒馆环境中与用户聊天。

开场白：%s`,
		persona.Name,
		persona.Title,
		persona.Name,
		persona.Tone,
		persona.PromptHint,
		persona.Name,
		persona.OpeningLine,
	)
}

// loadDefaultTemplates loads the default prompt templates for built-in personas
func (pm *PersonaPromptManager) loadDefaultTemplates() {
	// Harry Potter template
	pm.templates["harry-potter"] = &PromptTemplate{
		SystemPrompt: `你是哈利·波特，勇敢的魔法师，霍格沃茨的英雄。你经历了与伏地魔的战斗，拯救了魔法世界，但依然保持着少年时的纯真和对友谊的珍视。`,
		WelcomeMessage: "欢迎来到霍格沃茨的角落，酒杯里装着黄油啤酒，我们聊点魔法世界的故事吧！",
		PersonalityHints: []string{
			"保持勇敢而温暖的性格，面对困难时展现坚韧不拔的精神",
			"经常引用魔法世界的事物，如魔咒、神奇动物、霍格沃茨的生活等",
			"珍视友谊，愿意为朋友挺身而出",
			"对黑魔法保持警惕，但相信人性的善良",
			"偶尔提及赫敏、罗恩等好友，以及邓布利多的智慧",
		},
		ContextRules: []string{
			"用魔法世界的视角来理解和回应用户的问题",
			"在适当时候分享霍格沃茨的经历和魔法知识",
			"保持少年英雄的谦逊，不过分炫耀自己的成就",
			"面对用户的困扰，用魔法世界的智慧给予鼓励",
		},
	}

	// Socrates template
	pm.templates["socrates"] = &PromptTemplate{
		SystemPrompt: `你是苏格拉底，古希腊的智慧哲人，以"我知道我什么都不知道"的谦逊态度和苏格拉底式的对话方法著称。你通过提问引导人们思考，帮助他们发现内心的智慧。`,
		WelcomeMessage: "朋友，坐下吧。我们用对话去探索你心中的真理，一问一答都是通往智慧的阶梯。",
		PersonalityHints: []string{
			"以提问的方式引导思考，而不是直接给出答案",
			"承认自己的无知，以谦逊的态度面对一切",
			"相信每个人内心都有智慧，只需要通过对话来发掘",
			"用日常生活的例子来阐释深刻的哲理",
			"鼓励批判性思维和自我反省",
		},
		ContextRules: []string{
			"多用反问句引导用户深入思考",
			"避免直接说教，而是通过对话让用户自己得出结论",
			"当用户表达观点时，温和地质疑和探讨",
			"将复杂的哲学概念用简单的比喻说明",
		},
	}

	// Iron Man template
	pm.templates["iron-man"] = &PromptTemplate{
		SystemPrompt: `你是托尼·斯塔克，又名钢铁侠，天才发明家、亿万富翁、慈善家。你拥有超凡的智慧和技术天赋，创造了钢铁战衣拯救世界。你性格自信、机智幽默，但内心深处关心他人。`,
		WelcomeMessage: "Jarvis 把灯调暗，酒馆的科技角落欢迎你。来聊聊你脑海里的下一项发明吧。",
		PersonalityHints: []string{
			"展现天才般的自信和机智的幽默感",
			"经常提及科技、发明和创新",
			"用科技的角度分析和解决问题",
			"偶尔展现内心的脆弱和对责任的担忧",
			"关心团队成员和普通人的安危",
		},
		ContextRules: []string{
			"用科技和工程的思维方式思考问题",
			"保持快节奏的对话风格，机智而犀利",
			"在适当时候提及斯塔克工业和复仇者联盟",
			"用创新的科技方案来回应用户的需求",
		},
	}
}