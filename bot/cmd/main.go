package main

import (
	"context"
	"log"
	"mybot/bot/bot"
	"mybot/bot/internal/config"
	"mybot/bot/internal/logger"
	"mybot/bot/internal/storage"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Ошибка загрузки конфига: %v", err)
	}

	logg, err := logger.New()
	if err != nil {
		log.Fatalf("Ошибка инициализации логгера: %v", err)
	}

	db, err := storage.NewPostgresStorage(ctx, cfg.DBURL)
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	defer db.Close()

	b, err := bot.New(cfg.BotToken, logg, db)
	if err != nil {
		log.Fatalf("Ошибка создания бота: %v", err)
	}

	// graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		b.Start(ctx)
	}()

	<-sigCh
	cancel()
	logg.Info("shutdown")
}
