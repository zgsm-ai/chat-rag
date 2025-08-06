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
	MatchKey string `mapstructure:"match_key"`
	Rules    string `mapstructure:"rules"`
}

type RulesConfig struct {
	Agents map[string]AgentConfig `yaml:"agents"`
}

func NewRulesInjector() *RulesInjector {
	return &RulesInjector{}
}

func (r *RulesInjector) loadRulesConfig() error {
	// 获取项目根目录的路径
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// 构建配置文件路径
	configPath := filepath.Join(projectRoot, "etc", "rules.yaml")

	// 使用config包加载YAML配置
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

	// 加载规则配置
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

	// 提取开头第一段内容（以第一个换行符或空行分隔）
	firstParagraph := content
	if idx := strings.IndexAny(content, "\n\r"); idx != -1 {
		firstParagraph = content[:idx]
	}

	for agentName, agentConfig := range r.rulesConfig.Agents {
		if strings.Contains(firstParagraph, agentConfig.MatchKey) {
			logger.Info("Detected agent and adding rules", zap.String("agent_type", agentName))
			// Add the rules to the end of the system content
			result := content + "\n# Rules from " + agentName + "\n" + agentConfig.Rules
			return result, nil
		}
	}

	// No matching agent type found, return original content
	logger.Info("No matching agent type found in first paragraph, skipping rules injection")
	return content, nil
}
