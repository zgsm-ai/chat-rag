package strategies

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/zgsm-ai/chat-rag/internal/bootstrap"
	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/functions"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/promptflow/ds"
	"github.com/zgsm-ai/chat-rag/internal/promptflow/processor"
	"github.com/zgsm-ai/chat-rag/internal/tokenizer"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
	"go.uber.org/zap"
)

// ProcessorChainBuilder is an interface for building the processor chain
type ProcessorChainBuilder interface {
	buildProcessorChain() error
}

type RagCompressProcessor struct {
	ctx          context.Context
	llmClient    client.LLMInterface
	tokenCounter *tokenizer.TokenCounter
	config       config.Config
	identity     *model.Identity
	// functionsManager *functions.ToolManager
	modelName     string
	toolsExecutor functions.ToolExecutor
	agentName     string // detected agent type
	promptMode    string // current prompt mode

	userMsgFilter *processor.UserMsgFilter
	// functionAdapter *processor.FunctionAdapter
	// userCompressor *processor.UserCompressor
	xmlToolAdapter *processor.XmlToolAdapter
	start          *processor.Start
	end            *processor.End

	// chainBuilder used to build the processor chain
	chainBuilder ProcessorChainBuilder
}

// copyAndSetQuotaIdentity
func copyAndSetQuotaIdentity(headers *http.Header) *http.Header {
	headersCopy := make(http.Header)
	for k, v := range *headers {
		headersCopy[k] = v
	}
	headersCopy.Set(types.HeaderQuotaIdentity, "system")
	return &headersCopy
}

// NewRagCompressProcessor creates a new RAG compression processor
func NewRagCompressProcessor(
	ctx context.Context,
	svcCtx *bootstrap.ServiceContext,
	headers *http.Header,
	identity *model.Identity,
	modelName string,
	promptMode string,
) (*RagCompressProcessor, error) {
	llmClient, err := client.NewLLMClient(
		svcCtx.Config.LLM,
		svcCtx.Config.ContextCompressConfig.SummaryModel,
		copyAndSetQuotaIdentity(headers),
	)
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
	}

	if promptMode == "" {
		promptMode = "vibe"
	}

	processor := &RagCompressProcessor{
		ctx:          ctx,
		modelName:    modelName,
		llmClient:    llmClient,
		config:       svcCtx.Config,
		tokenCounter: svcCtx.TokenCounter,
		identity:     identity,
		// functionsManager: svcCtx.FunctionsManager,
		toolsExecutor: svcCtx.ToolExecutor,
		promptMode:    promptMode,
		start:         processor.NewStartPoint(),
		end:           processor.NewEndpoint(),
	}

	processor.chainBuilder = processor

	return processor, nil
}

// Arrange processes the prompt with RAG compression
func (p *RagCompressProcessor) Arrange(messages []types.Message) (*ds.ProcessedPrompt, error) {
	promptMsg, err := processor.NewPromptMsg(messages)
	if err != nil {
		return &ds.ProcessedPrompt{
			Messages: messages,
		}, fmt.Errorf("create prompt message: %w", err)
	}

	// Detect agent type from system message
	systemContent, err := utils.ExtractSystemContent(promptMsg.GetSystemMsg())
	if err != nil {
		logger.WarnC(p.ctx, "Failed to extract system content", zap.Error(err))
	} else {
		p.agentName = p.detectAgent(systemContent)
	}

	// use polymorphism to call the buildProcessorChain method of the subclass
	if err := p.chainBuilder.buildProcessorChain(); err != nil {
		return &ds.ProcessedPrompt{
			Messages: messages,
		}, fmt.Errorf("build processor chain: %w", err)
	}

	p.start.Execute(promptMsg)

	return p.createProcessedPrompt(promptMsg), nil
}

// buildProcessorChain constructs and connects the processor chain
func (p *RagCompressProcessor) buildProcessorChain() error {
	p.userMsgFilter = processor.NewUserMsgFilter(
		&p.config.PreciseContextConfig,
		p.promptMode,
		p.agentName,
		p.tokenCounter,
	)
	p.xmlToolAdapter = processor.NewXmlToolAdapter(
		p.ctx,
		p.toolsExecutor,
		&p.config.Tools,
		p.agentName,
		p.promptMode,
	)
	// p.userCompressor = processor.NewUserCompressor(
	// 	p.ctx,
	// 	p.config,
	// 	p.llmClient,
	// 	p.tokenCounter,
	// )

	// execute chain
	p.start.SetNext(p.userMsgFilter)
	p.userMsgFilter.SetNext(p.xmlToolAdapter)
	// p.xmlToolAdapter.SetNext(p.userCompressor)
	p.xmlToolAdapter.SetNext(p.end)

	return nil
}

// createProcessedPrompt creates the final processed prompt result
func (p *RagCompressProcessor) createProcessedPrompt(
	promptMsg *processor.PromptMsg,
) *ds.ProcessedPrompt {
	processedMsgs := processor.SetLanguage(p.identity.Language, promptMsg.AssemblePrompt())
	return &ds.ProcessedPrompt{
		Messages:     processedMsgs,
		Tools:        promptMsg.GetTools(),
		Agent:        p.agentName,
		TokenMetrics: p.userMsgFilter.TokenMetrics,
	}
}

// detectAgent detects the agent type based on the system message content
func (p *RagCompressProcessor) detectAgent(systemMsg string) string {
	if len(p.config.PreciseContextConfig.AgentsMatch) == 0 {
		logger.Info("No agents configured for matching",
			zap.String("method", "RagCompressProcessor.detectAgent"))
		return ""
	}

	// Extract the first paragraph content (separated by the first newline or empty line)
	firstParagraph := systemMsg
	if idx := strings.IndexAny(systemMsg, "\n\r"); idx != -1 {
		firstParagraph = systemMsg[:idx]
	}

	// Iterate through all agents to find a match
	for _, agentConfig := range p.config.PreciseContextConfig.AgentsMatch {
		if strings.Contains(firstParagraph, agentConfig.MatchKey) {
			logger.InfoC(p.ctx, "Detected agent",
				zap.String("prompt_mode", p.promptMode),
				zap.String("agent", agentConfig.AgentName))
			return agentConfig.AgentName
		}
	}

	logger.InfoC(p.ctx, "No agent type detected")
	return ""
}
