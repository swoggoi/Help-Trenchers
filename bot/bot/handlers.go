package bot

import (
	"context"
	"mybot/bot/internal/keys"
	"mybot/bot/internal/storage"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Handlers struct {
	bot     *tgbotapi.BotAPI
	storage storage.Storage
}

func NewHandlers(bot *tgbotapi.BotAPI, storage storage.Storage) *Handlers {
	return &Handlers{
		bot:     bot,
		storage: storage,
	}
}

func (h *Handlers) HandleMessage(ctx context.Context, update tgbotapi.Update) {
	if update.Message == nil && update.CallbackQuery == nil {
		return
	}

	var chatID int64
	var fromID int64
	var text string

	if update.Message != nil {
		chatID = update.Message.Chat.ID
		fromID = update.Message.From.ID
		text = update.Message.Text

		user := &storage.User{
			ID:        fromID,
			Username:  update.Message.From.UserName,
			FirstName: update.Message.From.FirstName,
			LastName:  update.Message.From.LastName,
			State:     storage.StateIdle,
		}
		_, _ = h.storage.GetOrCreateUser(ctx, user)

		switch text {
		case "/start", "/help":
			h.sendMainMenu(ctx, chatID)
			return
		default:
			msg := tgbotapi.NewMessage(chatID, "Используй кнопки меню.")
			_, _ = h.bot.Send(msg)
			return
		}
	}

	if update.CallbackQuery != nil {
		h.handleCallback(ctx, update)
		return
	}
}

func (h *Handlers) sendMainMenu(ctx context.Context, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Главное меню:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ Информация", "info"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💳 Оплата", "pay"),
		),
	)
	_, _ = h.bot.Send(msg)
}

func (h *Handlers) handleCallback(ctx context.Context, update tgbotapi.Update) {
	if update.CallbackQuery == nil {
		return
	}

	chatID := update.CallbackQuery.Message.Chat.ID
	fromID := update.CallbackQuery.From.ID
	data := update.CallbackQuery.Data

	_, _ = h.bot.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))

	switch data {
	case "info":
		msg := tgbotapi.NewMessage(chatID, "Help Trenchers — бот для получения доступа к софту. После оплаты ты получаешь уникальный ключ, который даёт доступ к инструментам.")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("💳 Оплата", "pay"),
			),
		)
		_, _ = h.bot.Send(msg)

	case "pay":
		_ = h.storage.SetState(ctx, fromID, storage.StateChoosingPay)

		msg := tgbotapi.NewMessage(chatID, "Выбери способ оплаты:")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⭐ Звёзды", "stars"),
				tgbotapi.NewInlineKeyboardButtonData("🪙 Крипта", "crypto"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel"),
			),
		)
		_, _ = h.bot.Send(msg)

	case "stars", "crypto":
		_ = h.storage.SetState(ctx, fromID, storage.StateWaitingConfirm)

		msg := tgbotapi.NewMessage(chatID, "Ок. Теперь нажми «✅ Подтвердить оплату».")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Подтвердить оплату", "confirm"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel"),
			),
		)
		_, _ = h.bot.Send(msg)

	case "confirm":
		key := keys.Generate()

		err := h.storage.CreateKey(ctx, &storage.AccessKey{
			UserID:        fromID,
			Key:           key,
			PaymentMethod: "fake",
			IsUsed:        false,
			CreatedAt:     time.Now(),
			ExpiresAt:     time.Now().Add(24 * time.Hour),
		})
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, "Ошибка сохранения ключа.")
			_, _ = h.bot.Send(msg)
			_ = h.storage.SetState(ctx, fromID, storage.StateIdle)
			h.sendMainMenu(ctx, chatID)
			return
		}

		msg := tgbotapi.NewMessage(chatID, "Готово! Вот твой ключ:\n```"+key+"```")
		msg.ParseMode = "Markdown"
		_, _ = h.bot.Send(msg)

		_ = h.storage.SetState(ctx, fromID, storage.StateIdle)
		h.sendMainMenu(ctx, chatID)

	case "cancel":
		_ = h.storage.SetState(ctx, fromID, storage.StateIdle)
		msg := tgbotapi.NewMessage(chatID, "Отменено.")
		_, _ = h.bot.Send(msg)
		h.sendMainMenu(ctx, chatID)

	default:
		msg := tgbotapi.NewMessage(chatID, "Неизвестное действие.")
		_, _ = h.bot.Send(msg)
	}
}
