package pump

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const WSURL = "wss://pumpportal.fun/api/data"

// Event — унифицированное событие от pumpportal.
type Event struct {
	Type string // "new_token" | "migration"
	Raw  json.RawMessage
}

// NewToken описывает создание монеты (subscribeNewToken).
type NewToken struct {
	TxType         string `json:"txType"`
	Mint           string `json:"mint"`
	Name           string `json:"name"`
	Symbol         string `json:"symbol"`
	Creator        string `json:"creator"`
	CreatorBalance string `json:"creatorBalance"`
	SolAmount      string `json:"solAmount"`
	MarketCapSol   string `json:"marketCapSol"`
	Pool           string `json:"pool"`
	Timestamp      int64  `json:"timestamp"`
}

// Migration описывает миграцию монеты (subscribeMigration).
type Migration struct {
	TxType    string `json:"txType"`
	Mint      string `json:"mint"`
	Name      string `json:"name"`
	Symbol    string `json:"symbol"`
	Creator   string `json:"creator"`
	Timestamp int64  `json:"timestamp"`
}

type Client struct {
	conn *websocket.Conn
	url  string
}

func NewClient() *Client {
	return &Client{url: WSURL}
}

// SubscribeNewToken подписывается на новые токены.
func (c *Client) SubscribeNewToken() error {
	return c.write(map[string]any{"method": "subscribeNewToken"})
}

// SubscribeMigration подписывается на миграции.
func (c *Client) SubscribeMigration() error {
	return c.write(map[string]any{"method": "subscribeMigration"})
}

func (c *Client) write(v any) error {
	return c.conn.WriteJSON(v)
}

// RunConn устанавливает соединение и читает события, отправляя их в out.
// При разрыве переподключается.
func (c *Client) Run(ctx context.Context, out chan<- Event) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.url, nil)
		if err != nil {
			log.Printf("pump: dial error: %v (retry 5s)", err)
			time.Sleep(5 * time.Second)
			continue
		}
		c.conn = conn
		log.Println("pump: connected")

		if err := c.SubscribeNewToken(); err != nil {
			log.Printf("pump: sub newToken: %v", err)
		}
		if err := c.SubscribeMigration(); err != nil {
			log.Printf("pump: sub migration: %v", err)
		}

		readErr := c.readLoop(ctx, out)
		conn.Close()
		if readErr != nil {
			log.Printf("pump: read error: %v (reconnect)", readErr)
		}
		time.Sleep(2 * time.Second)
	}
}

func (c *Client) readLoop(ctx context.Context, out chan<- Event) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return err
		}
		var probe struct {
			TxType string `json:"txType"`
		}
		if err := json.Unmarshal(data, &probe); err != nil {
			continue
		}
		ev := Event{Raw: data}
		switch probe.TxType {
		case "create":
			ev.Type = "new_token"
		case "migrate":
			ev.Type = "migration"
		default:
			continue
		}
		select {
		case out <- ev:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
