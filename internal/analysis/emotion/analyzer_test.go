package emotion

import "testing"

func TestAnalyzeSadUserGetsComfort(t *testing.T) {
	decision := Analyze("我今天很难过", "我会陪着你一起面对")
	if decision.Emotion != Comfort {
		t.Fatalf("expected comfort emotion, got %s", decision.Emotion)
	}
	if decision.Scale < 1 || decision.Scale > 5 {
		t.Fatalf("emotion scale out of range: %f", decision.Scale)
	}
}

func TestAnalyzeExcitedUser(t *testing.T) {
	decision := Analyze("太棒了!!! 我们成功了", "真是振奋的消息！")
	if decision.Emotion != Excited {
		t.Fatalf("expected excited emotion, got %s", decision.Emotion)
	}
	if decision.Scale < 1.5 {
		t.Fatalf("expected boosted scale for excitement, got %f", decision.Scale)
	}
}

func TestAnalyzeHappyAIResponse(t *testing.T) {
	decision := Analyze("谢谢你", "我也替你感到开心和激动")
	if decision.Emotion != Happy && decision.Emotion != Excited {
		t.Fatalf("expected happy/excited emotion, got %s", decision.Emotion)
	}
}
