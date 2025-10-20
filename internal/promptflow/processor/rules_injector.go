package processor

import (
	"fmt"
	"strings"

	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"go.uber.org/zap"
)

type RulesInjector struct {
	BaseProcessor

	promptMode  string
	rulesConfig *config.RulesConfig
}

func NewRulesInjector(promptMode string, rulesConfig *config.RulesConfig) *RulesInjector {
	return &RulesInjector{
		promptMode:  promptMode,
		rulesConfig: rulesConfig,
	}
}

func (r *RulesInjector) Execute(promptMsg *PromptMsg) {
	const method = "RulesInjector.Execute"
	logger.Info("Start injecting rules into system prompt", zap.String("method", method))

	if promptMsg == nil {
		r.Err = fmt.Errorf("received prompt message is empty")
		logger.Error(r.Err.Error(), zap.String("method", method))
		return
	}

	// Check if rules configuration is available
	if r.rulesConfig == nil {
		logger.Error("Rules configuration is not available - this should not happen in production",
			zap.String("method", method))
		r.Err = fmt.Errorf("rules configuration is not available - service configuration error")
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
		// Check if current promptMode is in match_modes list
		if len(agentConfig.MatchModes) == 0 {
			continue // Skip this rule if match_modes is empty
		}

		modeMatched := false
		for _, mode := range agentConfig.MatchModes {
			if mode == r.promptMode {
				modeMatched = true
				break
			}
		}
		if !modeMatched {
			continue // Skip this rule if mode doesn't match
		}

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
