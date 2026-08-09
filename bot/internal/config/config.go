package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken string
	DBURL    string
}

func Load() (*Config, error) {
	_ = godotenv.Load()
	token := os.Getenv("BOT_TOKEN")
	dburl := os.Getenv("DB_URL")
	return &Config{
		BotToken: token,
		DBURL:    dburl,
	}, nil
}
