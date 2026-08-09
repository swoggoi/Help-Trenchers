package logger

import (
	"os"

	"go.uber.org/zap"
)

type Logger struct {
	*zap.SugaredLogger
}

func New() (*Logger, error) {
	if os.Getenv("LOG_LEVEL") == "development" {
		z, err := zap.NewDevelopment()
		if err != nil {
			return nil, err
		}
		return &Logger{z.Sugar()}, nil
	}

	z, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	return &Logger{z.Sugar()}, nil
}
