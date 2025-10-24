package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/glebk/smoke-bot/internal/domain"
)

// UserRepository implements domain.UserRepository using SQLite
type UserRepository struct {
	db *Database
}

// NewUserRepository creates a new UserRepository
func NewUserRepository(db *Database) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user
func (r *UserRepository) Create(user *domain.User) error {
	query := `
		INSERT INTO users (id, username, first_name, last_name, is_remote_today, remote_until, is_hidden, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()

	// Auto-hide user "eyerise"
	if user.Username == "eyerise" {
		user.IsHidden = true
	}

	_, err := r.db.GetDB().Exec(query,
		user.ID,
		user.Username,
		user.FirstName,
		user.LastName,
		boolToInt(user.IsRemoteToday),
		user.RemoteUntil,
		boolToInt(user.IsHidden),
		now,
		now,
	)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	user.CreatedAt = now
	user.UpdatedAt = now

	return nil
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(id int64) (*domain.User, error) {
	query := `
		SELECT id, username, first_name, last_name, is_remote_today, remote_until, is_hidden, created_at, updated_at
		FROM users
		WHERE id = ?
	`

	user := &domain.User{}
	var isRemote int
	var isHidden int
	var remoteUntil sql.NullTime
	var lastName sql.NullString

	err := r.db.GetDB().QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.FirstName,
		&lastName,
		&isRemote,
		&remoteUntil,
		&isHidden,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.IsRemoteToday = intToBool(isRemote)
	user.IsHidden = intToBool(isHidden)
	if remoteUntil.Valid {
		user.RemoteUntil = &remoteUntil.Time
	}
	if lastName.Valid {
		user.LastName = lastName.String
	}

	return user, nil
}

// GetAll retrieves all users
func (r *UserRepository) GetAll() ([]*domain.User, error) {
	query := `
		SELECT id, username, first_name, last_name, is_remote_today, remote_until, is_hidden, created_at, updated_at
		FROM users
		ORDER BY username
	`

	rows, err := r.db.GetDB().Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all users: %w", err)
	}
	defer rows.Close()

	var users []*domain.User

	for rows.Next() {
		user := &domain.User{}
		var isRemote int
		var isHidden int
		var remoteUntil sql.NullTime
		var lastName sql.NullString

		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.FirstName,
			&lastName,
			&isRemote,
			&remoteUntil,
			&isHidden,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		user.IsRemoteToday = intToBool(isRemote)
		user.IsHidden = intToBool(isHidden)
		if remoteUntil.Valid {
			user.RemoteUntil = &remoteUntil.Time
		}
		if lastName.Valid {
			user.LastName = lastName.String
		}

		users = append(users, user)
	}

	return users, nil
}

// Update updates a user
func (r *UserRepository) Update(user *domain.User) error {
	query := `
		UPDATE users
		SET username = ?, first_name = ?, last_name = ?, is_remote_today = ?, remote_until = ?, is_hidden = ?, updated_at = ?
		WHERE id = ?
	`

	// Auto-hide user "eyerise"
	if user.Username == "eyerise" {
		user.IsHidden = true
	}

	now := time.Now()
	_, err := r.db.GetDB().Exec(query,
		user.Username,
		user.FirstName,
		user.LastName,
		boolToInt(user.IsRemoteToday),
		user.RemoteUntil,
		boolToInt(user.IsHidden),
		now,
		user.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	user.UpdatedAt = now

	return nil
}

// Delete deletes a user
func (r *UserRepository) Delete(id int64) error {
	query := `DELETE FROM users WHERE id = ?`

	_, err := r.db.GetDB().Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// SetRemoteStatus sets the remote status for a user
func (r *UserRepository) SetRemoteStatus(userID int64, until time.Time) error {
	query := `
		UPDATE users
		SET is_remote_today = 1, remote_until = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := r.db.GetDB().Exec(query, until, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to set remote status: %w", err)
	}

	return nil
}

// ClearExpiredRemoteStatus clears remote status for users where the time has expired
func (r *UserRepository) ClearExpiredRemoteStatus() error {
	query := `
		UPDATE users
		SET is_remote_today = 0, remote_until = NULL, updated_at = ?
		WHERE is_remote_today = 1 AND remote_until < ?
	`

	now := time.Now()
	_, err := r.db.GetDB().Exec(query, now, now)
	if err != nil {
		return fmt.Errorf("failed to clear expired remote status: %w", err)
	}

	return nil
}

// Helper functions
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i != 0
}
