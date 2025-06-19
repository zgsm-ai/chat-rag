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
	config         config.Config
	identity       *types.Identity
	next           Processor
	semanticResult *client.SemanticData
}

func NewSemanticSearch(
	ctx context.Context,
	config config.Config,
	semanticClient client.SemanticInterface,
	identity *types.Identity,
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
	logger.Info("starting system prompt compression", zap.String("method", method))

	if promptMsg == nil {
		logger.Error("nil prompt message received", zap.String("method", method))
		return
	}

	startTime := time.Now()
	defer func() {
		s.Latency = time.Since(startTime).Milliseconds()
	}()

	semanticContext, err := s.searchSemanticContext(utils.GetContentAsString(promptMsg.lastUserMsg))
	if err != nil {
		logger.Error("failed to search semantic context",
			zap.Error(err),
			zap.String("method", method),
		)
		s.Err = err
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
		return
	}

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

	logger.Info("executing semantic search",
		zap.String("query", filteredQuery),
		zap.String("method", method),
	)

	req := s.buildSemanticRequest(filteredQuery)
	resp, err := s.semanticClient.Search(s.ctx, req)
	if err != nil {
		return "", fmt.Errorf("semantic client search: %w", err)
	}
	s.semanticResult = resp

	contextStr := s.buildContextString(resp.Results)
	if contextStr == "" {
		logger.Info("no results above score threshold",
			zap.String("method", method),
		)
		return "", nil
	}

	logger.Info("semantic context built successfully",
		zap.String("method", method),
	)
	return contextStr, nil
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
		if result.Score < s.config.SemanticScoreThreshold {
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

// func (s *SemanticSearch) Execute(promptMsg *PromptMsg) {
// 	logger.Info("[SemanticSearch] starting system prompt compression",
// 		zap.String("method", "Execute"),
// 	)
// 	if promptMsg == nil {
// 		logger.Error("promptMsg is nil!")
// 		return
// 	}

// 	var semanticLatency int64

// 	// Record start time for semantic search
// 	semanticStart := time.Now()
// 	semanticContext, err := s.searchSemanticContext(utils.GetContentAsString(promptMsg.lastUserMsg))
// 	if err != nil {
// 		logger.Error("failed to search semantic",
// 			zap.Error(err),
// 			zap.String("method", "Process"),
// 		)
// 		s.Err = err
// 	}

// 	if semanticContext != "" {
// 		codebaseContextText := model.Content{
// 			Type: model.ContTypeText,
// 			Text: fmt.Sprintf("<codebase_search_details>\n%s\n</codebase_search_details>", semanticContext),
// 		}

// 		var content model.Content

// 		contents, err := content.ExtractMsgContent(promptMsg.lastUserMsg)
// 		if err != nil {
// 			logger.Error("failed to extract user message content",
// 				zap.Error(err),
// 				zap.String("method", "Process"),
// 			)
// 			s.Err = err
// 			return
// 		}

// 		// Insert contextText to user message content
// 		contents = append(contents, codebaseContextText)
// 		lastMsg := &types.Message{
// 			Role:    types.RoleUser,
// 			Content: contents,
// 		}

// 		promptMsg.lastUserMsg = lastMsg
// 	}

// 	semanticLatency = time.Since(semanticStart).Milliseconds()
// 	s.Lantency = semanticLatency
// 	if s.next != nil {
// 		s.next.Execute(promptMsg)
// 	} else {
// 		logger.Error("semantic search completed, but no next processor found",
// 			zap.String("method", "Execute"),
// 		)
// 	}
// }

// func (s *SemanticSearch) SetNext(next Processor) {
// 	s.next = next
// }

// // searchSemanticContext performs semantic search and constructs context string
// func (s *SemanticSearch) searchSemanticContext(query string) (string, error) {
// 	// Filter query to remove environment details
// 	filterQuery := query
// 	if strings.Contains(query, "<environment_details>") {
// 		start := strings.Index(query, "<environment_details>")
// 		end := strings.Index(query, "</environment_details>") + len("</environment_details>")
// 		filterQuery = query[:start] + query[end:]
// 	}
// 	logger.Info("semantic search query",
// 		zap.String("query", filterQuery),
// 		zap.String("method", "searchSemanticContext"),
// 	)

// 	semanticReq := client.SemanticRequest{
// 		ClientId:      s.identity.ClientID,
// 		CodebasePath:  s.identity.ProjectPath,
// 		Query:         filterQuery,
// 		TopK:          s.config.TopK,
// 		Authorization: s.identity.AuthToken,
// 	}

// 	// Execute search
// 	semanticResp, err := s.semanticClient.Search(s.ctx, semanticReq)
// 	if err != nil {
// 		err := fmt.Errorf("failed to search semantic:\n%w", err)
// 		return "", err
// 	}

// 	// Build context string from results
// 	var contextParts []string
// 	logger.Info("semantic search results",
// 		zap.Int("count", len(semanticResp.Results)),
// 		zap.String("method", "searchSemanticContext"),
// 	)
// 	for _, result := range semanticResp.Results {
// 		if result.Score < s.config.SemanticScoreThreshold {
// 			continue
// 		}

// 		contextParts = append(contextParts,
// 			fmt.Sprintf("File path: %s\nScore: %.2f\nCode Chunk: \n%s",
// 				result.FilePath, result.Score, result.Content))
// 	}

// 	semanticContext := "[codebase_search] Result:\n" + strings.Join(contextParts, "\n\n")
// 	logger.Info("searched semantic context",
// 		zap.String("context", semanticContext),
// 		zap.String("method", "searchSemanticContext"),
// 	)

// 	return semanticContext, nil
// }
