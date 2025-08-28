package processor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
	"go.uber.org/zap"
)

type SemanticSearch struct {
	Recorder
	ctx            context.Context
	semanticClient client.SemanticInterface
	config         config.SemanticSearchConfig
	identity       *model.Identity

	next Processor

	// SemanticResult is the result of the semantic search.
	SemanticResult string
}

func NewSemanticSearch(
	ctx context.Context,
	config config.SemanticSearchConfig,
	semanticClient client.SemanticInterface,
	identity *model.Identity,
) *SemanticSearch {
	return &SemanticSearch{
		ctx:            ctx,
		config:         config,
		semanticClient: semanticClient,
		identity:       identity,
	}
}

func (s *SemanticSearch) Execute(promptMsg *PromptMsg) {
	const method = "SemanticSearch.Execute"
	logger.Info("starting semantic search", zap.String("method", method))

	if promptMsg == nil {
		logger.Error("nil prompt message received", zap.String("method", method))
		s.Err = fmt.Errorf("nil prompt message received")
		return
	}

	startTime := time.Now()
	defer func() {
		s.Latency = time.Since(startTime).Milliseconds()
	}()

	semanticContext, err := s.searchSemanticContext(utils.GetContentAsString(promptMsg.lastUserMsg.Content))
	if err != nil {
		logger.Error("failed to search semantic context",
			zap.Error(err),
			zap.String("method", method),
		)
		s.Err = err
		s.passToNext(promptMsg)
		return
	}

	if semanticContext == "" {
		logger.Info("no relevant semantic context found", zap.String("method", method))
		s.passToNext(promptMsg)
		return
	}

	if err := s.enrichPromptWithContext(promptMsg, semanticContext); err != nil {
		logger.Error("failed to enrich prompt with context",
			zap.Error(err),
			zap.String("method", method),
		)
		s.Err = err
		s.passToNext(promptMsg)
		return
	}

	s.Handled = true
	s.passToNext(promptMsg)
}

func (s *SemanticSearch) SetNext(next Processor) {
	s.next = next
}

func (s *SemanticSearch) passToNext(promptMsg *PromptMsg) {
	if s.next == nil {
		logger.Error("semantic search completed but no next processor configured",
			zap.String("method", "SemanticSearch.Execute"),
		)
		return
	}
	s.next.Execute(promptMsg)
}

func (s *SemanticSearch) enrichPromptWithContext(promptMsg *PromptMsg, context string) error {
	codebaseContext := model.Content{
		Type: model.ContTypeText,
		Text: fmt.Sprintf("<codebase_search_details>\n%s\n</codebase_search_details>", context),
	}

	var content model.Content

	contents, err := content.ExtractMsgContent(promptMsg.lastUserMsg)
	if err != nil {
		return fmt.Errorf("extract message content: %w", err)
	}

	contents = append(contents, codebaseContext)
	promptMsg.lastUserMsg = &types.Message{
		Role:    types.RoleUser,
		Content: contents,
	}
	return nil
}

func (s *SemanticSearch) searchSemanticContext(query string) (string, error) {
	const method = "SemanticSearch.searchSemanticContext"
	filteredQuery := s.filterQuery(query)

	logger.Info("executing semantic search", zap.String("method", method))

	req := s.buildSemanticRequest(filteredQuery)
	resp, err := s.semanticClient.Search(s.ctx, req)
	if err != nil {
		return "", fmt.Errorf("semantic client search: %w", err)
	}
	s.SemanticResult = resp

	// Since resp is now a string (JSON), we need to parse it to extract results
	// For now, we'll return the raw JSON response as context
	if resp == "" {
		logger.Info("no results above score threshold",
			zap.String("method", method),
		)
		return "", nil
	}

	// Return the raw JSON response as context
	logger.Info("semantic context built successfully",
		zap.String("method", method),
	)
	return resp, nil
}

func (s *SemanticSearch) filterQuery(query string) string {
	if !strings.Contains(query, "<environment_details>") {
		return query
	}

	start := strings.Index(query, "<environment_details>")
	end := strings.Index(query, "</environment_details>") + len("</environment_details>")
	return query[:start] + query[end:]
}

func (s *SemanticSearch) buildSemanticRequest(query string) client.SemanticRequest {
	return client.SemanticRequest{
		ClientId:      s.identity.ClientID,
		CodebasePath:  s.identity.ProjectPath,
		Query:         query,
		TopK:          s.config.TopK,
		Authorization: s.identity.AuthToken,
	}
}

func (s *SemanticSearch) buildContextString(results []client.SemanticResult) string {
	var contextParts []string

	for _, result := range results {
		if result.Score < s.config.ScoreThreshold {
			continue
		}

		contextParts = append(contextParts,
			fmt.Sprintf("File path: %s\nScore: %.2f\nCode Chunk: \n%s",
				result.FilePath, result.Score, result.Content))
	}

	if len(contextParts) == 0 {
		return ""
	}

	return "[codebase_search] Result:\n" + strings.Join(contextParts, "\n\n")
}
