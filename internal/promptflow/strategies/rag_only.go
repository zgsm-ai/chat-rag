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

type RagOnlyProcessor struct {
	ctx            context.Context
	semanticClient client.SemanticInterface
	tokenCounter   *tokenizer.TokenCounter
	config         config.Config
	identity       *model.Identity

	semanticSearch *processor.SemanticSearch
	end            *processor.End
}

// NewRagOnlyProcessor creates a new RAG compression processor
func NewRagOnlyProcessor(
	ctx context.Context,
	svcCtx *bootstrap.ServiceContext,
	identity *model.Identity,
) (*RagOnlyProcessor, error) {
	return &RagOnlyProcessor{
		ctx:            ctx,
		semanticClient: client.NewSemanticClient(svcCtx.Config.Tools.SemanticSearch),
		config:         svcCtx.Config,
		tokenCounter:   svcCtx.TokenCounter,
		identity:       identity,
	}, nil
}

// Arrange processes the prompt with RAG compression
func (p *RagOnlyProcessor) Arrange(messages []types.Message) (*ds.ProcessedPrompt, error) {
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

	p.semanticSearch.Execute(promptMsg)

	return p.createProcessedPrompt(promptMsg), nil
}

// buildProcessorChain constructs and connects the processor chain
func (p *RagOnlyProcessor) buildProcessorChain() error {
	p.semanticSearch = processor.NewSemanticSearch(
		p.ctx,
		p.config.Tools.SemanticSearch,
		p.semanticClient,
		p.identity,
	)
	p.end = processor.NewEndpoint()

	// chain order: semantic -> end
	p.semanticSearch.SetNext(p.end)

	return nil
}

// createProcessedPrompt creates the final processed prompt result
func (p *RagOnlyProcessor) createProcessedPrompt(
	promptMsg *processor.PromptMsg,
) *ds.ProcessedPrompt {
	processedMsgs := processor.SetLanguage(p.identity.Language, promptMsg.AssemblePrompt())
	return &ds.ProcessedPrompt{
		Messages: processedMsgs,
	}
}
