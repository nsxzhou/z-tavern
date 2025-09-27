package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/components/model"
)

// Config 聚合整个服务的配置项。
type Config struct {
	Server ServerConfig
	AI     AIConfig
	Speech SpeechConfig
}

// Load 从环境变量加载配置。
func Load() (*Config, error) {
	server, err := loadServerConfig()
	if err != nil {
		return nil, err
	}

	ai, err := loadAIConfig()
	if err != nil {
		return nil, err
	}

	speech, err := loadSpeechConfig()
	if err != nil {
		return nil, err
	}

	return &Config{Server: server, AI: ai, Speech: speech}, nil
}

// ServerConfig 描述 HTTP 服务配置。
type ServerConfig struct {
	Addr string
}

// loadServerConfig 解析服务器监听地址。
func loadServerConfig() (ServerConfig, error) {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	if strings.Contains(port, ":") {
		// 允许用户直接传入 ":8080" 或 "127.0.0.1:8080"。
		return ServerConfig{Addr: port}, nil
	}

	if strings.Contains(port, " ") {
		return ServerConfig{}, fmt.Errorf("invalid PORT value: %q", port)
	}

	return ServerConfig{Addr: ":" + port}, nil
}

// AIConfig 描述大模型相关配置。
type AIConfig struct {
	APIKey              string
	AccessKey           string
	SecretKey           string
	Model               string
	BaseURL             string
	Region              string
	Temperature         *float64
	TopP                *float64
	MaxTokens           *int
	StreamResponse      bool
	EmotionLLMEnabled   bool
	EmotionHistoryLimit int
}

// SpeechConfig 描述语音服务相关配置
type SpeechConfig struct {
	AppID       string
	AccessToken string
	APIKey      string
	AccessKey   string
	SecretKey   string
	Region      string
	BaseURL     string
	ASRModel    string
	ASRLanguage string
	TTSVoice    string
	TTSSpeed    float32
	TTSVolume   float32
	TTSLanguage string
	Timeout     int
	Enabled     bool
}

// Enabled 表示是否提供了必需的密钥。
func (c AIConfig) Enabled() bool {
	return c.Model != "" && (c.APIKey != "" || (c.AccessKey != "" && c.SecretKey != ""))
}

// NewChatModel 使用配置创建一个模型实例。
func (c AIConfig) NewChatModel(ctx context.Context) (model.ChatModel, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("Ark 凭证或模型配置缺失，至少提供 ARK_API_KEY + Model 或 AK/SK 组合")
	}

	var temperature *float32
	if c.Temperature != nil {
		val := float32(*c.Temperature)
		temperature = &val
	}

	var topP *float32
	if c.TopP != nil {
		val := float32(*c.TopP)
		topP = &val
	}

	var maxTokens *int
	if c.MaxTokens != nil {
		val := *c.MaxTokens
		maxTokens = &val
	}

	cfg := &ark.ChatModelConfig{
		BaseURL:     c.BaseURL,
		Region:      c.Region,
		APIKey:      c.APIKey,
		AccessKey:   c.AccessKey,
		SecretKey:   c.SecretKey,
		Model:       c.Model,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		TopP:        topP,
	}

	return ark.NewChatModel(ctx, cfg)
}

func loadAIConfig() (AIConfig, error) {
	temperature, err := parseOptionalFloatEnv("ARK_TEMPERATURE")
	if err != nil {
		return AIConfig{}, err
	}

	topP, err := parseOptionalFloatEnv("ARK_TOP_P")
	if err != nil {
		return AIConfig{}, err
	}

	maxTokens, err := parseOptionalIntEnv("ARK_MAX_TOKENS")
	if err != nil {
		return AIConfig{}, err
	}

	stream, err := parseBoolEnv("ARK_STREAM", true)
	if err != nil {
		return AIConfig{}, err
	}

	emotionEnabled, err := parseBoolEnv("AI_EMOTION_LLM_ENABLED", false)
	if err != nil {
		return AIConfig{}, err
	}

	emotionHistory := 6
	if historyOverride, err := parseOptionalIntEnv("AI_EMOTION_HISTORY_LIMIT"); err != nil {
		return AIConfig{}, err
	} else if historyOverride != nil {
		if *historyOverride < 1 {
			emotionHistory = 1
		} else {
			emotionHistory = *historyOverride
		}
	}

	return AIConfig{
		APIKey:              strings.TrimSpace(os.Getenv("ARK_API_KEY")),
		AccessKey:           strings.TrimSpace(os.Getenv("ARK_ACCESS_KEY")),
		SecretKey:           strings.TrimSpace(os.Getenv("ARK_SECRET_KEY")),
		Model:               strings.TrimSpace(os.Getenv("Model")),
		BaseURL:             getEnvOrDefault("ARK_BASE_URL", "https://ark.cn-beijing.volces.com/api/v3"),
		Region:              getEnvOrDefault("ARK_REGION", "cn-beijing"),
		Temperature:         temperature,
		TopP:                topP,
		MaxTokens:           maxTokens,
		StreamResponse:      stream,
		EmotionLLMEnabled:   emotionEnabled,
		EmotionHistoryLimit: emotionHistory,
	}, nil
}

func loadSpeechConfig() (SpeechConfig, error) {
	// 解析超时设置
	timeout, err := parseOptionalIntEnv("SPEECH_TIMEOUT")
	if err != nil {
		return SpeechConfig{}, err
	}
	timeoutSeconds := 30 // 默认30秒
	if timeout != nil {
		timeoutSeconds = *timeout
	}

	// 解析TTS速度和音量
	speed, err := parseOptionalFloat32Env("SPEECH_TTS_SPEED")
	if err != nil {
		return SpeechConfig{}, err
	}
	ttsSpeed := float32(1.0) // 默认1.0倍速
	if speed != nil {
		ttsSpeed = *speed
	}

	volume, err := parseOptionalFloat32Env("SPEECH_TTS_VOLUME")
	if err != nil {
		return SpeechConfig{}, err
	}
	ttsVolume := float32(1.0) // 默认1.0音量
	if volume != nil {
		ttsVolume = *volume
	}

	appID := strings.TrimSpace(os.Getenv("SPEECH_APP_ID"))

	accessToken := strings.TrimSpace(os.Getenv("SPEECH_ACCESS_TOKEN"))
	apiKey := strings.TrimSpace(os.Getenv("SPEECH_API_KEY"))
	if accessToken == "" {
		accessToken = apiKey
	}

	accessKey := strings.TrimSpace(os.Getenv("SPEECH_ACCESS_KEY"))
	secretKey := strings.TrimSpace(os.Getenv("SPEECH_SECRET_KEY"))

	// 如果没有专门的语音配置，尝试使用AI配置
	if accessToken == "" && accessKey == "" {
		accessToken = strings.TrimSpace(os.Getenv("ARK_API_KEY"))
		apiKey = accessToken
		accessKey = strings.TrimSpace(os.Getenv("ARK_ACCESS_KEY"))
		secretKey = strings.TrimSpace(os.Getenv("ARK_SECRET_KEY"))
	}

	enabled := appID != "" && accessToken != ""

	return SpeechConfig{
		AppID:       appID,
		AccessToken: accessToken,
		APIKey:      apiKey,
		AccessKey:   accessKey,
		SecretKey:   secretKey,
		Region:      getEnvOrDefault("SPEECH_REGION", "cn-beijing"),
		BaseURL:     getEnvOrDefault("SPEECH_BASE_URL", ""),
		ASRModel:    getEnvOrDefault("SPEECH_ASR_MODEL", ""),
		ASRLanguage: getEnvOrDefault("SPEECH_ASR_LANGUAGE", "zh-CN"),
		TTSVoice:    getEnvOrDefault("SPEECH_TTS_VOICE", ""),
		TTSSpeed:    ttsSpeed,
		TTSVolume:   ttsVolume,
		TTSLanguage: getEnvOrDefault("SPEECH_TTS_LANGUAGE", "zh-CN"),
		Timeout:     timeoutSeconds,
		Enabled:     enabled,
	}, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return defaultValue
}

func parseBoolEnv(key string, defaultValue bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue, nil
	}

	val, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("invalid %s value %q: %w", key, raw, err)
	}
	return val, nil
}

func parseOptionalFloatEnv(key string) (*float64, error) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return nil, nil
	}

	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}

	val, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid %s value %q: %w", key, value, err)
	}
	return &val, nil
}

func parseOptionalIntEnv(key string) (*int, error) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return nil, nil
	}

	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}

	val, err := strconv.Atoi(value)
	if err != nil {
		return nil, fmt.Errorf("invalid %s value %q: %w", key, value, err)
	}
	return &val, nil
}

func parseOptionalFloat32Env(key string) (*float32, error) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return nil, nil
	}

	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}

	val, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid %s value %q: %w", key, value, err)
	}
	result := float32(val)
	return &result, nil
}
