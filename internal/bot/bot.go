package bot

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/glebk/smoke-bot/internal/config"
	"github.com/glebk/smoke-bot/internal/domain"
	"github.com/glebk/smoke-bot/internal/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot represents the Telegram bot
type Bot struct {
	api     *tgbotapi.BotAPI
	service *service.SmokeService
	config  *config.Config
}

// New creates a new Bot instance
func New(token string, service *service.SmokeService, cfg *config.Config) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	log.Printf("Authorized on account %s", api.Self.UserName)

	return &Bot{
		api:     api,
		service: service,
		config:  cfg,
	}, nil
}

// Start starts the bot
func (b *Bot) Start() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	// Start background routine to auto-complete old sessions
	go b.autoCompleteSessionsRoutine()

	for update := range updates {
		if update.Message != nil {
			b.handleMessage(update.Message)
		} else if update.CallbackQuery != nil {
			b.handleCallbackQuery(update.CallbackQuery)
		}
	}

	return nil
}

// autoCompleteSessionsRoutine runs in background and auto-completes sessions after 15 minutes
func (b *Bot) autoCompleteSessionsRoutine() {
	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	defer ticker.Stop()

	for range ticker.C {
		completedSession, err := b.service.AutoCompleteOldSessions()
		if err != nil {
			log.Printf("Error auto-completing sessions: %v", err)
			continue
		}

		if completedSession != nil {
			// Session was auto-completed, notify participants
			b.notifySessionCompleted(completedSession)
		}
	}
}

// notifySessionCompleted notifies all participants that the session has ended
func (b *Bot) notifySessionCompleted(session *domain.Session) {
	// Get all responses to notify everyone who participated
	responses, err := b.service.GetSessionResponses(session.ID)
	if err != nil {
		log.Printf("Error getting session responses: %v", err)
		return
	}

	// Build final summary with past tense
	var attended []string
	var attendedDelayed []string

	for _, resp := range responses {
		user, err := b.service.GetUser(resp.UserID)
		if err != nil {
			continue
		}

		// Skip hidden users
		if user.IsHidden {
			continue
		}

		displayName := user.Username
		if displayName == "" {
			displayName = user.FirstName
		}

		switch resp.Response {
		case domain.ResponseAccepted:
			attended = append(attended, displayName)
		case domain.ResponseAcceptedDelayed:
			attendedDelayed = append(attendedDelayed, displayName)
		}
	}

	summary := "📊 *Итоги перекура:*\n\n"

	if len(attended) > 0 {
		summary += "✅ *Были на перекуре:*\n"
		for _, name := range attended {
			summary += fmt.Sprintf("  • @%s\n", name)
		}
		summary += "\n"
	}

	if len(attendedDelayed) > 0 {
		summary += "⏱ *Пришли позже:*\n"
		for _, name := range attendedDelayed {
			summary += fmt.Sprintf("  • @%s\n", name)
		}
		summary += "\n"
	}

	if len(attended) == 0 && len(attendedDelayed) == 0 {
		summary = "Никто не пришёл на перекур 😔"
	}

	completionMsg := fmt.Sprintf("⏰ *Перекур завершён (15 минут прошло)*\n\n%s", summary)

	// Notify the initiator
	initiator, _ := b.service.GetUser(session.InitiatorID)
	if initiator == nil || !initiator.IsHidden {
		msg := tgbotapi.NewMessage(session.InitiatorID, completionMsg)
		msg.ParseMode = "Markdown"
		if _, err := b.api.Send(msg); err != nil {
			log.Printf("Error notifying initiator: %v", err)
		}
	}

	// Notify all users who accepted
	notifiedUsers := make(map[int64]bool)
	notifiedUsers[session.InitiatorID] = true

	for _, resp := range responses {
		// Only notify users who accepted
		if resp.Response == domain.ResponseAccepted || resp.Response == domain.ResponseAcceptedDelayed {
			if !notifiedUsers[resp.UserID] {
				user, _ := b.service.GetUser(resp.UserID)
				if user == nil || !user.IsHidden {
					msg := tgbotapi.NewMessage(resp.UserID, completionMsg)
					msg.ParseMode = "Markdown"
					if _, err := b.api.Send(msg); err != nil {
						log.Printf("Error notifying user %d: %v", resp.UserID, err)
					}
				}
				notifiedUsers[resp.UserID] = true
			}
		}
	}
}

// handleMessage handles incoming messages
func (b *Bot) handleMessage(message *tgbotapi.Message) {
	// Register or update user
	b.registerUser(message.From)

	// Check if command
	if message.IsCommand() {
		b.handleCommand(message)
		return
	}

	// Handle keyboard button
	if message.Text == "🚬 Го курить!" {
		b.handleSmoke(message)
		return
	}
}

// handleCommand handles bot commands
func (b *Bot) handleCommand(message *tgbotapi.Message) {
	switch message.Command() {
	case "start":
		b.handleStart(message)
	case "smoke":
		b.handleSmoke(message)
	case "status":
		b.handleStatus(message)
	case "cancel":
		b.handleCancel(message)
	case "office":
		b.handleBackToOffice(message)
	case "help":
		b.handleHelp(message)
	default:
		b.sendMessage(message.Chat.ID, "Неизвестная команда. Используйте /help чтобы узнать больше")
	}
}

// handleStart handles the /start command
func (b *Bot) handleStart(message *tgbotapi.Message) {
	text := fmt.Sprintf(
		"👋 Добро пожаловать в бот курильщика, %s!\n\n"+
			"Этот бот поможет скоординироваться с коллегами для перекура.\n\n"+
			"Используйте /smoke или нажмите на кнопку ниже, чтобы пригласить других\n"+
			"Используйте /status чтобы увидеть текущий статус перекура\n"+
			"Используйте /help для показа информации",
		message.From.FirstName,
	)

	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🚬 Го курить!"),
		),
	)

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ReplyMarkup = keyboard
	msg.ParseMode = "Markdown"

	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending start message: %v", err)
	}
}

// handleSmoke handles the smoke break initiation
func (b *Bot) handleSmoke(message *tgbotapi.Message) {
	// Check working hours
	if !b.config.IsWorkingHours() {
		b.sendMessage(message.Chat.ID,
			"⏰ К сожалению, сейчас не время перекуров. Повторить можно в рабочее время (09:00 - 23:00).")
		return
	}

	// Start new session
	session, err := b.service.StartSession(message.From.ID)
	if err != nil {
		if strings.Contains(err.Error(), "already an active") {
			b.sendMessage(message.Chat.ID,
				"⚠️ Сейчас уже идет активный перекур! Используйте /status чтобы узнать больше")
		} else {
			b.sendMessage(message.Chat.ID,
				"❌ Не вышло организовать перекур. Попробуйте позже")
			log.Printf("Error starting session: %v", err)
		}
		return
	}

	// Get initiator info
	initiator, err := b.service.GetUser(message.From.ID)
	if err != nil {
		log.Printf("Error getting initiator: %v", err)
		return
	}

	initiatorName := initiator.Username
	if initiatorName == "" {
		initiatorName = initiator.FirstName
	}

	// Notify all active users
	activeUsers, err := b.service.GetActiveUsers(message.From.ID)
	if err != nil {
		log.Printf("Error getting active users: %v", err)
		return
	}

	if len(activeUsers) == 0 {
		// Cancel the session since no one to notify
		b.service.CancelSession(session.ID)
		b.sendMessage(message.Chat.ID,
			"😔 Активных курильщиков в боте нет. Наслаждайтесь своим уединением!")
		return
	}

	// Send confirmation to initiator with cancel button
	cancelButton := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Отменить перекур", fmt.Sprintf("cancel:%d", session.ID)),
		),
	)

	msg := tgbotapi.NewMessage(message.Chat.ID,
		fmt.Sprintf("✅ Перекур начался! Уведомления направлены %d коллегам...\n\nИспользуйте /cancel или кнопку ниже для отмены.", len(activeUsers)))
	msg.ReplyMarkup = cancelButton

	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending confirmation: %v", err)
	}

	// Send invitation to all active users
	for _, user := range activeUsers {
		b.sendInvitation(user.ID, session.ID, initiatorName)
	}
}

// handleStatus shows the current session status
func (b *Bot) handleStatus(message *tgbotapi.Message) {
	session, err := b.service.GetActiveSession()
	if err != nil {
		log.Printf("Error getting active session: %v", err)
		b.sendMessage(message.Chat.ID, "❌ Ошибка при проверке статуса перекура")
		return
	}

	if session == nil {
		b.sendMessage(message.Chat.ID, "📭 Сейчас перекура нет")
		return
	}

	summary, err := b.service.GetSessionSummary(session.ID)
	if err != nil {
		log.Printf("Error getting session summary: %v", err)
		b.sendMessage(message.Chat.ID, "❌ Что-то пошло не так в этом перекуре")
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, summary)
	msg.ParseMode = "Markdown"

	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending status: %v", err)
	}
}

// handleCancel handles canceling an active session
func (b *Bot) handleCancel(message *tgbotapi.Message) {
	session, err := b.service.GetActiveSession()
	if err != nil {
		log.Printf("Error getting active session: %v", err)
		b.sendMessage(message.Chat.ID, "❌ Ошибка при проверке статуса перекура")
		return
	}

	if session == nil {
		b.sendMessage(message.Chat.ID, "📭 Нет активного перекура для отмены")
		return
	}

	// Check if user is the initiator
	if session.InitiatorID != message.From.ID {
		b.sendMessage(message.Chat.ID, "⛔️ Только инициатор перекура может его отменить")
		return
	}

	// Get all users who responded
	respondedUsers, err := b.service.GetSessionRespondents(session.ID)
	if err != nil {
		log.Printf("Error getting respondents: %v", err)
	}

	// Cancel the session
	if err := b.service.CancelSession(session.ID); err != nil {
		log.Printf("Error canceling session: %v", err)
		b.sendMessage(message.Chat.ID, "❌ Не удалось отменить перекур")
		return
	}

	b.sendMessage(message.Chat.ID, "✅ Перекур отменён!")

	// Notify all users who responded
	for _, user := range respondedUsers {
		if user.ID != message.From.ID {
			b.sendMessage(user.ID, "❌ Перекур был отменён инициатором")
		}
	}
}

// handleBackToOffice removes remote status
func (b *Bot) handleBackToOffice(message *tgbotapi.Message) {
	user, err := b.service.GetUser(message.From.ID)
	if err != nil {
		log.Printf("Error getting user: %v", err)
		b.sendMessage(message.Chat.ID, "❌ Ошибка получения статуса")
		return
	}

	if user == nil {
		b.sendMessage(message.Chat.ID, "⚠️ Сначала используйте /start")
		return
	}

	if !user.IsRemoteToday {
		b.sendMessage(message.Chat.ID, "✅ Вы и так не на удаленке. Можете получать уведомления!")
		return
	}

	if err := b.service.ClearRemoteStatus(message.From.ID); err != nil {
		log.Printf("Error clearing remote status: %v", err)
		b.sendMessage(message.Chat.ID, "❌ Не удалось сбросить статус")
		return
	}

	b.sendMessage(message.Chat.ID, "🏢 Отлично! Вы вернулись в офис. Теперь будете получать уведомления о перекурах!")
}

// handleHelp shows help information
func (b *Bot) handleHelp(message *tgbotapi.Message) {
	text := `*Бот для курильщиков - Помощь*

*Команды:*
/start - Активировать бота и показать меню
/smoke - Пригласить коллег на перекур
/status - Проверить текущий статус перекура
/cancel - Отменить текущий перекур (только для инициатора)
/office - Вернуться в офис (отменить статус "на удаленке")
/help - Показать помощь

*Как это работает:*
1. Нажмите "🚬 Го курить!" или используйте /smoke
2. Все коллеги получат уведомление
3. Они могут ответить:
   • ✅ Го курить! - Присоединиться сразу
   • ⏱ В течение 5 минут - Присоединиться с задержкой
   • ❌ Не, спс - Отклонить приглашение
   • 🏠 Я на удаленке (больше уведомлений не будет до завтра)

*Рабочие часы:*
Бот обрабатывает запросы только в рабочее время (09:00 - 23:00).

Наслаждайтесь перекурами! 🚬☕`

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ParseMode = "Markdown"

	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending help: %v", err)
	}
}

// sendInvitation sends a smoking invitation to a user
func (b *Bot) sendInvitation(userID int64, sessionID int64, initiatorName string) {
	text := fmt.Sprintf("🚬 @%s приглашает вас на перекур!\n\nГо курить?", initiatorName)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Го курить!", fmt.Sprintf("accept:%d", sessionID)),
			tgbotapi.NewInlineKeyboardButtonData("⏱ В течение 5 минут", fmt.Sprintf("delayed:%d", sessionID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Не, спс", fmt.Sprintf("deny:%d", sessionID)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Я на удаленке", fmt.Sprintf("remote:%d", sessionID)),
		),
	)

	msg := tgbotapi.NewMessage(userID, text)
	msg.ReplyMarkup = keyboard

	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending invitation to user %d: %v", userID, err)
	}
}

// handleCallbackQuery handles button callbacks
func (b *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	// Parse callback data
	parts := strings.Split(query.Data, ":")
	if len(parts) != 2 {
		b.answerCallback(query.ID, "Invalid response")
		return
	}

	action := parts[0]
	sessionID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		b.answerCallback(query.ID, "Invalid session ID")
		return
	}

	// Register user if not already
	b.registerUser(query.From)

	// Handle cancel action
	if action == "cancel" {
		session, err := b.service.GetActiveSession()
		if err != nil || session == nil || session.ID != sessionID {
			b.answerCallback(query.ID, "❌ Перекур уже не активен")
			return
		}

		if session.InitiatorID != query.From.ID {
			b.answerCallback(query.ID, "⛔️ Только инициатор может отменить")
			return
		}

		// Get all users who responded
		respondedUsers, err := b.service.GetSessionRespondents(sessionID)
		if err != nil {
			log.Printf("Error getting respondents: %v", err)
		}

		// Cancel the session
		if err := b.service.CancelSession(sessionID); err != nil {
			log.Printf("Error canceling session: %v", err)
			b.answerCallback(query.ID, "❌ Не удалось отменить")
			return
		}

		b.answerCallback(query.ID, "✅ Перекур отменён!")

		// Update initiator's message
		editMsg := tgbotapi.NewEditMessageText(
			query.Message.Chat.ID,
			query.Message.MessageID,
			query.Message.Text+"\n\n❌ *Перекур отменён*",
		)
		editMsg.ParseMode = "Markdown"
		if _, err := b.api.Send(editMsg); err != nil {
			log.Printf("Error editing message: %v", err)
		}

		// Notify all users who responded
		for _, user := range respondedUsers {
			if user.ID != query.From.ID {
				b.sendMessage(user.ID, "❌ Перекур был отменён инициатором")
			}
		}
		return
	}

	// Verify session is still active
	session, err := b.service.GetActiveSession()
	if err != nil || session == nil || session.ID != sessionID {
		b.answerCallback(query.ID, "❌ Этот перекур уже не активен")

		// Update message to show it's cancelled
		editMsg := tgbotapi.NewEditMessageText(
			query.Message.Chat.ID,
			query.Message.MessageID,
			query.Message.Text+"\n\n❌ *Перекур отменён*",
		)
		editMsg.ParseMode = "Markdown"
		if _, err := b.api.Send(editMsg); err != nil {
			log.Printf("Error editing message: %v", err)
		}
		return
	}

	// Map action to response type
	var responseType domain.ResponseType
	var responseText string

	switch action {
	case "accept":
		responseType = domain.ResponseAccepted
		responseText = "✅ Отлично! Увидимся в курилке!"
	case "delayed":
		responseType = domain.ResponseAcceptedDelayed
		responseText = "⏱ Ясненько! Увидимся в течение 5 минут!"
	case "deny":
		responseType = domain.ResponseDenied
		responseText = "👌 Пон! В следующий раз тогда."
	case "remote":
		responseType = domain.ResponseRemote
		responseText = "🏠 Удаленно сегодня. Никаких уведомлений до завтра.\n\nИспользуйте /office чтобы вернуться в офис."
	default:
		b.answerCallback(query.ID, "Неизвестное действие")
		return
	}

	// Get user info for notification
	respondent, err := b.service.GetUser(query.From.ID)
	if err != nil {
		log.Printf("Error getting respondent: %v", err)
	}

	respondentName := query.From.FirstName
	if respondent != nil && respondent.Username != "" {
		respondentName = "@" + respondent.Username
	}

	// Record response
	if err := b.service.RespondToSession(sessionID, query.From.ID, responseType); err != nil {
		log.Printf("Error recording response: %v", err)
		b.answerCallback(query.ID, "❌ Ошибка записи ответа")
		return
	}

	// Answer callback
	b.answerCallback(query.ID, responseText)

	// Update message to show response
	editMsg := tgbotapi.NewEditMessageText(
		query.Message.Chat.ID,
		query.Message.MessageID,
		query.Message.Text+"\n\n"+responseText,
	)

	if _, err := b.api.Send(editMsg); err != nil {
		log.Printf("Error editing message: %v", err)
	}

	// Send notifications based on response type
	b.notifyParticipants(session, query.From.ID, respondentName, responseType)
}

// registerUser registers or updates a user
func (b *Bot) registerUser(user *tgbotapi.User) {
	username := user.UserName
	if username == "" {
		username = fmt.Sprintf("user%d", user.ID)
	}

	lastName := user.LastName

	if err := b.service.RegisterUser(user.ID, username, user.FirstName, lastName); err != nil {
		log.Printf("Error registering user %d: %v", user.ID, err)
	}
}

// sendMessage sends a simple text message
func (b *Bot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

// answerCallback answers a callback query
func (b *Bot) answerCallback(callbackID string, text string) {
	callback := tgbotapi.NewCallback(callbackID, text)
	if _, err := b.api.Request(callback); err != nil {
		log.Printf("Error answering callback: %v", err)
	}
}

// notifyParticipants notifies relevant users about a response
func (b *Bot) notifyParticipants(session *domain.Session, responderID int64, responderName string, responseType domain.ResponseType) {
	// Check if responder is hidden
	responder, err := b.service.GetUser(responderID)
	if err != nil {
		log.Printf("Error getting responder: %v", err)
		return
	}

	// Don't notify about hidden users
	if responder != nil && responder.IsHidden {
		return
	}

	// Get all responses for this session
	responses, err := b.service.GetSessionResponses(session.ID)
	if err != nil {
		log.Printf("Error getting session responses: %v", err)
		return
	}

	// Build notification message based on response type
	var notificationMsg string
	switch responseType {
	case domain.ResponseAccepted:
		notificationMsg = fmt.Sprintf("✅ %s идёт на перекур!", responderName)
	case domain.ResponseAcceptedDelayed:
		notificationMsg = fmt.Sprintf("⏱ %s придёт в течение 5 минут!", responderName)
	case domain.ResponseDenied:
		notificationMsg = fmt.Sprintf("❌ %s не идёт на перекур", responderName)
	case domain.ResponseRemote:
		notificationMsg = fmt.Sprintf("🏠 %s на удалёнке сегодня", responderName)
	}

	// Always notify the initiator (unless they're hidden)
	if session.InitiatorID != responderID {
		initiator, _ := b.service.GetUser(session.InitiatorID)
		if initiator == nil || !initiator.IsHidden {
			b.sendMessage(session.InitiatorID, notificationMsg)
		}
	}

	// If response is accept or delayed, notify all other accepted users
	if responseType == domain.ResponseAccepted || responseType == domain.ResponseAcceptedDelayed {
		for _, resp := range responses {
			// Skip the responder themselves and the initiator (already notified)
			if resp.UserID == responderID || resp.UserID == session.InitiatorID {
				continue
			}

			// Only notify users who accepted (not denied or remote)
			if resp.Response == domain.ResponseAccepted || resp.Response == domain.ResponseAcceptedDelayed {
				// Don't notify hidden users
				user, _ := b.service.GetUser(resp.UserID)
				if user == nil || !user.IsHidden {
					b.sendMessage(resp.UserID, notificationMsg)
				}
			}
		}
	}
}
