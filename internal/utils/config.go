package utils

import (
	"fmt"

	"github.com/spf13/viper"
	"github.com/zgsm-ai/chat-rag/internal/config"
)

// LoadConfig loads configuration from the specified file path using viper
func LoadConfig(configPath string) (config.Config, error) {
	var c config.Config

	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		return c, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := viper.Unmarshal(&c); err != nil {
		return c, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return c, nil
}

// MustLoadConfig loads configuration and panics if there's an error
func MustLoadConfig(configPath string) config.Config {
	c, err := LoadConfig(configPath)
	if err != nil {
		panic("Failed to load config: " + err.Error())
	}
	return c
}
