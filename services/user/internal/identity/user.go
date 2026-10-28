package identity

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
	StatusDeleted   Status = "deleted"
)

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusActive, StatusSuspended, StatusDeleted:
		return true
	default:
		return false
	}
}

type User struct {
	ID              uuid.UUID
	Email           string
	PasswordHash    string
	Status          Status
	DisplayName     string
	EmailVerifiedAt *time.Time
	LastLoginAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

type Role struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
