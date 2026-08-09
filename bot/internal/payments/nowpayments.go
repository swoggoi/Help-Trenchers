package payments

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NowPaymentsClient — клиент к NOWPayments API (оплата криптой).
type NowPaymentsClient struct {
	apiKey    string
	ipnSecret string
	baseURL   string
	client    *http.Client
}

func NewNowPaymentsClient(apiKey, ipnSecret string) *NowPaymentsClient {
	return &NowPaymentsClient{
		apiKey:    apiKey,
		ipnSecret: ipnSecret,
		baseURL:   "https://api.nowpayments.io/v1",
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

// AvailableCurrencies — список монет, которые предлагаем юзеру.
var AvailableCurrencies = []string{"btc", "eth", "usdt", "usdc", "sol", "ton", "trx", "ltc"}

type npInvoiceReq struct {
	PriceAmount    float64 `json:"price_amount"`
	PriceCurrency  string  `json:"price_currency"`
	PayCurrency    string  `json:"pay_currency"`
	OrderID        string  `json:"order_id"`
	IPNCallbackURL string  `json:"ipn_callback_url,omitempty"`
	OrderDescription string `json:"order_description,omitempty"`
}

type NPInvoice struct {
	InvoiceID     string  `json:"invoice_id"`
	InvoiceURL    string  `json:"invoice_url"`
	PayAddress    string  `json:"pay_address"`
	PayAmount     string `json:"pay_amount"`
	PayCurrency   string `json:"pay_currency"`
	PriceAmount   string `json:"price_amount"`
	PriceCurrency string  `json:"price_currency"`
	OrderID       string  `json:"order_id"`
}

func (c *NowPaymentsClient) CreateInvoice(ctx context.Context, price float64, payCurrency, orderID, ipnURL string) (*NPInvoice, error) {
	req := npInvoiceReq{
		PriceAmount:    price,
		PriceCurrency:  "usd",
		PayCurrency:    payCurrency,
		OrderID:        orderID,
		IPNCallbackURL: ipnURL,
		OrderDescription: "Help Trenchers subscription",
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/invoice", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("nowpayments invoice status %d: %s", resp.StatusCode, string(b))
	}

	var inv NPInvoice
	if err := json.NewDecoder(resp.Body).Decode(&inv); err != nil {
		return nil, err
	}
	return &inv, nil
}

// NPInvoiceStatus — ответ статуса инвойса.
type NPInvoiceStatus struct {
	InvoiceID     string `json:"invoice_id"`
	PaymentStatus string `json:"payment_status"`
	PayAddress    string `json:"pay_address"`
	PayAmount     string `json:"pay_amount"`
	PayCurrency   string `json:"pay_currency"`
	OrderID       string `json:"order_id"`
}

// StatusFinished возвращает true, когда средства можно считать полученными.
func (s NPInvoiceStatus) IsPaid() bool {
	switch s.PaymentStatus {
	case "finished", "confirmed", "paid", "partially_paid":
		return true
	}
	return false
}

func (c *NowPaymentsClient) GetInvoiceStatus(ctx context.Context, invoiceID string) (*NPInvoiceStatus, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/invoice/"+invoiceID, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("x-api-key", c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("nowpayments status %d: %s", resp.StatusCode, string(b))
	}

	var st NPInvoiceStatus
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return nil, err
	}
	return &st, nil
}

// StatusIPN — входящий webhook от NOWPayments.
type StatusIPN struct {
	InvoiceID       string  `json:"invoice_id"`
	OrderID         string  `json:"order_id"`
	PaymentStatus   string  `json:"payment_status"`
	PayAddress      string  `json:"pay_address"`
	PriceAmount     float64 `json:"price_amount"`
	PriceCurrency   string  `json:"price_currency"`
	PayAmount       float64 `json:"pay_amount"`
	PayCurrency     string  `json:"pay_currency"`
	ActuallyPaid    float64 `json:"actually_paid"`
	OutcomeAmount   float64 `json:"outcome_amount"`
	OutcomeCurrency string  `json:"outcome_currency"`
}

func (ipn StatusIPN) IsPaid() bool {
	switch ipn.PaymentStatus {
	case "finished", "confirmed", "paid", "partially_paid":
		return true
	}
	return false
}

// VerifyIPN проверяет HMAC-SHA512 подпись webhook по ipn_secret.
// NOWPayments шлёт заголовок x-nowpayments-signature = HEX(HMAC_SHA512(body, ipn_secret)).
func (c *NowPaymentsClient) VerifyIPN(body []byte, signature string) bool {
	if c.ipnSecret == "" {
		return false
	}
	mac := hmac.New(sha512.New, []byte(c.ipnSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
