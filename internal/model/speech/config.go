package speech

// SpeechConfig 语音服务配置
type SpeechConfig struct {
	// Volcengine 配置
	AppID          string `json:"appId"`            // 火山引擎 APP ID
	AccessToken    string `json:"accessToken"`      // 火山引擎 Access Token
	APIKey         string `json:"apiKey,omitempty"` // 兼容旧配置的 API Key
	AccessKey      string `json:"accessKey"`        // Access Key（备选方式）
	SecretKey      string `json:"secretKey"`        // Secret Key（备选方式）
	Region         string `json:"region"`           // 服务区域
	BaseURL        string `json:"baseUrl"`          // 基础URL
	ConcurrentMode bool   `json:"concurrentMode"`   // ASR并发模式（false为小时版）

	// ASR 配置
	ASRModel    string `json:"asrModel"`
	ASRLanguage string `json:"asrLanguage"`

	// TTS 配置
	TTSVoice    string  `json:"ttsVoice"`
	TTSSpeed    float32 `json:"ttsSpeed"`
	TTSVolume   float32 `json:"ttsVolume"`
	TTSLanguage string  `json:"ttsLanguage"`

	// 通用配置
	Timeout int `json:"timeout"` // seconds
}
