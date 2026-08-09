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

		// создаём/обновляем юзера в БД
		user := &storage.User{
			ID:        fromID,
			Username:  update.Message.From.UserName,
			FirstName: update.Message.From.FirstName,
			LastName:  update.Message.From.LastName,
			State:     storage.StateIdle,
		}
		_, _ = h.storage.GetOrCreateUser(ctx, user)
	}

	if update.CallbackQuery != nil {
		h.handleCallback(ctx, update)
		return
	}

	state, _ := h.storage.GetState(ctx, fromID)

	switch state {
	case storage.StateIdle:
		h.handleIdleState(ctx, chatID, fromID, text)
	case storage.StateChoosingPay:
		h.handleChoosingPayState(ctx, chatID, fromID, text)
	case storage.StateWaitingConfirm:
		h.handleWaitingConfirmState(ctx, chatID, fromID, text)
	}
}

// ... функции handleIdleState, handleChoosingPayState, handleWaitingConfirmState
// теперь принимают ctx первым аргументом и используют h.storage.CreateKey

func (h *Handlers) handleIdleState(ctx context.Context, chatID int64, fromID int64, text string) {
	switch text {
	case "/start", "/help":
		msg := tgbotapi.NewMessage(chatID, "Привет! Нажми /buy, чтобы начать.")
		_, _ = h.bot.Send(msg)

	case "/buy":
		_ = h.storage.SetState(ctx, fromID, storage.StateChoosingPay)

		msg := tgbotapi.NewMessage(chatID, "Выбери способ оплаты:")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("⭐ Звёзды"),
				tgbotapi.NewKeyboardButton("🪙 Крипта"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("❌ Отмена"),
			),
		)
		_, _ = h.bot.Send(msg)

	default:
		msg := tgbotapi.NewMessage(chatID, "Команда не найдена. Используй /start или /buy")
		_, _ = h.bot.Send(msg)
	}
}

func (h *Handlers) handleChoosingPayState(ctx context.Context, chatID int64, fromID int64, text string) {
	switch text {
	case "⭐ Звёзды", "🪙 Крипта":
		_ = h.storage.SetState(ctx, fromID, storage.StateWaitingConfirm)

		msg := tgbotapi.NewMessage(chatID, "Ок. Теперь нажми «✅ Подтвердить оплату».")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("✅ Подтвердить оплату"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("❌ Отмена"),
			),
		)
		_, _ = h.bot.Send(msg)

	case "❌ Отмена":
		_ = h.storage.SetState(ctx, fromID, storage.StateIdle)
		msg := tgbotapi.NewMessage(chatID, "Отменено.")
		_, _ = h.bot.Send(msg)

	default:
		msg := tgbotapi.NewMessage(chatID, "Выбери вариант кнопкой.")
		_, _ = h.bot.Send(msg)
	}
}

func (h *Handlers) handleWaitingConfirmState(ctx context.Context, chatID int64, fromID int64, text string) {
	switch text {
	case "✅ Подтвердить оплату":
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
			return
		}

		msg := tgbotapi.NewMessage(chatID, "Готово! Вот твой ключ:\n```"+key+"```")
		msg.ParseMode = "Markdown"
		_, _ = h.bot.Send(msg)

		_ = h.storage.SetState(ctx, fromID, storage.StateIdle)

	case "❌ Отмена":
		_ = h.storage.SetState(ctx, fromID, storage.StateIdle)
		msg := tgbotapi.NewMessage(chatID, "Отменено.")
		_, _ = h.bot.Send(msg)

	default:
		msg := tgbotapi.NewMessage(chatID, "Нажми кнопку «✅ Подтвердить оплату» или «❌ Отмена».")
		_, _ = h.bot.Send(msg)
	}
}

func (h *Handlers) handleCallback(ctx context.Context, update tgbotapi.Update) {
	if update.CallbackQuery == nil {
		return
	}

	_, _ = h.bot.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))
}
