package storage

import (
	"context"
	"time"
)

type UserState string

const (
	StateIdle           UserState = "idle"
	StateChoosingPay    UserState = "choosing_pay"
	StateWaitingConfirm UserState = "waiting_confirm"
)

type User struct {
	ID        int64
	Username  string
	FirstName string
	LastName  string
	State     UserState
}

type AccessKey struct {
	ID            int64
	UserID        int64
	Key           string
	PaymentMethod string // stars и прочая currency
	IsUsed        bool
	CreatedAt     time.Time
	ExpiresAt     time.Time
	UsedAt        *time.Time
}

type Storage interface {
	// users
	GetOrCreateUser(ctx context.Context, user *User) (*User, error)
	SetState(ctx context.Context, userID int64, state UserState) error
	GetState(ctx context.Context, userID int64) (UserState, error)

	// keys
	CreateKey(ctx context.Context, key *AccessKey) error
	GetUserKeys(ctx context.Context, userID int64) ([]*AccessKey, error)
}
