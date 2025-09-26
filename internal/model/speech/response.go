package speech

import "time"

// ASRResponse 语音识别响应
type ASRResponse struct {
	SessionID  string    `json:"sessionId"`
	Text       string    `json:"text"`
	Confidence float64   `json:"confidence"`
	Duration   int64     `json:"duration"` // milliseconds
	RequestID  string    `json:"requestId,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// TTSResponse 语音合成响应
type TTSResponse struct {
	SessionID string    `json:"sessionId"`
	AudioData []byte    `json:"-"`
	AudioURL  string    `json:"audioUrl,omitempty"`
	Duration  int64     `json:"duration"` // milliseconds
	Format    string    `json:"format"`
	RequestID string    `json:"requestId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// StreamingASRChunk 流式ASR数据块
type StreamingASRChunk struct {
	SessionID  string    `json:"sessionId"`
	Text       string    `json:"text"`
	IsFinal    bool      `json:"isFinal"`
	Confidence float64   `json:"confidence"`
	StartTime  int64     `json:"startTime"`  // milliseconds from start
	EndTime    int64     `json:"endTime"`    // milliseconds from start
	RequestID  string    `json:"requestId,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}