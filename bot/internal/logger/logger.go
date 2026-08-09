package logger

import (
	"go.uber.org/zap"
)

type Logger struct {
	*zap.SugaredLogger
}

func New() (*Logger, error) {
	z, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	return &Logger{z.Sugar()}, nil
}
