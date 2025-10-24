package service

import (
	"fmt"
	"time"

	"github.com/glebk/smoke-bot/internal/domain"
)

// SmokeService handles business logic for smoking sessions
type SmokeService struct {
	userRepo    domain.UserRepository
	sessionRepo domain.SessionRepository
}

// NewSmokeService creates a new SmokeService
func NewSmokeService(userRepo domain.UserRepository, sessionRepo domain.SessionRepository) *SmokeService {
	service := &SmokeService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
	}

	// Clean up any old active sessions from previous runs
	service.CleanupOldSessions()

	return service
}

// CleanupOldSessions completes any active sessions older than 1 hour
func (s *SmokeService) CleanupOldSessions() {
	session, err := s.sessionRepo.GetActiveSession()
	if err != nil || session == nil {
		return
	}

	// If session is older than 1 hour, complete it
	if time.Since(session.CreatedAt) > time.Hour {
		_ = s.CompleteSession(session.ID)
	}
}

// AutoCompleteOldSessions automatically completes sessions older than 15 minutes
func (s *SmokeService) AutoCompleteOldSessions() (*domain.Session, error) {
	session, err := s.sessionRepo.GetActiveSession()
	if err != nil || session == nil {
		return nil, err
	}

	// If session is older than 15 minutes, complete it
	if time.Since(session.CreatedAt) > 15*time.Minute {
		if err := s.CompleteSession(session.ID); err != nil {
			return nil, err
		}
		return session, nil
	}

	return nil, nil
}

// RegisterUser registers a new user or updates existing one
func (s *SmokeService) RegisterUser(id int64, username, firstName, lastName string) error {
	existingUser, err := s.userRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("failed to check user: %w", err)
	}

	if existingUser != nil {
		// Update user info
		existingUser.Username = username
		existingUser.FirstName = firstName
		existingUser.LastName = lastName
		return s.userRepo.Update(existingUser)
	}

	// Create new user
	user := &domain.User{
		ID:        id,
		Username:  username,
		FirstName: firstName,
		LastName:  lastName,
	}

	return s.userRepo.Create(user)
}

// StartSession starts a new smoking session
func (s *SmokeService) StartSession(initiatorID int64) (*domain.Session, error) {
	// Check if there's already an active session
	activeSession, err := s.sessionRepo.GetActiveSession()
	if err != nil {
		return nil, fmt.Errorf("failed to check active session: %w", err)
	}

	if activeSession != nil {
		return nil, fmt.Errorf("there is already an active smoking session")
	}

	// Create new session
	session := &domain.Session{
		InitiatorID: initiatorID,
		Status:      domain.SessionStatusActive,
	}

	if err := s.sessionRepo.Create(session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// RespondToSession records a user's response to a session
func (s *SmokeService) RespondToSession(sessionID int64, userID int64, responseType domain.ResponseType) error {
	// Verify session exists and is active
	session, err := s.sessionRepo.GetByID(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	if session == nil {
		return fmt.Errorf("session not found")
	}

	if session.Status != domain.SessionStatusActive {
		return fmt.Errorf("session is not active")
	}

	// Handle "I am remote" response
	if responseType == domain.ResponseRemote {
		if err := s.SetRemoteStatus(userID); err != nil {
			return fmt.Errorf("failed to set remote status: %w", err)
		}
	}

	// Add or update response
	response := &domain.SessionResponse{
		SessionID: sessionID,
		UserID:    userID,
		Response:  responseType,
	}

	return s.sessionRepo.AddResponse(response)
}

// GetSessionSummary returns a formatted summary of session responses
func (s *SmokeService) GetSessionSummary(sessionID int64) (string, error) {
	responses, err := s.sessionRepo.GetResponses(sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to get responses: %w", err)
	}

	var accepted []string
	var acceptedDelayed []string
	var denied []string

	for _, resp := range responses {
		user, err := s.userRepo.GetByID(resp.UserID)
		if err != nil {
			continue
		}

		// Skip hidden users - they should be invisible everywhere
		if user.IsHidden {
			continue
		}

		displayName := user.Username
		if displayName == "" {
			displayName = user.FirstName
		}

		switch resp.Response {
		case domain.ResponseAccepted:
			accepted = append(accepted, displayName)
		case domain.ResponseAcceptedDelayed:
			acceptedDelayed = append(acceptedDelayed, displayName)
		case domain.ResponseDenied:
			denied = append(denied, displayName)
		}
	}

	summary := "📊 *Статус перекура:*\n\n"

	if len(accepted) > 0 {
		summary += "✅ *Идут сейчас:*\n"
		for _, name := range accepted {
			summary += fmt.Sprintf("  • @%s\n", name)
		}
		summary += "\n"
	}

	if len(acceptedDelayed) > 0 {
		summary += "⏱ *Придут в течение 5 минут:*\n"
		for _, name := range acceptedDelayed {
			summary += fmt.Sprintf("  • @%s\n", name)
		}
		summary += "\n"
	}

	if len(denied) > 0 {
		summary += "❌ *Не идут:*\n"
		for _, name := range denied {
			summary += fmt.Sprintf("  • @%s\n", name)
		}
	}

	if len(accepted) == 0 && len(acceptedDelayed) == 0 && len(denied) == 0 {
		summary = "Пока никто не ответил"
	}

	return summary, nil
}

// GetActiveUsers returns all users who are not in remote status
func (s *SmokeService) GetActiveUsers(excludeUserID int64) ([]*domain.User, error) {
	// Clear expired remote statuses first
	if err := s.userRepo.ClearExpiredRemoteStatus(); err != nil {
		return nil, fmt.Errorf("failed to clear expired remote status: %w", err)
	}

	allUsers, err := s.userRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	var activeUsers []*domain.User
	for _, user := range allUsers {
		// Exclude the initiator, remote users, and hidden users
		if user.ID != excludeUserID && !user.IsRemoteToday && !user.IsHidden {
			activeUsers = append(activeUsers, user)
		}
	}

	return activeUsers, nil
}

// SetRemoteStatus sets a user as remote until end of day (23:59)
func (s *SmokeService) SetRemoteStatus(userID int64) error {
	now := time.Now()
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())

	return s.userRepo.SetRemoteStatus(userID, endOfDay)
}

// ClearRemoteStatus removes remote status for a user
func (s *SmokeService) ClearRemoteStatus(userID int64) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return fmt.Errorf("user not found")
	}

	user.IsRemoteToday = false
	user.RemoteUntil = nil

	return s.userRepo.Update(user)
}

// CompleteSession marks a session as completed
func (s *SmokeService) CompleteSession(sessionID int64) error {
	return s.sessionRepo.CompleteSession(sessionID)
}

// GetActiveSession returns the current active session if exists
func (s *SmokeService) GetActiveSession() (*domain.Session, error) {
	return s.sessionRepo.GetActiveSession()
}

// GetUser returns a user by ID
func (s *SmokeService) GetUser(userID int64) (*domain.User, error) {
	return s.userRepo.GetByID(userID)
}

// CancelSession cancels an active session
func (s *SmokeService) CancelSession(sessionID int64) error {
	session, err := s.sessionRepo.GetByID(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	if session == nil {
		return fmt.Errorf("session not found")
	}

	session.Status = domain.SessionStatusCancelled
	now := time.Now()
	session.CompletedAt = &now

	return s.sessionRepo.Update(session)
}

// GetSessionRespondents returns all users who responded to a session
func (s *SmokeService) GetSessionRespondents(sessionID int64) ([]*domain.User, error) {
	responses, err := s.sessionRepo.GetResponses(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get responses: %w", err)
	}

	var users []*domain.User
	userMap := make(map[int64]bool) // To avoid duplicates

	for _, resp := range responses {
		// Only include users who accepted (not denied or remote)
		if resp.Response == domain.ResponseAccepted || resp.Response == domain.ResponseAcceptedDelayed {
			if !userMap[resp.UserID] {
				user, err := s.userRepo.GetByID(resp.UserID)
				if err != nil {
					continue
				}
				users = append(users, user)
				userMap[resp.UserID] = true
			}
		}
	}

	return users, nil
}

// GetSessionResponses returns all responses for a session
func (s *SmokeService) GetSessionResponses(sessionID int64) ([]*domain.SessionResponse, error) {
	return s.sessionRepo.GetResponses(sessionID)
}
