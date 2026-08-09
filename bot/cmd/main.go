package main

import (
	"context"
	"log"
	"mybot/bot/bot"
	"mybot/bot/internal/config"
	"mybot/bot/internal/logger"
	"mybot/bot/internal/payments"
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

	var sol *payments.SolanaClient
	if wallet := os.Getenv("SOL_WALLET"); wallet != "" {
		rpcURL := os.Getenv("SOL_RPC_URL")
		if rpcURL == "" {
			rpcURL = payments.DefaultRPCURL
		}
		sol, err = payments.NewSolanaClient(rpcURL, wallet)
		if err != nil {
			log.Printf("Предупреждение: Solana-клиент не создан (%v). Крипто-оплата не будет работать.", err)
		}
	} else {
		log.Println("SOL_WALLET не задан — крипто-оплата отключена")
	}

	b, err := bot.New(cfg.BotToken, logg, db, sol)
	if err != nil {
		log.Fatalf("Ошибка создания бота: %v", err)
	}

	
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		b.Start(ctx)
	}()

	<-sigCh
	cancel()
	logg.Info("shutdown")
}
