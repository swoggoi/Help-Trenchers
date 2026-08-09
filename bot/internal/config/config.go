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

// candidatePaths — список путей к .env, которые пробуем загрузить.
// Перебираем как относительно cwd, так и абсолютно (от места запуска вверх),
// чтобы .env подхватывался независимо от того, откуда запущен бот.
func candidatePaths() []string {
	var paths []string
	paths = append(paths, ".env")
	paths = append(paths, filepath.Join("..", ".env"))
	paths = append(paths, filepath.Join("..", "..", ".env"))
	paths = append(paths, filepath.Join("bot", ".env"))

	if wd, err := os.Getwd(); err == nil {
		cur := wd
		for i := 0; i < 6; i++ {
			paths = append(paths, filepath.Join(cur, ".env"))
			parent := filepath.Dir(cur)
			if parent == cur {
				break
			}
			cur = parent
		}
	}
	return paths
}

func Load() (*Config, error) {
	loaded := false
	for _, p := range candidatePaths() {
		if _, err := os.Stat(p); err == nil {
			if err := godotenv.Load(p); err == nil {
				loaded = true
			}
		}
	}
	// последняя попытка — стандартный поиск godotenv
	_ = godotenv.Load()

	if !loaded && os.Getenv("BOT_TOKEN") == "" {
		// не критично для ошибки, но логируем
		_, _ = os.Stderr.WriteString("WARNING: .env not found in known locations\n")
	}

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
