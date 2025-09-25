package strategies

import (
	"context"
	"net/http"

	"github.com/zgsm-ai/chat-rag/internal/bootstrap"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/promptflow/processor"
)

type StrictWorflowProcessor struct {
	RagCompressProcessor

	ruleInjector *processor.RulesInjector
}

// NewStrictWorflowProcessor creates a new strict workflow processor with rule injection
func NewStrictWorflowProcessor(
	ctx context.Context,
	svcCtx *bootstrap.ServiceContext,
	headers *http.Header,
	identity *model.Identity,
	modelName string,
) (*StrictWorflowProcessor, error) {
	ragCompressProcessor, err := NewRagCompressProcessor(ctx, svcCtx, headers, identity, modelName)
	if err != nil {
		return nil, err
	}

	processor := &StrictWorflowProcessor{
		RagCompressProcessor: *ragCompressProcessor,
	}

	processor.chainBuilder = processor

	return processor, nil
}

// buildProcessorChain constructs and connects the processor chain
func (p *StrictWorflowProcessor) buildProcessorChain() error {
	p.userMsgFilter = processor.NewUserMsgFilter()
	p.xmlToolAdapter = processor.NewXmlToolAdapter(p.ctx, p.toolsExecutor)
	p.ruleInjector = processor.NewRulesInjector()
	p.userCompressor = processor.NewUserCompressor(
		p.ctx,
		p.config,
		p.llmClient,
		p.tokenCounter,
	)

	// build chain
	p.start.SetNext(p.ruleInjector)
	p.ruleInjector.SetNext(p.userMsgFilter)
	p.userMsgFilter.SetNext(p.xmlToolAdapter)
	p.xmlToolAdapter.SetNext(p.userCompressor)
	p.userCompressor.SetNext(p.end)

	return nil
}
