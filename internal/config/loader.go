package config

import (
	"fmt"

	"github.com/spf13/viper"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"go.uber.org/zap"
)

// loadConfig loads configuration from the specified file path using viper
func loadConfig(configPath string) (Config, error) {
	var c Config

	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		return c, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := viper.Unmarshal(&c); err != nil {
		return c, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	logger.Info("loaded config", zap.Any("config", c))

	return c, nil
}

// MustLoadConfig loads configuration and panics if there's an error
func MustLoadConfig(configPath string) Config {
	c, err := loadConfig(configPath)
	if err != nil {
		panic("Failed to load config: " + err.Error())
	}
	return c
}
