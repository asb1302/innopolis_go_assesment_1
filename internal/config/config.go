package config

import (
	"time"
)

type Config struct {
	ValidTokens    []string
	WorkerInterval time.Duration
	FilesDir       string
	NumWorkers     int
	MaxRetries     int
	RetryInterval  time.Duration
}

func LoadConfig() *Config {
	return &Config{
		ValidTokens:    []string{"valid_token_1", "valid_token_2"},
		WorkerInterval: 1 * time.Second,
		FilesDir:       "files",
		NumWorkers:     5,
		MaxRetries:     3,
		RetryInterval:  2 * time.Second,
	}
}
