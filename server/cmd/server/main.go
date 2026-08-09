package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mybot/server/internal/hub"
	"mybot/server/internal/pump"
	"mybot/server/internal/store"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		log.Fatal("DB_URL обязателен")
	}

	st, err := store.New(ctx, dsn)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	// импорт devs.txt (seed для Twitter-модуля, если файл есть)
	if path := os.Getenv("DEVS_TXT"); path != "" {
		n, err := st.ImportDevsTxt(ctx, path)
		if err != nil {
			log.Printf("импорт devs.txt: %v", err)
		} else {
			log.Printf("импортировано Twitter-аккаунтов: %d", n)
		}
	}

	// авто-загрузка кошельков девов из файлов
	if p := os.Getenv("DEV_WALLETS_GOOD"); p != "" {
		n, e := st.LoadWalletsFile(ctx, p, "good")
		log.Printf("загружено good-кошельков: %d (%v)", n, e)
	}
	if p := os.Getenv("DEV_WALLETS_SCAM"); p != "" {
		n, e := st.LoadWalletsFile(ctx, p, "scam")
		log.Printf("загружено scam-кошельков: %d (%v)", n, e)
	}

	h := hub.New()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", h.ServeWS)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/dev/ban", func(w http.ResponseWriter, r *http.Request) {
		wallet := r.URL.Query().Get("wallet")
		if wallet == "" {
			http.Error(w, "wallet required", http.StatusBadRequest)
			return
		}
		if err := st.SetDev(ctx, wallet, "scam", "banned"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "wallet": wallet, "kind": "scam"})
	})
	mux.HandleFunc("/dev/trust", func(w http.ResponseWriter, r *http.Request) {
		wallet := r.URL.Query().Get("wallet")
		if wallet == "" {
			http.Error(w, "wallet required", http.StatusBadRequest)
			return
		}
		if err := st.SetDev(ctx, wallet, "good", "trusted"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "wallet": wallet, "kind": "good"})
	})
	mux.HandleFunc("/dev/list", func(w http.ResponseWriter, r *http.Request) {
		list, err := st.ListDevs(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"devs": list})
	})
	mux.HandleFunc("/dev/load", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		kind := r.URL.Query().Get("kind")
		if kind != "good" && kind != "scam" {
			kind = "good"
		}
		if path == "" {
			http.Error(w, "path required", http.StatusBadRequest)
			return
		}
		n, err := st.LoadWalletsFile(ctx, path, kind)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "loaded": n, "kind": kind})
	})

	wsAddr := os.Getenv("WS_ADDR")
	if wsAddr == "" {
		wsAddr = ":9090"
	}
	go func() {
		log.Printf("Data Server на %s (ws=/ws, api=/dev/*)", wsAddr)
		if err := http.ListenAndServe(wsAddr, mux); err != nil {
			log.Fatalf("http: %v", err)
		}
	}()

	events := make(chan pump.Event, 256)
	pc := pump.NewClient()
	go func() {
		if err := pc.Run(ctx, events); err != nil {
			log.Printf("pump run: %v", err)
		}
	}()

	go func() {
		for ev := range events {
			switch ev.Type {
			case "new_token":
				var nt pump.NewToken
				if err := json.Unmarshal(ev.Raw, &nt); err != nil {
					continue
				}
				kind, _ := st.DevKind(ctx, nt.Creator)
				autoOpen := kind == "good"
				skip := kind == "scam"
				if skip {
					continue
				}
				h.Broadcast(map[string]any{
					"type":      "new_token",
					"source":    "pumpfun",
					"mint":      nt.Mint,
					"name":      nt.Name,
					"symbol":    nt.Symbol,
					"creator":   nt.Creator,
					"devBuySol": nt.SolAmount,
					"autoOpen":  autoOpen,
					"devKind":   kind,
					"ts":        nt.Timestamp,
				})
			case "migration":
				var mg pump.Migration
				if err := json.Unmarshal(ev.Raw, &mg); err != nil {
					continue
				}
				h.Broadcast(map[string]any{
					"type":    "migration",
					"source":  "pumpfun",
					"mint":    mg.Mint,
					"name":    mg.Name,
					"symbol":  mg.Symbol,
					"creator": mg.Creator,
					"ts":      mg.Timestamp,
				})
			}
		}
	}()

	log.Println("Data Server запущен. Ctrl+C для выхода.")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	cancel()
	time.Sleep(200 * time.Millisecond)
	log.Println("shutdown")
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
