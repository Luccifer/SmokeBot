# Smoke Break Bot 🚬

A professional Telegram bot written in Go that helps coordinate smoke breaks among colleagues at the office.

## Features

- **Invite colleagues for smoke breaks** - Simply press a button or use a command
- **Smart notifications** - Only active (non-remote) users receive invitations
- **Multiple response options**:
  - ✅ I'm coming - Accept immediately
  - ⏱ In 5 minutes - Accept with a delay
  - ❌ Not now - Decline the invitation
  - 🏠 I'm remote - Mark as remote (stops all notifications until next day)
- **Working hours validation** - Only processes requests between 09:00 and 23:00
- **Real-time session status** - Track who's coming and who declined
- **Automatic remote status expiration** - Remote status automatically clears at 23:59

## Architecture

This bot follows **clean architecture** principles and Go best practices:

```
smoke-bot/
├── cmd/
│   └── smoke-bot/          # Application entry point
│       └── main.go
├── internal/
│   ├── bot/                # Telegram bot handlers
│   │   └── bot.go
│   ├── config/             # Configuration management
│   │   └── config.go
│   ├── domain/             # Domain models and interfaces
│   │   ├── user.go
│   │   └── session.go
│   ├── repository/         # Data access layer
│   │   └── sqlite/
│   │       ├── database.go
│   │       ├── user_repository.go
│   │       └── session_repository.go
│   └── service/            # Business logic layer
│       └── smoke_service.go
├── go.mod
├── go.sum
└── README.md
```

### Layers

1. **Domain Layer** (`internal/domain/`) - Core business entities and repository interfaces
2. **Repository Layer** (`internal/repository/`) - Data persistence implementation (SQLite)
3. **Service Layer** (`internal/service/`) - Business logic and use cases
4. **Bot Layer** (`internal/bot/`) - Telegram bot handlers and user interaction
5. **Config Layer** (`internal/config/`) - Configuration and environment management

## Prerequisites

- Go 1.21 or higher
- A Telegram Bot Token (get it from [@BotFather](https://t.me/botfather))

## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd smoke-bot
```

2. Install dependencies:
```bash
go mod download
```

3. Create a `.env` file in the project root:
```env
TELEGRAM_BOT_TOKEN=your_bot_token_here
DATABASE_PATH=./smoke_bot.db
```

4. Build the application:
```bash
go build -o smoke-bot cmd/smoke-bot/main.go
```

## Usage

### Running the bot

```bash
./smoke-bot
```

Or with environment variables directly:
```bash
TELEGRAM_BOT_TOKEN=your_token ./smoke-bot
```

### Bot Commands

- `/start` - Start the bot and display the main menu
- `/smoke` - Initiate a smoke break session
- `/status` - View current session status
- `/help` - Display help information

### Keyboard Shortcut

Use the **🚬 Let's go smoke!** button that appears after `/start` for quick access.

## How It Works

1. **User initiates a session** - Press "🚬 Let's go smoke!" or use `/smoke`
2. **Validation** - Bot checks if it's working hours (09:00-23:00)
3. **Notification** - All active colleagues receive an invitation with action buttons
4. **Response tracking** - Each response is recorded and visible in session status
5. **Remote status** - Users who select "I'm remote" won't receive notifications until tomorrow

## Database

The bot uses **SQLite** for data persistence with the following tables:

- `users` - Registered bot users and their remote status
- `sessions` - Smoking sessions and their status
- `session_responses` - User responses to session invitations

## Development

### Project Structure Highlights

- **Dependency Injection** - Clean dependency flow from main → bot → service → repository
- **Interface-based Design** - Repository interfaces in domain layer enable easy testing
- **Error Handling** - Proper error wrapping and logging throughout
- **Transaction Safety** - Database operations with proper constraint handling

### Adding New Features

1. Define new domain models in `internal/domain/`
2. Add repository methods if needed
3. Implement business logic in `internal/service/`
4. Add bot handlers in `internal/bot/`

### Testing

```bash
go test ./...
```

## Configuration

Environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `TELEGRAM_BOT_TOKEN` | Your Telegram bot token | *required* |
| `DATABASE_PATH` | Path to SQLite database file | `./smoke_bot.db` |

## Best Practices Applied

✅ Clean Architecture  
✅ Separation of Concerns  
✅ Repository Pattern  
✅ Dependency Injection  
✅ Interface-based Design  
✅ Proper Error Handling  
✅ Structured Logging  
✅ Environment Configuration  
✅ Graceful Shutdown  
✅ Database Migrations  

## License

MIT License

## Contributing

Contributions are welcome! Please follow Go best practices and maintain the clean architecture structure.

## Support

For issues or questions, please open an issue on GitHub.

---

Made with ❤️ and ☕ by developer who understand the importance of good breaks!

