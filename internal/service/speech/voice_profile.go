package speech

import (
	"strings"

	"github.com/zhouzirui/z-tavern/backend/internal/analysis/emotion"
)

var defaultEmotionLabels = map[emotion.Label]string{
	emotion.Happy:    "happy",
	emotion.Sad:      "sad",
	emotion.Angry:    "angry",
	emotion.Excited:  "excited",
	emotion.Tender:   "tender",
	emotion.Comfort:  "comfort",
	emotion.Magnetic: "magnetic",
}

var emotionVoiceWhitelist = map[string]struct{}{
	"zh_male_junlangnanyou_emo_v2_mars_bigtts":    {},
	"zh_male_yourougongzi_emo_v2_mars_bigtts":     {},
	"zh_male_aojiaobazong_emo_v2_mars_bigtts":     {},
	"zh_female_gaolengyujie_emo_v2_mars_bigtts":   {},
	"zh_female_tianxinxiaomei_emo_v2_mars_bigtts": {},
	"zh_female_linjuayi_emo_v2_mars_bigtts":       {},
	"en_female_candice_emo_v2_mars_bigtts":        {},
	"en_female_skye_emo_v2_mars_bigtts":           {},
	"en_male_glen_emo_v2_mars_bigtts":             {},
	"en_male_sylus_emo_v2_mars_bigtts":            {},
	"en_male_corey_emo_v2_mars_bigtts":            {},
}

// ComputeEmotionParameters 根据语音与情绪分析结果计算TTS情绪参数。
func ComputeEmotionParameters(voice string, decision emotion.Decision) (enable bool, label string, scale float32) {
	if decision.Emotion == emotion.Neutral || decision.Score <= 0 {
		return false, "", 0
	}

	if !supportsEmotion(voice) {
		return false, "", 0
	}

	mapped, ok := defaultEmotionLabels[decision.Emotion]
	if !ok {
		mapped = string(emotion.Neutral)
	}

	finalScale := decision.Scale
	if finalScale <= 0 {
		finalScale = 3
	}
	if finalScale < 1 {
		finalScale = 1
	}
	if finalScale > 5 {
		finalScale = 5
	}

	return true, mapped, finalScale
}

func supportsEmotion(voice string) bool {
	normalized := strings.ToLower(strings.TrimSpace(voice))
	if normalized == "" {
		return false
	}

	if _, ok := emotionVoiceWhitelist[normalized]; ok {
		return true
	}

	if strings.Contains(normalized, "_emo_") || strings.Contains(normalized, "_emo") {
		return true
	}

	return false
}
