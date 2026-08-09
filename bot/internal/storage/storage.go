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
	ID          int64
	Username    string
	FirstName   string
	LastName    string
	State       UserState
	PendingDays int
}

type AccessKey struct {
	ID            int64
	UserID        int64
	Key           string
	PaymentMethod string
	DurationDays  int
	IsUsed        bool
	CreatedAt     time.Time
	ExpiresAt     time.Time
	UsedAt        *time.Time
}

type Order struct {
	ID              int64
	UserID          int64
	PlanDays        int
	DepositAddress  string
	DepositPrivKey  string
	ExpectedLamports int64
	Status          string
	Signature       string
	KeyID           int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Storage interface {
	GetOrCreateUser(ctx context.Context, user *User) (*User, error)
	SetState(ctx context.Context, userID int64, state UserState) error
	GetState(ctx context.Context, userID int64) (UserState, error)
	SetPendingDays(ctx context.Context, userID int64, days int) error
	GetPendingDays(ctx context.Context, userID int64) (int, error)

	CreateKey(ctx context.Context, key *AccessKey) error
	GetUserKeys(ctx context.Context, userID int64) ([]*AccessKey, error)
	GetUserActiveKey(ctx context.Context, userID int64) (*AccessKey, error)

	CreateOrder(ctx context.Context, order *Order) error
	GetPendingOrders(ctx context.Context) ([]*Order, error)
	MarkOrderPaid(ctx context.Context, orderID int64, signature string, keyID int64) error
}
