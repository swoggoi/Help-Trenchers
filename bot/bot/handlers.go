package bot

import (
	"context"
	"fmt"
	"mybot/bot/internal/keys"
	"mybot/bot/internal/logger"
	"mybot/bot/internal/payments"
	"mybot/bot/internal/storage"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Handlers struct {
	bot     *tgbotapi.BotAPI
	storage storage.Storage
	solana  *payments.SolanaClient
	np      *payments.NowPaymentsClient
	log     *logger.Logger
}

func NewHandlers(bot *tgbotapi.BotAPI, storage storage.Storage, sol *payments.SolanaClient, np *payments.NowPaymentsClient, log *logger.Logger) *Handlers {
	return &Handlers{
		bot:     bot,
		storage: storage,
		solana:  sol,
		np:      np,
		log:     log,
	}
}

type Plan struct {
	Days       int
	Price      float64
	PriceStars int
}

var Plans = []Plan{
	{Days: 7, Price: 7, PriceStars: 700},
	{Days: 14, Price: 18, PriceStars: 1800},
	{Days: 30, Price: 30, PriceStars: 3000},
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
	if update.PreCheckoutQuery != nil {
		h.handlePreCheckout(ctx, update.PreCheckoutQuery)
		return
	}

	if update.Message != nil && update.Message.SuccessfulPayment != nil {
		h.handleSuccessfulPayment(ctx, update.Message)
		return
	}

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
func (h *Handlers) handlePreCheckout(ctx context.Context, pq *tgbotapi.PreCheckoutQuery) {
	ans := tgbotapi.PreCheckoutConfig{
		OK:                 true,
		PreCheckoutQueryID: pq.ID,
	}
	if _, err := h.bot.Send(ans); err != nil {
		h.log.Errorf("precheckout ответ: %v", err)
	}
}

func (h *Handlers) handleSuccessfulPayment(ctx context.Context, msg *tgbotapi.Message) {
	p := msg.SuccessfulPayment
	// payload формата "sub_<userID>_<days>"
	var userID int64
	var days int
	if _, err := fmt.Sscanf(p.InvoicePayload, "sub_%d_%d", &userID, &days); err != nil {
		h.log.Errorf("bad payload %q: %v", p.InvoicePayload, err)
		return
	}
	plan, ok := planByDays(days)
	if !ok {
		return
	}

	key := keys.Generate()
	expires := time.Now().AddDate(0, 0, plan.Days)
	ak := &storage.AccessKey{
		UserID:        userID,
		Key:           key,
		PaymentMethod: "stars",
		DurationDays:  plan.Days,
		IsUsed:        false,
		CreatedAt:     time.Now(),
		ExpiresAt:     expires,
	}
	if err := h.storage.CreateKey(ctx, ak); err != nil {
		h.log.Errorf("сохранение ключа stars: %v", err)
		return
	}

	out := fmt.Sprintf("✅ Оплата через Telegram Stars подтверждена!\nТвой ключ:\n```%s```\nСрок: %d дней (до %s)",
		key, plan.Days, expires.Format("02.01.2006 15:04"))
	reply := tgbotapi.NewMessage(msg.Chat.ID, out)
	reply.ParseMode = "Markdown"
	if _, err := h.bot.Send(reply); err != nil {
		h.log.Errorf("отправка ключа stars: %v", err)
	}
	_ = h.storage.SetState(ctx, userID, storage.StateIdle)
	h.sendMainMenu(ctx, msg.Chat.ID)
}

func (h *Handlers) WatchPayments(ctx context.Context) {
	if h.solana == nil {
		return
	}
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			orders, err := h.storage.GetPendingOrders(ctx)
			if err != nil {
				continue
			}
			for _, o := range orders {
				if time.Since(o.CreatedAt) > 24*time.Hour {
					continue // expired order, skip (no auto-cancel for now)
				}
				sig, err := h.solana.Watch(ctx, o.DepositAddress, o.ExpectedLamports)
				if err != nil || sig == "" {
					continue
				}
				if _, ok := planByDays(o.PlanDays); !ok {
					continue
				}
				key := keys.Generate()
				ak := &storage.AccessKey{
					UserID:        o.UserID,
					Key:           key,
					PaymentMethod: "crypto",
					DurationDays:  o.PlanDays,
					IsUsed:        false,
					CreatedAt:     time.Now(),
					ExpiresAt:     time.Now().AddDate(0, 0, o.PlanDays),
				}
				if err := h.storage.CreateKey(ctx, ak); err != nil {
					continue
				}
				_ = h.storage.MarkOrderPaid(ctx, o.ID, sig, ak.ID)
				// sweep funds to main wallet
				_, _ = h.solana.Sweep(ctx, o.DepositPrivKey)

				msg := tgbotapi.NewMessage(o.UserID,
					fmt.Sprintf("✅ Оплата подтверждена!\nТвой ключ:\n```%s```\nСрок: %d дней", key, o.PlanDays))
				msg.ParseMode = "Markdown"
				_, _ = h.bot.Send(msg)
			}
		}
	}
}

func (h *Handlers) createNPInvoice(ctx context.Context, chatID, fromID int64, coin string) {
	if h.np == nil {
		msg := tgbotapi.NewMessage(chatID, "❌ NowPayments не настроен. Обратись к администратору.")
		_, _ = h.bot.Send(msg)
		h.sendMainMenu(ctx, chatID)
		return
	}
	plan := h.getSelectedPlan(ctx, fromID)
	if plan == nil {
		msg := tgbotapi.NewMessage(chatID, "Ошибка: тариф не выбран.")
		_, _ = h.bot.Send(msg)
		h.sendMainMenu(ctx, chatID)
		return
	}

	orderID := fmt.Sprintf("sub_%d_%d", fromID, plan.Days)
	ipnURL := os.Getenv("NOWPAYMENTS_IPN_URL")
	inv, err := h.np.CreateInvoice(ctx, plan.Price, coin, orderID, ipnURL)
	if err != nil {
		h.log.Errorf("NP create invoice: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Не удалось создать счёт на оплату. Попробуй позже.")
		_, _ = h.bot.Send(msg)
		h.sendMainMenu(ctx, chatID)
		return
	}

	order := &storage.Order{
		UserID:      fromID,
		PlanDays:    plan.Days,
		NpInvoiceID: inv.InvoiceID,
	}
	if err := h.storage.CreateOrder(ctx, order); err != nil {
		h.log.Errorf("NP save order: %v", err)
		msg := tgbotapi.NewMessage(chatID, "❌ Ошибка сохранения заказа.")
		_, _ = h.bot.Send(msg)
		return
	}

	text := fmt.Sprintf("💳 Счёт создан (%s)\n\nТариф: %d дней\nСумма к оплате: %s %s\n\n"+
		"Оплати по ссылке или переведи на адрес:\n`%s`\n\n"+
		"После поступления средств ключ придёт автоматически (до нескольких минут).",
		strings.ToUpper(coin), plan.Days, inv.PayAmount, strings.ToUpper(inv.PayCurrency), inv.PayAddress)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🔗 Оплатить", inv.InvoiceURL),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel"),
		),
	)
	_, _ = h.bot.Send(msg)
}

func (h *Handlers) WatchNPPayments(ctx context.Context) {
	if h.np == nil {
		return
	}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 1) pending-ордера: проверяем статус инвойса через API (polling)
			orders, err := h.storage.GetPendingOrders(ctx)
			if err == nil {
				for _, o := range orders {
					if o.NpInvoiceID == "" {
						continue
					}
					if time.Since(o.CreatedAt) > 24*time.Hour {
						continue
					}
					st, err := h.np.GetInvoiceStatus(ctx, o.NpInvoiceID)
					if err != nil || st == nil || !st.IsPaid() {
						continue
					}
					h.issueKeyForOrder(ctx, o)
				}
			}

			// 2) ордера, помеченные оплаченными через IPN, но без выданного ключа
			paid, err := h.storage.GetPaidUnissuedOrders(ctx)
			if err == nil {
				for _, o := range paid {
					h.issueKeyForOrder(ctx, o)
				}
			}
		}
	}
}

// issueKeyForOrder выдаёт ключ по ордеру (если ещё не выдан) и уведомляет юзера.
func (h *Handlers) issueKeyForOrder(ctx context.Context, o *storage.Order) {
	if o.KeyID != 0 {
		return
	}
	if _, ok := planByDays(o.PlanDays); !ok {
		return
	}
	key := keys.Generate()
	ak := &storage.AccessKey{
		UserID:        o.UserID,
		Key:           key,
		PaymentMethod: "nowpayments",
		DurationDays:  o.PlanDays,
		IsUsed:        false,
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().AddDate(0, 0, o.PlanDays),
	}
	if err := h.storage.CreateKey(ctx, ak); err != nil {
		return
	}
	sig := "np:" + o.NpInvoiceID
	if o.NpInvoiceID == "" {
		sig = "crypto:" + o.Signature
	}
	_ = h.storage.MarkOrderPaid(ctx, o.ID, sig, ak.ID)

	msg := tgbotapi.NewMessage(o.UserID,
		fmt.Sprintf("✅ Оплата подтверждена!\nТвой ключ:\n```%s```\nСрок: %d дней", key, o.PlanDays))
	msg.ParseMode = "Markdown"
	_, _ = h.bot.Send(msg)
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

	fmt.Printf("DEBUG callback data: %q from %d\n", data, fromID)

	switch data {
	case "info":
		msg := tgbotapi.NewMessage(chatID,
			"Help Trenchers — доступ к софту по подписке.\n\n"+
				"После оплаты ты получаешь уникальный ключ на выбранный срок.\n"+
				"Тарифы: 7 дней / 14 дней / 31 день.\n"+
				"Оплата: ⭐ Telegram Stars или 🪙 крипта (NOWPayments).")
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

		text := fmt.Sprintf("Тариф: %d дней\nЦена: $%.0f / %d ⭐\n\nВыбери способ оплаты:",
			plan.Days, plan.Price, plan.PriceStars)
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

	case "np_pay_btc", "np_pay_eth", "np_pay_usdt", "np_pay_usdc", "np_pay_sol", "np_pay_ton", "np_pay_trx", "np_pay_ltc":
		coin := strings.TrimPrefix(data, "np_pay_")
		h.createNPInvoice(ctx, chatID, fromID, coin)

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
				{Label: fmt.Sprintf("%d дней", plan.Days), Amount: plan.PriceStars},
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
		_ = h.storage.SetState(ctx, fromID, storage.StateChoosingPay)
		h.sendCoinMenu(chatID, plan)

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

func (h *Handlers) sendCoinMenu(chatID int64, plan *Plan) {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, coin := range payments.AvailableCurrencies {
		cb := fmt.Sprintf("np_pay_%s", coin)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(strings.ToUpper(coin), cb),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel"),
	))

	text := fmt.Sprintf("💳 Оплата криптой через NOWPayments\nТариф: %d дней (~$%.2f)\n\nВыбери монету для оплаты:",
		plan.Days, plan.Price)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	_, _ = h.bot.Send(msg)
}

func (h *Handlers) sendPlansMenu(chatID int64) {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, plan := range Plans {
		cb := fmt.Sprintf("plan_%d", plan.Days)
		label := fmt.Sprintf("%d дней — $%.0f / %d ⭐", plan.Days, plan.Price, plan.PriceStars)
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
