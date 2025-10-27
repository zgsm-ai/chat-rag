package strategies

import (
	"context"
	"net/http"

	"github.com/zgsm-ai/chat-rag/internal/bootstrap"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/promptflow/processor"
)

type RagWithRuleProcessor struct {
	RagCompressProcessor

	promptMode   string
	rulesConfig  *config.RulesConfig
	ruleInjector *processor.RulesInjector
}

// NewRagWithRuleProcessor creates a new processor with rule injection
func NewRagWithRuleProcessor(
	ctx context.Context,
	svcCtx *bootstrap.ServiceContext,
	headers *http.Header,
	identity *model.Identity,
	modelName string,
	promoptMode string,
) (*RagWithRuleProcessor, error) {
	ragCompressProcessor, err := NewRagCompressProcessor(ctx, svcCtx, headers, identity, modelName)
	if err != nil {
		return nil, err
	}

	processor := &RagWithRuleProcessor{
		RagCompressProcessor: *ragCompressProcessor,
		rulesConfig:          svcCtx.RulesConfig,
	}

	if promoptMode == "" {
		promoptMode = "default"
	}
	processor.promptMode = promoptMode
	processor.chainBuilder = processor

	return processor, nil
}

// buildProcessorChain constructs and connects the processor chain
func (r *RagWithRuleProcessor) buildProcessorChain() error {
	r.userMsgFilter = processor.NewUserMsgFilter(r.config.PreciseContextConfig.EnableEnvDetailsFilter)
	r.xmlToolAdapter = processor.NewXmlToolAdapter(r.ctx, r.toolsExecutor)
	r.ruleInjector = processor.NewRulesInjector(r.promptMode, r.rulesConfig)
	r.userCompressor = processor.NewUserCompressor(
		r.ctx,
		r.config,
		r.llmClient,
		r.tokenCounter,
	)

	// build chain
	r.start.SetNext(r.ruleInjector)
	r.ruleInjector.SetNext(r.userMsgFilter)
	r.userMsgFilter.SetNext(r.xmlToolAdapter)
	r.xmlToolAdapter.SetNext(r.userCompressor)
	r.userCompressor.SetNext(r.end)

	return nil
}
