package router

import (
	"github.com/zgsm-ai/chat-rag/internal/config"
	ssemantic "github.com/zgsm-ai/chat-rag/internal/router/strategies/semantic"
)

// NewRunner creates a strategy instance based on config
func NewRunner(cfg config.RouterConfig) Strategy {
	switch cfg.Strategy {
	case "semantic", "":
		return ssemantic.New(cfg.Semantic)
	default:
		return nil
	}
}
