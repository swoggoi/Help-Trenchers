package store

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// DevKind возвращает тип дева: "good", "scam" или "" (неизвестен).
func (s *Store) DevKind(ctx context.Context, wallet string) (string, error) {
	var kind string
	err := s.pool.QueryRow(ctx,
		`SELECT kind FROM dev_lists WHERE wallet = $1`, wallet).
		Scan(&kind)
	if err != nil {
		return "", nil // неизвестен
	}
	return kind, nil
}

// SetDev задаёт тип дева (good/scam) с заметкой.
func (s *Store) SetDev(ctx context.Context, wallet, kind, note string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO dev_lists (wallet, kind, note) VALUES ($1, $2, $3)
         ON CONFLICT (wallet) DO UPDATE SET kind = EXCLUDED.kind, note = EXCLUDED.note`,
		wallet, kind, note)
	return err
}

// ListDevs возвращает всех девов с типом.
func (s *Store) ListDevs(ctx context.Context) ([]DevRow, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT wallet, kind, note, created_at FROM dev_lists ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DevRow
	for rows.Next() {
		var d DevRow
		if err := rows.Scan(&d.Wallet, &d.Kind, &d.Note, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type DevRow struct {
	Wallet    string    `json:"wallet"`
	Kind      string    `json:"kind"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// LoadWalletsFile загружает кошельки девов из файла (по одному на строку,
// либо JSON-массив строк). kind — 'good' или 'scam'.
func (s *Store) LoadWalletsFile(ctx context.Context, path, kind string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	// пробуем JSON-массив строк
	var asArray []string
	if err := json.Unmarshal(data, &asArray); err == nil && len(asArray) > 0 {
		n := 0
		for _, w := range asArray {
			if strings.TrimSpace(w) == "" {
				continue
			}
			if err := s.SetDev(ctx, strings.TrimSpace(w), kind, "file"); err == nil {
				n++
			}
		}
		return n, nil
	}
	// иначе построчно
	n := 0
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		w := strings.TrimSpace(sc.Text())
		if w == "" || strings.HasPrefix(w, "#") {
			continue
		}
		if err := s.SetDev(ctx, w, kind, "file"); err == nil {
			n++
		}
	}
	return n, sc.Err()
}

// ImportDevsTxt парсит devs.txt (массив {"h","t","a","f"}) и кладёт в twitter_accounts.
func (s *Store) ImportDevsTxt(ctx context.Context, path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var items []struct {
		H string `json:"h"`
		T bool   `json:"t"`
		A bool   `json:"a"`
		F bool   `json:"f"`
	}
	if err := json.Unmarshal(data, &items); err != nil {
		return 0, err
	}
	for _, it := range items {
		if it.H == "" {
			continue
		}
		_, err := s.pool.Exec(ctx,
			`INSERT INTO twitter_accounts (handle, flag_t, flag_a, flag_f) VALUES ($1, $2, $3, $4)
             ON CONFLICT (handle) DO UPDATE SET flag_t = EXCLUDED.flag_t, flag_a = EXCLUDED.flag_a, flag_f = EXCLUDED.flag_f`,
			it.H, it.T, it.A, it.F)
		if err != nil {
			return 0, err
		}
	}
	return len(items), nil
}
