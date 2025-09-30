package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	config "github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"go.uber.org/zap"
)

type RulesInjector struct {
	BaseProcessor
	rulesConfig *RulesConfig
}

type AgentConfig struct {
	MatchKeys []string `mapstructure:"match_keys"`
	Rules     string   `mapstructure:"rules"`
}

type RulesConfig struct {
	Agents map[string]AgentConfig `yaml:"agents"`
}

func NewRulesInjector() *RulesInjector {
	return &RulesInjector{}
}

func (r *RulesInjector) loadRulesConfig() error {
	// Get the project root directory path
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Build the configuration file path
	configPath := filepath.Join(projectRoot, "etc", "rules.yaml")

	// Load YAML configuration using config package
	config, err := config.LoadYAML[RulesConfig](configPath)
	if err != nil {
		return fmt.Errorf("failed to load rules config: %w", err)
	}

	r.rulesConfig = config
	logger.Info("Successfully loaded rules configuration", zap.String("config_path", configPath))

	return nil
}

func (r *RulesInjector) Execute(promptMsg *PromptMsg) {
	const method = "RulesInjector.Execute"
	logger.Info("Start injecting rules into system prompt", zap.String("method", method))

	if promptMsg == nil {
		r.Err = fmt.Errorf("received prompt message is empty")
		logger.Error(r.Err.Error(), zap.String("method", method))
		return
	}

	// Load rules configuration
	if err := r.loadRulesConfig(); err != nil {
		logger.Warn("Failed to load rules configuration",
			zap.String("method", method),
			zap.Error(err))
		r.Err = fmt.Errorf("failed to load rules configuration: %w", err)
		r.passToNext(promptMsg)
		return
	}

	systemContent, err := r.extractSystemContent(promptMsg.systemMsg)
	if err != nil {
		logger.Warn("Failed to extract system message content",
			zap.String("method", method),
			zap.Error(err))
		r.Err = fmt.Errorf("failed to extract system message content: %w", err)
		r.passToNext(promptMsg)
		return
	}

	// Process system content to inject rules based on agent type
	updatedContent, err := r.injectRulesIntoSystemContent(systemContent)
	if err != nil {
		logger.Warn("Failed to inject rules into system content",
			zap.String("method", method),
			zap.Error(err))
		r.Err = fmt.Errorf("failed to inject rules into system content: %w", err)
		r.passToNext(promptMsg)
		return
	}

	// Update the system message with the modified content
	promptMsg.UpdateSystemMsg(updatedContent)

	r.Handled = true
	r.passToNext(promptMsg)
}

// injectRulesIntoSystemContent injects rules based on the agent type detected in system content
func (r *RulesInjector) injectRulesIntoSystemContent(content string) (string, error) {
	if len(r.rulesConfig.Agents) > 0 {
		content = content + "\n\nRules:\n"
	}

	// Extract the first paragraph content (separated by the first newline or empty line)
	firstParagraph := content
	if idx := strings.IndexAny(content, "\n\r"); idx != -1 {
		firstParagraph = content[:idx]
	}

	for agentName, agentConfig := range r.rulesConfig.Agents {
		// Iterate through all match keys for each agent
		for _, matchKey := range agentConfig.MatchKeys {
			if strings.Contains(firstParagraph, matchKey) {
				logger.Info("Detected agent and adding rules",
					zap.String("agent_type", agentName),
					zap.String("matched_key", matchKey))
				// Add the rules to the end of the system content
				content = content + "\n\n# Rules from " + agentName + "\n" + agentConfig.Rules
			}
		}
	}

	return content, nil
}
