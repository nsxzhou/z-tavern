package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/joho/godotenv"
	"github.com/zhouzirui/z-tavern/backend/internal/config"
	"github.com/zhouzirui/z-tavern/backend/internal/handler"
	"github.com/zhouzirui/z-tavern/backend/internal/model/persona"
	speechModel "github.com/zhouzirui/z-tavern/backend/internal/model/speech"
	"github.com/zhouzirui/z-tavern/backend/internal/service/ai"
	"github.com/zhouzirui/z-tavern/backend/internal/service/chat"
	emotionservice "github.com/zhouzirui/z-tavern/backend/internal/service/emotion"
	"github.com/zhouzirui/z-tavern/backend/internal/service/speech"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("warning: failed to load .env file: %v", err)
		log.Println("continuing with system environment variables only")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Initialize persona store and chat service
	personaStore := persona.NewMemoryStore(persona.Seed())
	chatService := chat.NewService()

	// Initialize AI service
	var aiService *ai.Service
	if cfg.AI.Enabled() {
		aiService, err = ai.NewService(ctx, personaStore, cfg.AI)
		if err != nil {
			log.Printf("warning: failed to initialize AI service: %v", err)
			log.Println("continuing without AI functionality - 请检查 Ark 模型相关环境变量")
		} else {
			log.Println("AI service initialized successfully")
		}
	} else {
		log.Println("Ark 凭证未配置，跳过 AI 功能初始化")
	}

	// Initialize emotion analysis service (LLM-based guidance with fallback)
	emotionCfg := emotionservice.Config{
		Enabled:      cfg.AI.EmotionLLMEnabled,
		HistoryLimit: cfg.AI.EmotionHistoryLimit,
	}
	var chatModelForEmotion model.ChatModel
	if aiService != nil {
		chatModelForEmotion = aiService.GetChatModel()
	}
	emotionSvc, err := emotionservice.NewService(ctx, chatModelForEmotion, emotionCfg)
	if err != nil {
		log.Printf("warning: failed to initialize emotion service: %v", err)
		emotionSvc = nil
	} else if emotionSvc != nil && emotionSvc.Enabled() {
		log.Println("Emotion classifier service enabled")
	} else if emotionCfg.Enabled {
		log.Println("Emotion classifier requested but chat model unavailable, falling back to heuristics")
	} else {
		log.Println("Emotion classifier disabled by configuration")
	}

	// Initialize Speech service
	var speechService *speech.Service
	if cfg.Speech.Enabled {
		speechConfig := &speechModel.SpeechConfig{
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
		speechService = speech.NewService(speechConfig)
		log.Println("Speech service initialized successfully")
	} else {
		log.Println("语音服务凭证未配置，跳过语音功能初始化")
	}

	router := handler.NewRouter(personaStore, chatService, aiService, emotionSvc, speechService)

	startServer(ctx, cfg.Server, router)
}

func startServer(ctx context.Context, serverCfg config.ServerConfig, router http.Handler) {
	addr := serverCfg.Addr
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Printf("Z Tavern backend listening on %s", addr)
	if err := runServer(ctx, srv); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func runServer(ctx context.Context, srv *http.Server) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		err := <-errCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
