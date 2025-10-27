package strategies

import (
	"context"
	"fmt"
	"net/http"

	"github.com/zgsm-ai/chat-rag/internal/bootstrap"
	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/functions"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/promptflow/ds"
	"github.com/zgsm-ai/chat-rag/internal/promptflow/processor"
	"github.com/zgsm-ai/chat-rag/internal/tokenizer"
	"github.com/zgsm-ai/chat-rag/internal/types"
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

	userMsgFilter *processor.UserMsgFilter
	// functionAdapter *processor.FunctionAdapter
	userCompressor *processor.UserCompressor
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
) (*RagCompressProcessor, error) {
	llmClient, err := client.NewLLMClient(
		svcCtx.Config.LLM,
		svcCtx.Config.ContextCompressConfig.SummaryModel,
		copyAndSetQuotaIdentity(headers),
	)
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
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
	p.userMsgFilter = processor.NewUserMsgFilter(p.config.PreciseContextConfig.EnableEnvDetailsFilter)
	p.xmlToolAdapter = processor.NewXmlToolAdapter(p.ctx, p.toolsExecutor)
	// p.functionAdapter = processor.NewFunctionAdapter(
	// 	p.modelName,
	// 	p.config.LLM.FuncCallingModels,
	// 	p.functionsManager,
	// )
	p.userCompressor = processor.NewUserCompressor(
		p.ctx,
		p.config,
		p.llmClient,
		p.tokenCounter,
	)

	// execute chain
	p.start.SetNext(p.userMsgFilter)
	p.userMsgFilter.SetNext(p.xmlToolAdapter)
	// p.userMsgFilter.SetNext(p.functionAdapter)
	p.xmlToolAdapter.SetNext(p.userCompressor)
	p.userCompressor.SetNext(p.end)

	return nil
}

// createProcessedPrompt creates the final processed prompt result
func (p *RagCompressProcessor) createProcessedPrompt(
	promptMsg *processor.PromptMsg,
) *ds.ProcessedPrompt {
	processedMsgs := processor.SetLanguage(p.identity.Language, promptMsg.AssemblePrompt())
	return &ds.ProcessedPrompt{
		Messages:               processedMsgs,
		SummaryLatency:         p.userCompressor.Latency,
		SummaryErr:             p.userCompressor.Err,
		IsUserPromptCompressed: p.userCompressor.Handled,
		Tools:                  promptMsg.GetTools(),
	}
}
