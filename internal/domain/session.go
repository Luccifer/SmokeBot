package domain

import "time"

// SessionStatus represents the current status of a smoking session
type SessionStatus string

const (
	SessionStatusActive   SessionStatus = "active"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusCancelled SessionStatus = "cancelled"
)

// ResponseType represents how a user responded to a smoking invitation
type ResponseType string

const (
	ResponseAccepted       ResponseType = "accepted"
	ResponseAcceptedDelayed ResponseType = "accepted_delayed"
	ResponseDenied         ResponseType = "denied"
	ResponseRemote         ResponseType = "remote"
)

// Session represents a smoking session
type Session struct {
	ID          int64
	InitiatorID int64
	Status      SessionStatus
	CreatedAt   time.Time
	CompletedAt *time.Time
}

// SessionResponse represents a user's response to a session
type SessionResponse struct {
	ID         int64
	SessionID  int64
	UserID     int64
	Response   ResponseType
	CreatedAt  time.Time
}

// SessionRepository defines the interface for session storage
type SessionRepository interface {
	Create(session *Session) error
	GetByID(id int64) (*Session, error)
	GetActiveSession() (*Session, error)
	Update(session *Session) error
	CompleteSession(sessionID int64) error
	
	// Response methods
	AddResponse(response *SessionResponse) error
	GetResponses(sessionID int64) ([]*SessionResponse, error)
	GetUserResponse(sessionID int64, userID int64) (*SessionResponse, error)
	UpdateResponse(response *SessionResponse) error
}

