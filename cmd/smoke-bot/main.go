package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	
	"github.com/glebk/smoke-bot/internal/bot"
	"github.com/glebk/smoke-bot/internal/config"
	"github.com/glebk/smoke-bot/internal/repository/sqlite"
	"github.com/glebk/smoke-bot/internal/service"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	// Initialize database
	db, err := sqlite.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	log.Printf("Database initialized at: %s", cfg.DatabasePath)
	
	// Initialize repositories
	userRepo := sqlite.NewUserRepository(db)
	sessionRepo := sqlite.NewSessionRepository(db)
	
	// Initialize service
	smokeService := service.NewSmokeService(userRepo, sessionRepo)
	
	// Initialize bot
	telegramBot, err := bot.New(cfg.TelegramToken, smokeService, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize bot: %v", err)
	}
	
	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	
	// Start bot in goroutine
	go func() {
		log.Println("Bot started. Press Ctrl+C to stop.")
		if err := telegramBot.Start(); err != nil {
			log.Fatalf("Bot stopped with error: %v", err)
		}
	}()
	
	// Wait for stop signal
	<-stop
	log.Println("Shutting down gracefully...")
}

