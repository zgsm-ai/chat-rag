package config

import (
	"fmt"
	"os"
	"path/filepath"

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

// LoadRulesConfig loads the rules configuration from etc/rules.yaml
func LoadRulesConfig() (*RulesConfig, error) {
	// Get the project root directory path
	projectRoot, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Build the configuration file path
	configPath := filepath.Join(projectRoot, "etc", "rules.yaml")

	// Load YAML configuration using config package
	rulesConfig, err := LoadYAML[RulesConfig](configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load rules config: %w", err)
	}

	// Log successful loading
	logger.Info("Rules configuration loaded successfully at service startup",
		zap.String("config_path", configPath),
		zap.Int("agents_count", len(rulesConfig.Agents)))

	return rulesConfig, nil
}
