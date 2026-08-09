package bot

import (
	"context"
	"mybot/bot/internal/logger"
	"mybot/bot/internal/payments"
	"mybot/bot/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api      *tgbotapi.BotAPI
	log      *logger.Logger
	storage  storage.Storage
	handlers *Handlers
}

func New(token string, log *logger.Logger, storage storage.Storage, sol *payments.SolanaClient, np *payments.NowPaymentsClient) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	api.Debug = false

	return &Bot{
		api:      api,
		log:      log,
		storage:  storage,
		handlers: NewHandlers(api, storage, sol, np, log),
	}, nil
}

func (b *Bot) Start(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	b.log.Infof("Бот запущен: %s", b.api.Self.UserName)

	// запуск watcher крипто-платежей (прямой SOL)
	go b.handlers.WatchPayments(ctx)
	// запуск watcher NowPayments (крипта через NP)
	go b.handlers.WatchNPPayments(ctx)

	for update := range updates {
		b.handlers.HandleMessage(ctx, update)
	}
}
