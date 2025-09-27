package emotion

import (
	"math"
	"strings"
)

// Label 表示TTS可以接受的情绪标签。
type Label string

const (
	Neutral  Label = "neutral"
	Happy    Label = "happy"
	Sad      Label = "sad"
	Angry    Label = "angry"
	Excited  Label = "excited"
	Tender   Label = "tender"
	Comfort  Label = "comfort"
	Magnetic Label = "magnetic"
)

// Decision 给出情绪识别结果以及推荐情绪强度。
type Decision struct {
	Emotion Label
	Scale   float32
	Score   int
}

var keywordBuckets = map[Label][]string{
	Happy: {
		"开心", "高兴", "喜悦", "快乐", "兴奋", "太好了", "太棒了", "真棒", "哈哈", "lol", "amazing",
		"awesome", "great", "thanks", "thank you", "love", "喜欢", "满意", "满意的", "好耶", "笑死", "开心的",
	},
	Sad: {
		"难过", "伤心", "失落", "沮丧", "悲伤", "哭", "痛苦", "寂寞", "孤单", "忧", "失望", "伤心欲绝",
		"unhappy", "sad", "cry", "depressed", "tragedy", "upset", "hurt", "sorrow", "心碎", "低落", "委屈",
	},
	Angry: {
		"生气", "愤怒", "火大", "气死", "怒", "烦死", "受够了", "炸", "怒火", "气愤", "抓狂", "怒不可遏",
		"angry", "furious", "rage", "mad", "annoyed", "pissed", "outrage", "storm off", "气炸", "爆炸",
	},
	Excited: {
		"期待", "激动", "太酷了", "震撼", "惊喜", "哇塞", "哇哦", "can't wait", "can't wait", "superb",
		"unbelievable", "hype", "燃", "热血", "热血沸腾", "兴奋", "酷", "给力", "炸裂", "wow", "惊艳", "太妙了",
	},
	Tender: {
		"温柔", "轻声", "慢慢", "柔和", "柔软", "soft", "gentle", "calm", "平静", "放松", "轻柔", "柔情",
		"细腻", "温和", "暖", "softly", "抚慰", "抚摸", "静静", "小心", "谨慎", "轻轻", "温软",
	},
	Comfort: {
		"别担心", "没事", "我懂", "理解", "支持", "陪着", "抱抱", "不要怕", "安心", "安慰", "放松", "陪伴",
		"安抚", "放心", "别害怕", "we are here", "for you", "calm down", "breathe", "take it easy", "i'm here",
		"you're safe", "不要着急", "慢慢来", "不要怕", "抱抱你", "给你力量",
	},
	Magnetic: {
		"认真", "严肃", "重要", "必须", "责任", "庄重", "郑重", "严谨", "focus", "关键", "critical", "serious",
		"必须要", "不能忽视", "格外注意", "十分重要", "谨慎", "务必", "记住", "请务必",
	},
}

var punctuationBoost = map[Label]int{
	Happy:   2,
	Excited: 3,
}

// Analyze 根据用户话语与AI回复推断应使用的语音情绪。
func Analyze(userUtterance, aiUtterance string) Decision {
	userScore := scoreText(userUtterance)
	aiScore := scoreText(aiUtterance)

	finalScore := aiScore
	// 若AI回复缺少明显情感，则根据用户情绪进行映射，从而提供安抚或共情。
	if finalScore.Score == 0 && userScore.Score > 0 {
		finalScore = coerceEmotionFromUser(userScore)
	}

	if finalScore.Score == 0 {
		return Decision{Emotion: Neutral, Scale: 3, Score: 0}
	}

	scale := 2 + float32(finalScore.Score)/4 // 基础为2，强度随得分提升
	if finalScore.Emotion == Excited {
		scale += 1
	}
	if finalScore.Emotion == Magnetic {
		scale = float32(math.Min(4.0, float64(scale)))
	}
	if finalScore.Emotion == Comfort || finalScore.Emotion == Tender {
		scale = float32(math.Min(3.5, float64(scale)))
	}

	if scale < 1 {
		scale = 1
	}
	if scale > 5 {
		scale = 5
	}

	return Decision{Emotion: finalScore.Emotion, Scale: scale, Score: finalScore.Score}
}

func scoreText(text string) Decision {
	normalized := strings.TrimSpace(strings.ToLower(text))
	if normalized == "" {
		return Decision{Emotion: Neutral, Scale: 0, Score: 0}
	}

	scores := make(map[Label]int)
	for label, keywords := range keywordBuckets {
		for _, word := range keywords {
			if word == "" {
				continue
			}
			if strings.Contains(normalized, strings.ToLower(word)) {
				scores[label] += 3
			}
		}
	}

	exclamations := strings.Count(text, "!")
	if exclamations > 0 {
		scores[Excited] += exclamations * punctuationBoost[Excited]
		if exclamations == 1 {
			scores[Happy] += punctuationBoost[Happy]
		}
	}

	bestLabel := Neutral
	bestScore := 0
	for label, s := range scores {
		if s > bestScore {
			bestScore = s
			bestLabel = label
		}
	}

	if bestScore == 0 {
		return Decision{Emotion: Neutral, Score: 0, Scale: 0}
	}

	return Decision{Emotion: bestLabel, Score: bestScore, Scale: 0}
}

func coerceEmotionFromUser(user Decision) Decision {
	switch user.Emotion {
	case Sad:
		return Decision{Emotion: Comfort, Score: user.Score}
	case Angry:
		return Decision{Emotion: Magnetic, Score: user.Score}
	case Excited:
		return Decision{Emotion: Excited, Score: user.Score}
	case Happy:
		return Decision{Emotion: Happy, Score: user.Score}
	case Tender, Comfort:
		return Decision{Emotion: Tender, Score: user.Score}
	default:
		return user
	}
}
