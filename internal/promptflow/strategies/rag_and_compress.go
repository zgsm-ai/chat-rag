package strategies

import (
	"context"
	"fmt"

	"github.com/zgsm-ai/chat-rag/internal/bootstrap"
	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/promptflow/ds"
	"github.com/zgsm-ai/chat-rag/internal/promptflow/processor"
	"github.com/zgsm-ai/chat-rag/internal/tokenizer"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

type RagProcessor struct {
	ctx            context.Context
	semanticClient client.SemanticInterface
	llmClient      client.LLMInterface
	tokenCounter   *tokenizer.TokenCounter
	config         config.Config
	identity       *model.Identity

	systemCompressor *processor.SystemCompressor
	semanticSearch   *processor.SemanticSearch
	userCompressor   *processor.UserCompressor
	end              *processor.End
}

// NewRagProcessor creates a new RAG compression processor
func NewRagProcessor(
	ctx context.Context,
	svcCtx *bootstrap.ServiceContext,
	identity *model.Identity,
) (*RagProcessor, error) {
	llmClient, err := client.NewLLMClient(
		svcCtx.Config.LLMEndpoint,
		svcCtx.Config.SummaryModel,
		svcCtx.ReqCtx.Headers,
	)
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
	}

	return &RagProcessor{
		ctx:            ctx,
		semanticClient: client.NewSemanticClient(svcCtx.Config.SemanticApiEndpoint),
		llmClient:      llmClient,
		config:         svcCtx.Config,
		tokenCounter:   svcCtx.TokenCounter,
		identity:       identity,
	}, nil
}

// Arrange processes the prompt with RAG compression
func (p *RagProcessor) Arrange(messages []types.Message) (*ds.ProcessedPrompt, error) {
	promptMsg, err := processor.NewPromptMsg(messages)
	if err != nil {
		return &ds.ProcessedPrompt{
			Messages: messages,
		}, fmt.Errorf("create prompt message: %w", err)
	}

	if err := p.buildProcessorChain(); err != nil {
		return &ds.ProcessedPrompt{
			Messages: messages,
		}, fmt.Errorf("build processor chain: %w", err)
	}

	p.systemCompressor.Execute(promptMsg)

	return p.createProcessedPrompt(promptMsg), nil
}

// buildProcessorChain constructs and connects the processor chain
func (p *RagProcessor) buildProcessorChain() error {
	p.systemCompressor = processor.NewSystemCompressor(
		p.config.SystemPromptSplitStr,
		p.llmClient,
	)
	p.semanticSearch = processor.NewSemanticSearch(
		p.ctx,
		p.config,
		p.semanticClient,
		p.identity,
	)
	p.userCompressor = processor.NewUserCompressor(
		p.ctx,
		p.config,
		p.llmClient,
		p.tokenCounter,
	)
	p.end = processor.NewEndpoint()

	// chain order: system -> semantic -> user
	p.systemCompressor.SetNext(p.semanticSearch)
	p.semanticSearch.SetNext(p.userCompressor)
	p.userCompressor.SetNext(p.end)

	return nil
}

// createProcessedPrompt creates the final processed prompt result
func (p *RagProcessor) createProcessedPrompt(
	promptMsg *processor.PromptMsg,
) *ds.ProcessedPrompt {
	return &ds.ProcessedPrompt{
		Messages:        promptMsg.AssemblePrompt(),
		SemanticLatency: p.semanticSearch.Latency,
		SemanticContext: p.semanticSearch.SemanticResult,
		SemanticErr:     p.semanticSearch.Err,
		SummaryLatency:  p.userCompressor.Latency,
		SummaryErr:      p.userCompressor.Err,
		IsCompressed:    p.userCompressor.Handled,
	}
}
