package config

import (
	"fmt"

	"github.com/spf13/viper"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"go.uber.org/zap"
)

// LoadYAML loads yaml from the specified file path using viper
func LoadYAML[T any](path string) (*T, error) {
	var yaml T

	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read YAML: %w", err)
	}

	if err := viper.Unmarshal(&yaml); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	return &yaml, nil
}

// MustLoadConfig loads configuration and panics if there's an error
func MustLoadConfig(configPath string) Config {
	c, err := LoadYAML[Config](configPath)
	if err != nil {
		panic("Failed to load config: " + err.Error())
	}

	logger.Info("loaded config", zap.Any("config", c))
	return *c
}
