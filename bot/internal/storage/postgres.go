package storage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStorage struct {
	pool *pgxpool.Pool
}

func NewPostgresStorage(ctx context.Context, dsn string) (*PostgresStorage, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	return &PostgresStorage{pool: pool}, nil
}

func (s *PostgresStorage) Close() {
	s.pool.Close()
}

// GetOrCreateUser
func (s *PostgresStorage) GetOrCreateUser(ctx context.Context, user *User) (*User, error) {
	// пробуем найти
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`,
		user.ID,
	).Scan(&exists)
	if err != nil {
		return nil, err
	}

	if exists {
		// загружаем актуальные данные
		var u User
		err := s.pool.QueryRow(ctx,
			`SELECT id, COALESCE(username,''), COALESCE(first_name,''), COALESCE(last_name,''), state
             FROM users WHERE id = $1`,
			user.ID,
		).Scan(&u.ID, &u.Username, &u.FirstName, &u.LastName, (*string)(&u.State))
		if err != nil {
			return nil, err
		}
		return &u, nil
	}

	// создаём
	_, err = s.pool.Exec(ctx,
		`INSERT INTO users (id, username, first_name, last_name, state, created_at, updated_at)
         VALUES ($1, $2, $3, $4, $5, NOW(), NOW())`,
		user.ID, user.Username, user.FirstName, user.LastName, user.State,
	)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *PostgresStorage) SetState(ctx context.Context, userID int64, state UserState) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET state = $1, updated_at = NOW() WHERE id = $2`,
		state, userID,
	)
	return err
}

func (s *PostgresStorage) GetState(ctx context.Context, userID int64) (UserState, error) {
	var state string
	err := s.pool.QueryRow(ctx,
		`SELECT state FROM users WHERE id = $1`,
		userID,
	).Scan(&state)
	if err != nil {
		return StateIdle, nil // если нет юзера — считаем idle
	}
	return UserState(state), nil
}

func (s *PostgresStorage) CreateKey(ctx context.Context, key *AccessKey) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO access_keys (user_id, key, payment_method, is_used, created_at, expires_at)
         VALUES ($1, $2, $3, $4, $5, $6)`,
		key.UserID, key.Key, key.PaymentMethod, key.IsUsed, key.CreatedAt, key.ExpiresAt,
	)
	return err
}

func (s *PostgresStorage) GetUserKeys(ctx context.Context, userID int64) ([]*AccessKey, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, key, payment_method, is_used, created_at, expires_at
         FROM access_keys WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*AccessKey
	for rows.Next() {
		var k AccessKey
		var usedAt *time.Time
		err := rows.Scan(
			&k.ID, &k.UserID, &k.Key, &k.PaymentMethod, &k.IsUsed,
			&k.CreatedAt, &k.ExpiresAt, &usedAt,
		)
		if err != nil {
			return nil, err
		}
		k.UsedAt = usedAt
		keys = append(keys, &k)
	}
	return keys, rows.Err()
}
