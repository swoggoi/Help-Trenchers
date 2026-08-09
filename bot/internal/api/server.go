package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"mybot/bot/internal/payments"
	"mybot/bot/internal/storage"
)

type Server struct {
	storage storage.Storage
	np      *payments.NowPaymentsClient
	apiKey  string
	srv     *http.Server
}

func New(storage storage.Storage, addr, apiKey string, np *payments.NowPaymentsClient) *Server {
	s := &Server{
		storage: storage,
		np:      np,
		apiKey:  apiKey,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/verify", s.handleVerify)
	mux.HandleFunc("/api/nowpayments/callback", s.handleNPCallback)
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	s.srv = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

func (s *Server) Start() error {
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		var body struct {
			Key string `json:"key"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		key = body.Key
	}

	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"valid": false,
			"error": "key required",
		})
		return
	}

	ak, err := s.storage.VerifyKey(r.Context(), key)
	if err != nil || ak == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"valid": false,
			"error": "key not found",
		})
		return
	}

	valid := !ak.IsUsed && time.Now().Before(ak.ExpiresAt)
	writeJSON(w, http.StatusOK, map[string]any{
		"valid":          valid,
		"user_id":        ak.UserID,
		"duration_days":  ak.DurationDays,
		"expires_at":     ak.ExpiresAt.Format(time.RFC3339),
		"payment_method": ak.PaymentMethod,
	})
}

// handleNPCallback обрабатывает webhook от NOWPayments.
// Логику выдачи ключа выполняет WatchNPPayments (polling статуса).
// Здесь мы только валидируем подпись и, при успешном статусе,
// помечаем соответствующий ордер оплаченным, чтобы ускорить выдачу.
func (s *Server) handleNPCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	if s.np != nil {
		sig := r.Header.Get("x-nowpayments-signature")
		if !s.np.VerifyIPN(body, sig) {
			http.Error(w, "bad signature", http.StatusForbidden)
			return
		}
	}

	var ipn payments.StatusIPN
	if err := json.Unmarshal(body, &ipn); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	// Если платёж подтверждён — помечаем ордер оплаченным.
	// WatchNPPayments довыдаст ключ. Если IPN вдруг раньше polling-а,
	// ключ выдастся именно отсюда не будем дублировать — оставим watcher-у.
	if ipn.IsPaid() && ipn.InvoiceID != "" {
		orders, err := s.storage.GetPendingOrders(r.Context())
		if err == nil {
			for _, o := range orders {
				if o.NpInvoiceID == ipn.InvoiceID {
					_ = s.storage.MarkOrderPaid(r.Context(), o.ID, "np:"+ipn.InvoiceID, 0)
					break
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
