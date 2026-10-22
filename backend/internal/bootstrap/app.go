package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"tzwall/backend/internal/ai"
	"tzwall/backend/internal/cache"
	"tzwall/backend/internal/config"
	"tzwall/backend/internal/database"
	"tzwall/backend/internal/handler"
	"tzwall/backend/internal/mailer"
	"tzwall/backend/internal/moderation"
	"tzwall/backend/internal/pkg/logger"
	"tzwall/backend/internal/service"
	"tzwall/backend/internal/storage"
)

// firstNonEmpty 返回第一个非空字符串，用于配置缺省回退。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// Run 装配依赖、迁移/种子、启动 HTTP 服务并优雅退出。
func Run() error {
	cfg := config.Load()
	log := logger.New()

	db, err := database.Open(cfg, log)
	if err != nil {
		return err
	}
	if err := database.Migrate(db, log); err != nil {
		return err
	}
	if err := database.Seed(db, cfg, log); err != nil {
		return err
	}

	c, _ := cache.New(cfg)
	store := service.NewConfigStoreWithKey(db, cfg.AESKey)
	// 源站缓存目录与上限：后台配置优先，回退环境变量与默认值；修改需重启
	mediaCacheDir := firstNonEmpty(store.Get("media_cache_dir"), cfg.MediaCacheDir, "./data/media_cache")
	mediaCacheMaxGB := cfg.MediaCacheMaxGB
	if v := store.Get("media_cache_max_gb"); v != "" {
		if n, e := strconv.Atoi(v); e == nil && n > 0 {
			mediaCacheMaxGB = n
		}
	}
	st := storage.New(cfg, mediaCacheDir, mediaCacheMaxGB)
	mod := moderation.New(cfg, db)

	// AI 审核：配置来自后台系统配置（加密存储），保存即生效
	aiClient := ai.NewClient(func() ai.Settings {
		return ai.Settings{
			Enabled:    store.BoolValue("ai_enabled", false),
			BaseURL:    store.Get("ai_base_url"),
			APIKey:     store.SecretValue("ai_api_key"),
			Model:      store.Get("ai_model"),
			Prompt:     firstNonEmpty(store.Get("ai_prompt"), config.DefaultAIPrompt),
			Timeout:    time.Duration(store.Int("ai_timeout_seconds", 10)) * time.Second,
			DailyLimit: store.Int("ai_daily_limit", 500),
		}
	}, c)

	// 邮件：后台保存 SMTP 配置后下一次发信即生效（.env 优先）
	mailSettings := func() mailer.Settings {
		s := mailer.Settings{
			Host: store.Get("smtp_host"),
			User: store.Get("smtp_user"),
			Pass: store.SecretValue("smtp_pass"),
			From: store.Get("smtp_from"),
		}
		if port := store.Get("smtp_port"); port != "" {
			s.Port, _ = strconv.Atoi(port)
		}
		if s.Host == "" || s.User == "" {
			s = mailer.Settings{
				Host: cfg.SmtpHost,
				Port: cfg.SmtpPort,
				User: cfg.SmtpUser,
				Pass: cfg.SmtpPass,
				From: cfg.SmtpFrom,
			}
		}
		return s
	}
	m := mailer.NewDynamic(mailSettings)

	// 审核轮询（回调不可用时的兜底）
	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	moderation.NewWorker(db, mod, log).Start(workerCtx)

	r := handler.SetupRouter(handler.Deps{
		Cfg:        cfg,
		DB:         db,
		Cache:      c,
		Storage:    st,
		Moderation: mod,
		Mailer:     m,
		Store:      store,
		AI:         aiClient,
	})

	srv := &http.Server{
		Addr:    cfg.ServerHost + ":" + cfg.ServerPort,
		Handler: r,
	}

	go func() {
		log.Info("服务启动", "port", cfg.ServerPort, "storage", cfg.StorageConfigured, "mailer", cfg.MailerConfigured)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("服务异常退出", "err", err.Error())
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("正在关闭服务...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}
	_ = c.Close()
	log.Info("服务已停止")
	return nil
}
