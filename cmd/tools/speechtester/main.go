package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/zhouzirui/z-tavern/backend/internal/config"
	speechmodel "github.com/zhouzirui/z-tavern/backend/internal/model/speech"
	"github.com/zhouzirui/z-tavern/backend/internal/service/speech"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	if err := godotenv.Load(); err != nil {
		log.Printf("[WARN] 无法加载 .env，改用系统环境变量: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	if !cfg.Speech.Enabled {
		log.Fatal("语音服务未启用，请先在环境变量中配置 SPEECH_* 或 Ark 凭证")
	}

	mode := flag.String("mode", "", "测试模式: asr 或 tts")
	audioPath := flag.String("audio", "", "ASR 输入音频文件路径")
	text := flag.String("text", "", "TTS 输入文本")
	outputPath := flag.String("out", "", "TTS 输出音频文件路径 (默认根据格式自动生成)")
	format := flag.String("format", "", "音频格式 (ASR: 输入格式; TTS: 输出格式)")
	language := flag.String("lang", "", "语言代码，默认使用配置中的语言")
	voice := flag.String("voice", "", "TTS 声音 ID，默认使用配置中的 TTSVoice")
	session := flag.String("session", "", "自定义 sessionID，留空则自动生成")
	timeout := flag.Duration("timeout", 45*time.Second, "请求超时时间")

	flag.Parse()

	if *mode != "asr" && *mode != "tts" {
		flag.Usage()
		log.Fatal("请通过 -mode=asr 或 -mode=tts 指定测试模式")
	}

	sessionID := *session
	if sessionID == "" {
		sessionID = fmt.Sprintf("manual-%d", time.Now().UnixNano())
	}

	speechCfg := &speechmodel.SpeechConfig{
		AppID:       cfg.Speech.AppID,
		AccessToken: cfg.Speech.AccessToken,
		APIKey:      cfg.Speech.APIKey,
		AccessKey:   cfg.Speech.AccessKey,
		SecretKey:   cfg.Speech.SecretKey,
		Region:      cfg.Speech.Region,
		BaseURL:     cfg.Speech.BaseURL,
		ASRModel:    cfg.Speech.ASRModel,
		ASRLanguage: cfg.Speech.ASRLanguage,
		TTSVoice:    cfg.Speech.TTSVoice,
		TTSSpeed:    cfg.Speech.TTSSpeed,
		TTSVolume:   cfg.Speech.TTSVolume,
		TTSLanguage: cfg.Speech.TTSLanguage,
		Timeout:     cfg.Speech.Timeout,
	}

	svc := speech.NewService(speechCfg)
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	switch *mode {
	case "asr":
		runASR(ctx, svc, cfg, sessionID, *audioPath, *format, *language)
	case "tts":
		runTTS(ctx, svc, cfg, sessionID, *text, *voice, *format, *language, *outputPath)
	}
}

func runASR(ctx context.Context, svc *speech.Service, cfg *config.Config, sessionID, audioPath, format, language string) {
	if audioPath == "" {
		log.Fatal("ASR 模式需要通过 -audio 指定音频文件路径")
	}

	file, err := os.Open(audioPath)
	if err != nil {
		log.Fatalf("打开音频文件失败: %v", err)
	}
	defer file.Close()

	if format == "" {
		format = strings.TrimPrefix(strings.ToLower(filepath.Ext(audioPath)), ".")
		if format == "" {
			format = "wav"
		}
	}

	if language == "" {
		language = cfg.Speech.ASRLanguage
	}

	req := &speechmodel.ASRRequest{
		SessionID: sessionID,
		AudioData: file,
		Format:    format,
		Language:  language,
	}

	log.Printf("开始进行 ASR 测试: session=%s format=%s language=%s", sessionID, format, language)

	resp, err := svc.TranscribeAudio(ctx, req)
	if err != nil {
		log.Fatalf("ASR 调用失败: %v", err)
	}

	log.Printf("ASR 识别成功: text=%q confidence=%.2f duration=%dms", resp.Text, resp.Confidence, resp.Duration)
}

func runTTS(ctx context.Context, svc *speech.Service, cfg *config.Config, sessionID, text, voice, format, language, outputPath string) {
	if strings.TrimSpace(text) == "" {
		log.Fatal("TTS 模式需要通过 -text 提供待合成文本")
	}

	if voice == "" {
		voice = cfg.Speech.TTSVoice
	}

	if language == "" {
		language = cfg.Speech.TTSLanguage
	}

	if format == "" {
		format = "mp3"
	}

	if outputPath == "" {
		filename := fmt.Sprintf("tts-output-%d.%s", time.Now().Unix(), format)
		outputPath = filename
	}

	req := &speechmodel.TTSRequest{
		SessionID: sessionID,
		Text:      text,
		Voice:     voice,
		Format:    format,
		Language:  language,
	}

	log.Printf("开始进行 TTS 测试: session=%s voice=%s format=%s", sessionID, voice, format)

	resp, err := svc.SynthesizeSpeech(ctx, req)
	if err != nil {
		log.Fatalf("TTS 调用失败: %v", err)
	}

	if err := os.WriteFile(outputPath, resp.AudioData, 0o644); err != nil {
		log.Fatalf("写入音频文件失败: %v", err)
	}

	log.Printf("TTS 合成成功: 输出文件 %s, 时长=%dms", outputPath, resp.Duration)
}
