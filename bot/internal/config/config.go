package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken string
	DBURL    string
}

func Load() (*Config, error) {
	_ = godotenv.Load()
	_ = godotenv.Load(filepath.Join("..", ".env"))
	_ = godotenv.Load(filepath.Join("..", "..", ".env"))
	_ = godotenv.Load(".env.local")

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return nil, errors.New("BOT_TOKEN is required")
	}

	dburl := os.Getenv("DB_URL")
	if dburl == "" {
		return nil, errors.New("DB_URL is required")
	}

	return &Config{
		BotToken: token,
		DBURL:    dburl,
	}, nil
}
