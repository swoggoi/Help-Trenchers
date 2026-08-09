package bot

import (
	"context"
	"fmt"
	"mybot/bot/internal/keys"
	"mybot/bot/internal/storage"
	"os"
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

type Plan struct {
	Days       int
	PriceSOL   string
	PriceStars int
}

var Plans = []Plan{
	{Days: 7, PriceSOL: "2.0", PriceStars: 200},
	{Days: 14, PriceSOL: "4.0", PriceStars: 400},
	{Days: 31, PriceSOL: "8.0", PriceStars: 800},
}

func planByDays(days int) (Plan, bool) {
	for _, p := range Plans {
		if p.Days == days {
			return p, true
		}
	}
	return Plan{}, false
}

func (h *Handlers) HandleMessage(ctx context.Context, update tgbotapi.Update) {
	if update.Message == nil && update.CallbackQuery == nil {
		return
	}

	var chatID int64
	var fromID int64

	if update.Message != nil {
		chatID = update.Message.Chat.ID
		fromID = update.Message.From.ID

		user := &storage.User{
			ID:        fromID,
			Username:  update.Message.From.UserName,
			FirstName: update.Message.From.FirstName,
			LastName:  update.Message.From.LastName,
			State:     storage.StateIdle,
		}
		_, _ = h.storage.GetOrCreateUser(ctx, user)

		switch update.Message.Text {
		case "/start", "/help":
			h.sendMainMenu(ctx, chatID)
		default:
			msg := tgbotapi.NewMessage(chatID, "Используй кнопки меню ниже.")
			_, _ = h.bot.Send(msg)
		}
		return
	}

	if update.CallbackQuery != nil {
		h.handleCallback(ctx, update)
	}
}

func (h *Handlers) sendMainMenu(ctx context.Context, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Главное меню Help Trenchers:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ Информация", "info"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💳 Подписка", "subscription"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👤 Профиль", "profile"),
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
		msg := tgbotapi.NewMessage(chatID,
			"Help Trenchers — доступ к софту по подписке.\n\n"+
				"После оплаты ты получаешь уникальный ключ на выбранный срок.\n"+
				"Тарифы: 7 дней / 14 дней / 31 день.\n"+
				"Оплата: ⭐ Telegram Stars или 🪙 крипта (SOL).")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("💳 Подписка", "subscription"),
			),
		)
		_, _ = h.bot.Send(msg)

	case "subscription":
		_ = h.storage.SetState(ctx, fromID, storage.StateChoosingPay)
		h.sendPlansMenu(chatID)

	case "profile":
		h.handleProfile(ctx, chatID, fromID)

	case "plan_7", "plan_14", "plan_31":
		days := 7
		switch data {
		case "plan_14":
			days = 14
		case "plan_31":
			days = 31
		}
		plan, ok := planByDays(days)
		if !ok {
			return
		}
		_ = h.storage.SetPendingDays(ctx, fromID, days)
		_ = h.storage.SetState(ctx, fromID, storage.StateWaitingConfirm)

		text := fmt.Sprintf("Тариф: %d дней\nЦена: %s SOL / %d ⭐\n\nВыбери способ оплаты:",
			plan.Days, plan.PriceSOL, plan.PriceStars)
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⭐ Telegram Stars", "pay_stars"),
				tgbotapi.NewInlineKeyboardButtonData("🪙 Крипта", "pay_crypto"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel"),
			),
		)
		_, _ = h.bot.Send(msg)

	case "pay_stars":
		plan := h.getSelectedPlan(ctx, fromID)
		if plan == nil {
			msg := tgbotapi.NewMessage(chatID, "Ошибка: тариф не выбран.")
			_, _ = h.bot.Send(msg)
			h.sendMainMenu(ctx, chatID)
			return
		}
		// Telegram Stars инвойс. Сумма в копейках Stars (1 XTR = 100).
		invoice := tgbotapi.NewInvoice(
			chatID,
			fmt.Sprintf("Help Trenchers — %d дней", plan.Days),
			"Доступ к софту на выбранный срок",
			fmt.Sprintf("sub_%d_%d", fromID, plan.Days),
			"",
			"",
			"XTR",
			[]tgbotapi.LabeledPrice{
				{Label: fmt.Sprintf("%d дней", plan.Days), Amount: plan.PriceStars * 100},
			},
		)
		_, _ = h.bot.Send(invoice)

	case "pay_crypto":
		plan := h.getSelectedPlan(ctx, fromID)
		if plan == nil {
			msg := tgbotapi.NewMessage(chatID, "Ошибка: тариф не выбран.")
			_, _ = h.bot.Send(msg)
			h.sendMainMenu(ctx, chatID)
			return
		}
		wallet := os.Getenv("SOL_WALLET")
		if wallet == "" {
			wallet = "YOUR_SOL_WALLET_ADDRESS"
		}
		text := fmt.Sprintf("Оплата криптой (SOL):\n\nТариф: %d дней\nСумма: %s SOL\n\n"+
			"Кошелёк Solana:\n`%s`\n\nПосле перевода нажми «✅ Подтвердить оплату» и пришли TXID ответным сообщением.",
			plan.Days, plan.PriceSOL, wallet)
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Подтвердить оплату", "confirm_crypto"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel"),
			),
		)
		_, _ = h.bot.Send(msg)

	case "confirm_crypto":
		plan := h.getSelectedPlan(ctx, fromID)
		if plan == nil {
			plan = &Plans[0]
		}
		h.issueKey(ctx, chatID, fromID, plan, "crypto")

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

func (h *Handlers) sendPlansMenu(chatID int64) {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, plan := range Plans {
		cb := fmt.Sprintf("plan_%d", plan.Days)
		label := fmt.Sprintf("%d дней — %s SOL / %d ⭐", plan.Days, plan.PriceSOL, plan.PriceStars)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, cb),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel"),
	))

	msg := tgbotapi.NewMessage(chatID, "Выбери тариф подписки:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	_, _ = h.bot.Send(msg)
}

func (h *Handlers) getSelectedPlan(ctx context.Context, userID int64) *Plan {
	days, err := h.storage.GetPendingDays(ctx, userID)
	if err != nil || days == 0 {
		return &Plans[0]
	}
	plan, ok := planByDays(days)
	if !ok {
		return &Plans[0]
	}
	return &plan
}

func (h *Handlers) issueKey(ctx context.Context, chatID int64, fromID int64, plan *Plan, method string) {
	key := keys.Generate()
	expires := time.Now().AddDate(0, 0, plan.Days)

	err := h.storage.CreateKey(ctx, &storage.AccessKey{
		UserID:        fromID,
		Key:           key,
		PaymentMethod: method,
		DurationDays:  plan.Days,
		IsUsed:        false,
		CreatedAt:     time.Now(),
		ExpiresAt:     expires,
	})
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Ошибка сохранения ключа.")
		_, _ = h.bot.Send(msg)
		_ = h.storage.SetState(ctx, fromID, storage.StateIdle)
		h.sendMainMenu(ctx, chatID)
		return
	}

	msg := tgbotapi.NewMessage(chatID,
		fmt.Sprintf("✅ Готово! Твой ключ:\n```%s```\nСрок: %d дней (до %s)",
			key, plan.Days, expires.Format("02.01.2006 15:04")))
	msg.ParseMode = "Markdown"
	_, _ = h.bot.Send(msg)

	_ = h.storage.SetState(ctx, fromID, storage.StateIdle)
	h.sendMainMenu(ctx, chatID)
}

func (h *Handlers) handleProfile(ctx context.Context, chatID int64, userID int64) {
	key, err := h.storage.GetUserActiveKey(ctx, userID)
	if err != nil || key == nil {
		msg := tgbotapi.NewMessage(chatID,
			"👤 Профиль\n\nУ тебя нет активного ключа.\nНажми «💳 Подписка», чтобы получить доступ.")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("💳 Подписка", "subscription"),
			),
		)
		_, _ = h.bot.Send(msg)
		return
	}

	remaining := time.Until(key.ExpiresAt)
	daysLeft := int(remaining.Hours() / 24)
	hoursLeft := int(remaining.Hours()) % 24

	text := fmt.Sprintf("👤 Профиль\n\n"+
		"Ключ: `%s`\n"+
		"Тариф: %d дней\n"+
		"Осталось: %d дн %d ч\n"+
		"Истекает: %s",
		key.Key, key.DurationDays, daysLeft, hoursLeft, key.ExpiresAt.Format("02.01.2006 15:04"))

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💳 Продлить", "subscription"),
		),
	)
	_, _ = h.bot.Send(msg)
}
