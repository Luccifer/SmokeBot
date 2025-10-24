package config

import (
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	TelegramToken string
	DatabasePath  string
	WorkingHours  WorkingHours
}

// WorkingHours defines when the bot should operate
type WorkingHours struct {
	StartHour int
	EndHour   int
	Location  *time.Location
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Try to load .env file (ignore error if not exists)
	_ = godotenv.Load()

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		// Hardcoded token as fallback
		token = ""
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./smoke_bot.db"
	}

	// Default to local timezone
	loc, err := time.LoadLocation("Local")
	if err != nil {
		loc = time.UTC
	}

	return &Config{
		TelegramToken: token,
		DatabasePath:  dbPath,
		WorkingHours: WorkingHours{
			StartHour: 9,
			EndHour:   23,
			Location:  loc,
		},
	}, nil
}

// IsWorkingHours checks if current time is within working hours
func (c *Config) IsWorkingHours() bool {
	now := time.Now().In(c.WorkingHours.Location)
	hour := now.Hour()
	return hour >= c.WorkingHours.StartHour && hour < c.WorkingHours.EndHour
}
