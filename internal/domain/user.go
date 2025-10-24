package domain

import "time"

// User represents a bot user
type User struct {
	ID            int64
	Username      string
	FirstName     string
	LastName      string
	IsRemoteToday bool
	RemoteUntil   *time.Time
	IsHidden      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// UserRepository defines the interface for user storage
type UserRepository interface {
	Create(user *User) error
	GetByID(id int64) (*User, error)
	GetAll() ([]*User, error)
	Update(user *User) error
	Delete(id int64) error
	SetRemoteStatus(userID int64, until time.Time) error
	ClearExpiredRemoteStatus() error
}
