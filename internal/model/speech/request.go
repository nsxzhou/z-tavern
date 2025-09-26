package speech

import (
	"io"
)

// ASRRequest 语音识别请求
type ASRRequest struct {
	SessionID string    `json:"sessionId"`
	AudioData io.Reader `json:"-"`
	Format    string    `json:"format"`    // mp3, wav, webm, etc.
	Language  string    `json:"language"`  // zh-CN, en-US, etc.
}

// TTSRequest 语音合成请求
type TTSRequest struct {
	SessionID string  `json:"sessionId"`
	Text      string  `json:"text"`
	Voice     string  `json:"voice"`     // 声音类型
	Speed     float32 `json:"speed"`     // 语速倍率 0.5-2.0
	Volume    float32 `json:"volume"`    // 音量 0.0-1.0
	Format    string  `json:"format"`    // mp3, wav, etc.
	Language  string  `json:"language"`  // zh-CN, en-US, etc.
}