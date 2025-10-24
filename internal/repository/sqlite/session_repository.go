package sqlite

import (
	"database/sql"
	"fmt"
	"time"
	
	"github.com/glebk/smoke-bot/internal/domain"
)

// SessionRepository implements domain.SessionRepository using SQLite
type SessionRepository struct {
	db *Database
}

// NewSessionRepository creates a new SessionRepository
func NewSessionRepository(db *Database) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create creates a new session
func (r *SessionRepository) Create(session *domain.Session) error {
	query := `
		INSERT INTO sessions (initiator_id, status, created_at)
		VALUES (?, ?, ?)
	`
	
	now := time.Now()
	result, err := r.db.GetDB().Exec(query,
		session.InitiatorID,
		session.Status,
		now,
	)
	
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get session ID: %w", err)
	}
	
	session.ID = id
	session.CreatedAt = now
	
	return nil
}

// GetByID retrieves a session by ID
func (r *SessionRepository) GetByID(id int64) (*domain.Session, error) {
	query := `
		SELECT id, initiator_id, status, created_at, completed_at
		FROM sessions
		WHERE id = ?
	`
	
	session := &domain.Session{}
	var completedAt sql.NullTime
	
	err := r.db.GetDB().QueryRow(query, id).Scan(
		&session.ID,
		&session.InitiatorID,
		&session.Status,
		&session.CreatedAt,
		&completedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	
	if completedAt.Valid {
		session.CompletedAt = &completedAt.Time
	}
	
	return session, nil
}

// GetActiveSession retrieves the current active session
func (r *SessionRepository) GetActiveSession() (*domain.Session, error) {
	query := `
		SELECT id, initiator_id, status, created_at, completed_at
		FROM sessions
		WHERE status = ?
		ORDER BY created_at DESC
		LIMIT 1
	`
	
	session := &domain.Session{}
	var completedAt sql.NullTime
	
	err := r.db.GetDB().QueryRow(query, domain.SessionStatusActive).Scan(
		&session.ID,
		&session.InitiatorID,
		&session.Status,
		&session.CreatedAt,
		&completedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active session: %w", err)
	}
	
	if completedAt.Valid {
		session.CompletedAt = &completedAt.Time
	}
	
	return session, nil
}

// Update updates a session
func (r *SessionRepository) Update(session *domain.Session) error {
	query := `
		UPDATE sessions
		SET status = ?, completed_at = ?
		WHERE id = ?
	`
	
	_, err := r.db.GetDB().Exec(query,
		session.Status,
		session.CompletedAt,
		session.ID,
	)
	
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	
	return nil
}

// CompleteSession marks a session as completed
func (r *SessionRepository) CompleteSession(sessionID int64) error {
	query := `
		UPDATE sessions
		SET status = ?, completed_at = ?
		WHERE id = ?
	`
	
	now := time.Now()
	_, err := r.db.GetDB().Exec(query,
		domain.SessionStatusCompleted,
		now,
		sessionID,
	)
	
	if err != nil {
		return fmt.Errorf("failed to complete session: %w", err)
	}
	
	return nil
}

// AddResponse adds a user response to a session
func (r *SessionRepository) AddResponse(response *domain.SessionResponse) error {
	query := `
		INSERT INTO session_responses (session_id, user_id, response, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(session_id, user_id) DO UPDATE SET response = ?, created_at = ?
	`
	
	now := time.Now()
	result, err := r.db.GetDB().Exec(query,
		response.SessionID,
		response.UserID,
		response.Response,
		now,
		response.Response,
		now,
	)
	
	if err != nil {
		return fmt.Errorf("failed to add response: %w", err)
	}
	
	if response.ID == 0 {
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get response ID: %w", err)
		}
		response.ID = id
	}
	
	response.CreatedAt = now
	
	return nil
}

// GetResponses retrieves all responses for a session
func (r *SessionRepository) GetResponses(sessionID int64) ([]*domain.SessionResponse, error) {
	query := `
		SELECT id, session_id, user_id, response, created_at
		FROM session_responses
		WHERE session_id = ?
		ORDER BY created_at
	`
	
	rows, err := r.db.GetDB().Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get responses: %w", err)
	}
	defer rows.Close()
	
	var responses []*domain.SessionResponse
	
	for rows.Next() {
		response := &domain.SessionResponse{}
		
		err := rows.Scan(
			&response.ID,
			&response.SessionID,
			&response.UserID,
			&response.Response,
			&response.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan response: %w", err)
		}
		
		responses = append(responses, response)
	}
	
	return responses, nil
}

// GetUserResponse retrieves a specific user's response to a session
func (r *SessionRepository) GetUserResponse(sessionID int64, userID int64) (*domain.SessionResponse, error) {
	query := `
		SELECT id, session_id, user_id, response, created_at
		FROM session_responses
		WHERE session_id = ? AND user_id = ?
	`
	
	response := &domain.SessionResponse{}
	
	err := r.db.GetDB().QueryRow(query, sessionID, userID).Scan(
		&response.ID,
		&response.SessionID,
		&response.UserID,
		&response.Response,
		&response.CreatedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user response: %w", err)
	}
	
	return response, nil
}

// UpdateResponse updates a user's response
func (r *SessionRepository) UpdateResponse(response *domain.SessionResponse) error {
	query := `
		UPDATE session_responses
		SET response = ?, created_at = ?
		WHERE id = ?
	`
	
	now := time.Now()
	_, err := r.db.GetDB().Exec(query,
		response.Response,
		now,
		response.ID,
	)
	
	if err != nil {
		return fmt.Errorf("failed to update response: %w", err)
	}
	
	response.CreatedAt = now
	
	return nil
}

