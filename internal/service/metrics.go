package service

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/zgsm-ai/chat-rag/internal/model"
)

// MetricsService handles Prometheus metrics collection
type MetricsService struct {
	// Request metrics
	requestsTotal *prometheus.CounterVec

	// Token metrics
	originalTokensTotal   *prometheus.CounterVec
	compressedTokensTotal *prometheus.CounterVec
	compressionRatio      *prometheus.HistogramVec

	// Latency metrics
	semanticLatency  *prometheus.HistogramVec
	summaryLatency   *prometheus.HistogramVec
	mainModelLatency *prometheus.HistogramVec
	totalLatency     *prometheus.HistogramVec

	// Compression metrics
	userPromptCompressed *prometheus.CounterVec

	// Response metrics
	responseTokens *prometheus.CounterVec

	// Error metrics
	errorsTotal *prometheus.CounterVec
}

// NewMetricsService creates a new metrics service
func NewMetricsService() *MetricsService {
	ms := &MetricsService{
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "chat_rag_requests_total",
				Help: "Total number of chat completion requests",
			},
			[]string{"client_id", "model", "category", "user"},
		),

		originalTokensTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "chat_rag_original_tokens_total",
				Help: "Total number of original tokens processed",
			},
			[]string{"client_id", "model", "token_scope", "user"},
		),

		compressedTokensTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "chat_rag_compressed_tokens_total",
				Help: "Total number of compressed tokens processed",
			},
			[]string{"client_id", "model", "token_scope", "user"},
		),

		compressionRatio: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "chat_rag_compression_ratio",
				Help:    "Distribution of compression ratios",
				Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
			},
			[]string{"client_id", "model", "user"},
		),

		semanticLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "chat_rag_semantic_latency_ms",
				Help:    "Semantic processing latency in milliseconds",
				Buckets: []float64{10, 50, 100, 200, 500, 1000, 2000, 5000},
			},
			[]string{"client_id", "model", "user"},
		),

		summaryLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "chat_rag_summary_latency_ms",
				Help:    "Summary processing latency in milliseconds",
				Buckets: []float64{10, 50, 100, 200, 500, 1000, 2000, 5000},
			},
			[]string{"client_id", "model", "user"},
		),

		mainModelLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "chat_rag_main_model_latency_ms",
				Help:    "Main model processing latency in milliseconds",
				Buckets: []float64{100, 500, 1000, 2000, 5000, 10000, 20000},
			},
			[]string{"client_id", "model", "user"},
		),

		totalLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "chat_rag_total_latency_ms",
				Help:    "Total processing latency in milliseconds",
				Buckets: []float64{100, 500, 1000, 2000, 5000, 10000, 20000, 30000},
			},
			[]string{"client_id", "model", "user"},
		),

		userPromptCompressed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "chat_rag_user_prompt_compressed_total",
				Help: "Total number of requests where user prompt was compressed",
			},
			[]string{"client_id", "model", "user"},
		),

		responseTokens: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "chat_rag_response_tokens_total",
				Help: "Total number of response tokens generated",
			},
			[]string{"client_id", "model", "user"},
		),

		errorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "chat_rag_errors_total",
				Help: "Total number of errors encountered",
			},
			[]string{"client_id", "model", "error_type", "user"},
		),
	}

	// Register all metrics
	prometheus.MustRegister(
		ms.requestsTotal,
		ms.originalTokensTotal,
		ms.compressedTokensTotal,
		ms.compressionRatio,
		ms.semanticLatency,
		ms.summaryLatency,
		ms.mainModelLatency,
		ms.totalLatency,
		ms.userPromptCompressed,
		ms.responseTokens,
		ms.errorsTotal,
	)

	return ms
}

// RecordChatLog records metrics from a ChatLog entry
func (ms *MetricsService) RecordChatLog(log *model.ChatLog) {
	// Get base labels
	labels := ms.getBaseLabels(log)

	// Add category if available
	category := log.Category
	if category == "" {
		category = "unknown"
	}

	// Record request
	ms.requestsTotal.With(ms.addLabels(labels, "category", category)).Inc()

	// Record original tokens
	ms.originalTokensTotal.With(ms.addLabels(labels, "token_scope", "system")).Add(float64(log.OriginalTokens.SystemTokens))
	ms.originalTokensTotal.With(ms.addLabels(labels, "token_scope", "user")).Add(float64(log.OriginalTokens.UserTokens))
	ms.originalTokensTotal.With(ms.addLabels(labels, "token_scope", "all")).Add(float64(log.OriginalTokens.All))

	// Record compressed tokens
	ms.compressedTokensTotal.With(ms.addLabels(labels, "token_scope", "system")).Add(float64(log.CompressedTokens.SystemTokens))
	ms.compressedTokensTotal.With(ms.addLabels(labels, "token_scope", "user")).Add(float64(log.CompressedTokens.UserTokens))
	ms.compressedTokensTotal.With(ms.addLabels(labels, "token_scope", "all")).Add(float64(log.CompressedTokens.All))

	// Record compression ratio
	if log.CompressionRatio > 0 {
		ms.compressionRatio.With(labels).Observe(log.CompressionRatio)
	}

	// Record latencies
	if log.SemanticLatency > 0 {
		ms.semanticLatency.With(labels).Observe(float64(log.SemanticLatency))
	}

	if log.SummaryLatency > 0 {
		ms.summaryLatency.With(labels).Observe(float64(log.SummaryLatency))
	}

	if log.MainModelLatency > 0 {
		ms.mainModelLatency.With(labels).Observe(float64(log.MainModelLatency))
	}

	if log.TotalLatency > 0 {
		ms.totalLatency.With(labels).Observe(float64(log.TotalLatency))
	}

	if log.IsUserPromptCompressed {
		ms.userPromptCompressed.With(labels).Inc()
	}

	// Record response tokens
	if log.Usage.CompletionTokens > 0 {
		ms.responseTokens.With(labels).Add(float64(log.Usage.CompletionTokens))
	}

	// Record errors
	if log.Error != "" {
		ms.errorsTotal.With(ms.addLabels(labels, "error_type", "processing_error")).Inc()
	}
}

// getBaseLabels creates the base labels map with common fields
func (ms *MetricsService) getBaseLabels(log *model.ChatLog) prometheus.Labels {
	return prometheus.Labels{
		"client_id": log.Identity.ClientID,
		"model":     log.Model,
		"user":      log.Identity.UserName,
	}
}

// addLabels adds additional key-value pairs to existing labels
func (ms *MetricsService) addLabels(baseLabels prometheus.Labels, key, value string) prometheus.Labels {
	newLabels := make(prometheus.Labels)
	for k, v := range baseLabels {
		newLabels[k] = v
	}
	newLabels[key] = value
	return newLabels
}

// GetRegistry returns the Prometheus registry
func (ms *MetricsService) GetRegistry() *prometheus.Registry {
	return prometheus.DefaultRegisterer.(*prometheus.Registry)
}
