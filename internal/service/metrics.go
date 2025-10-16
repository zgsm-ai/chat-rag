package service

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"go.uber.org/zap"
)

const (
	// Base labels
	metricsBaseLabelClientID  = "client_id"
	metricsBaseLabelClientIDE = "client_ide"
	metricsBaseLabelModel     = "model"
	metricsBaseLabelUser      = "user"
	metricsBaseLabelLoginFrom = "login_from"
	metricsBaseLabelCaller    = "caller"
	metricsBaseLabelSender    = "sender"
	metricsBaseLabelDept1     = "dept_level1"
	metricsBaseLabelDept2     = "dept_level2"
	metricsBaseLabelDept3     = "dept_level3"
	metricsBaseLabelDept4     = "dept_level4"

	// Label names
	metricsLabelCategory   = "category"
	metricsLabelTokenScope = "token_scope"
	metricsLabelErrorType  = "error_type"

	// Metric names
	metricRequestsTotal         = "chat_rag_requests_total"
	metricOriginalTokensTotal   = "chat_rag_original_tokens_total"
	metricCompressedTokensTotal = "chat_rag_compressed_tokens_total"
	metricCompressionRatio      = "chat_rag_compression_ratio"
	metricSemanticLatency       = "chat_rag_semantic_latency_ms"
	metricSummaryLatency        = "chat_rag_summary_latency_ms"
	metricMainModelLatency      = "chat_rag_main_model_latency_ms"
	metricTotalLatency          = "chat_rag_total_latency_ms"
	metricUserPromptCompressed  = "chat_rag_user_prompt_compressed_total"
	metricResponseTokens        = "chat_rag_response_tokens_total"
	metricErrorsTotal           = "chat_rag_errors_total"

	// Default values
	defaultCategory = "unknown"
)

// Bucket definitions
var (
	compressionRatioBuckets = []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}
	fastLatencyBuckets      = []float64{10, 50, 100, 200, 500, 1000, 2000, 5000}
	mainModelLatencyBuckets = []float64{100, 500, 1000, 2000, 5000, 10000, 20000}
	totalLatencyBuckets     = []float64{100, 500, 1000, 2000, 5000, 10000, 20000, 30000}
)

// Base label list
var metricsBaseLabels = []string{
	metricsBaseLabelClientID,
	metricsBaseLabelClientIDE,
	metricsBaseLabelModel,
	metricsBaseLabelUser,
	metricsBaseLabelLoginFrom,
	metricsBaseLabelCaller,
	metricsBaseLabelSender,
	metricsBaseLabelDept1,
	metricsBaseLabelDept2,
	metricsBaseLabelDept3,
	metricsBaseLabelDept4,
}

// MetricsInterface defines the interface for metrics service
type MetricsInterface interface {
	RecordChatLog(log *model.ChatLog)
	GetRegistry() *prometheus.Registry
}

// MetricsService handles Prometheus metrics collection
type MetricsService struct {
	requestsTotal         *prometheus.CounterVec
	originalTokensTotal   *prometheus.CounterVec
	compressedTokensTotal *prometheus.CounterVec
	compressionRatio      *prometheus.HistogramVec
	semanticLatency       *prometheus.HistogramVec
	summaryLatency        *prometheus.HistogramVec
	mainModelLatency      *prometheus.HistogramVec
	totalLatency          *prometheus.HistogramVec
	userPromptCompressed  *prometheus.CounterVec
	responseTokens        *prometheus.CounterVec
	errorsTotal           *prometheus.CounterVec
}

// NewMetricsService creates a new metrics service
func NewMetricsService() MetricsInterface {
	ms := &MetricsService{}

	ms.requestsTotal = ms.createCounterVec(metricRequestsTotal, "Total number of chat completion requests", metricsLabelCategory)
	ms.originalTokensTotal = ms.createCounterVec(metricOriginalTokensTotal, "Total number of original tokens processed", metricsLabelTokenScope)
	ms.compressedTokensTotal = ms.createCounterVec(metricCompressedTokensTotal, "Total number of compressed tokens processed", metricsLabelTokenScope)
	ms.compressionRatio = ms.createHistogramVec(metricCompressionRatio, "Distribution of compression ratios", nil, compressionRatioBuckets)
	ms.semanticLatency = ms.createHistogramVec(metricSemanticLatency, "Semantic processing latency in milliseconds", nil, fastLatencyBuckets)
	ms.summaryLatency = ms.createHistogramVec(metricSummaryLatency, "Summary processing latency in milliseconds", nil, fastLatencyBuckets)
	ms.mainModelLatency = ms.createHistogramVec(metricMainModelLatency, "Main model processing latency in milliseconds", nil, mainModelLatencyBuckets)
	ms.totalLatency = ms.createHistogramVec(metricTotalLatency, "Total processing latency in milliseconds", nil, totalLatencyBuckets)
	ms.userPromptCompressed = ms.createCounterVec(metricUserPromptCompressed, "Total number of requests where user prompt was compressed")
	ms.responseTokens = ms.createCounterVec(metricResponseTokens, "Total number of response tokens generated")
	ms.errorsTotal = ms.createCounterVec(metricErrorsTotal, "Total number of errors encountered", metricsLabelErrorType)

	ms.registerMetrics()
	return ms
}

// createCounterVec creates a CounterVec with base labels
func (ms *MetricsService) createCounterVec(name, help string, extraLabels ...string) *prometheus.CounterVec {
	labels := metricsBaseLabels
	if len(extraLabels) > 0 {
		labels = append(labels, extraLabels...)
	}
	return prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: name,
			Help: help,
		},
		labels,
	)
}

// createHistogramVec creates a HistogramVec with base labels
func (ms *MetricsService) createHistogramVec(name, help string, extraLabels []string, buckets []float64) *prometheus.HistogramVec {
	labels := metricsBaseLabels
	if extraLabels != nil {
		labels = append(labels, extraLabels...)
	}
	return prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    name,
			Help:    help,
			Buckets: buckets,
		},
		labels,
	)
}

// registerMetrics registers all metrics
func (ms *MetricsService) registerMetrics() {
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
}

// RecordChatLog records metrics from a ChatLog entry
func (ms *MetricsService) RecordChatLog(log *model.ChatLog) {
	if log == nil {
		return
	}

	labels := ms.getBaseLabels(log)
	ms.recordRequestMetrics(log, labels)
	ms.recordTokenMetrics(log, labels)
	ms.recordLatencyMetrics(log, labels)
	ms.recordCompressionMetrics(log, labels)
	ms.recordResponseMetrics(log, labels)
	ms.recordErrorMetrics(log, labels)
}

// recordRequestMetrics records request related metrics
func (ms *MetricsService) recordRequestMetrics(log *model.ChatLog, labels prometheus.Labels) {
	category := log.Category
	if category == "" {
		category = defaultCategory
	}
	ms.requestsTotal.With(ms.addLabel(labels, metricsLabelCategory, category)).Inc()
}

// recordTokenMetrics records token related metrics
func (ms *MetricsService) recordTokenMetrics(log *model.ChatLog, labels prometheus.Labels) {
	// Record original tokens
	ms.recordTokenCount(ms.originalTokensTotal, log.OriginalTokens, labels)

	// Record compressed tokens
	ms.recordTokenCount(ms.compressedTokensTotal, log.ProcessedTokens, labels)
}

// recordTokenCount records token count
func (ms *MetricsService) recordTokenCount(metric *prometheus.CounterVec, tokens model.TokenStats, labels prometheus.Labels) {
	record := func(scope string, count int) {
		if count < 0 {
			logger.Warn("WARNING: negative token count",
				zap.String("scope", scope),
				zap.Int("count", count),
			)
			return
		}

		if count == 0 {
			return
		}

		metric.With(ms.addLabel(labels, metricsLabelTokenScope, scope)).Add(float64(count))
	}

	record("system", tokens.SystemTokens)
	record("user", tokens.UserTokens)
	record("all", tokens.All)
}

// recordLatencyMetrics records latency related metrics
func (ms *MetricsService) recordLatencyMetrics(log *model.ChatLog, labels prometheus.Labels) {
	if log.MainModelLatency > 0 {
		ms.mainModelLatency.With(labels).Observe(float64(log.MainModelLatency))
	}
	if log.TotalLatency > 0 {
		ms.totalLatency.With(labels).Observe(float64(log.TotalLatency))
	}
}

// recordCompressionMetrics records compression related metrics
func (ms *MetricsService) recordCompressionMetrics(log *model.ChatLog, labels prometheus.Labels) {
	if log.IsUserPromptCompressed {
		ms.userPromptCompressed.With(labels).Inc()
	}
}

// recordResponseMetrics records response related metrics
func (ms *MetricsService) recordResponseMetrics(log *model.ChatLog, labels prometheus.Labels) {
	if log.Usage.CompletionTokens > 0 {
		ms.responseTokens.With(labels).Add(float64(log.Usage.CompletionTokens))
	}
}

// recordErrorMetrics records error related metrics
func (ms *MetricsService) recordErrorMetrics(log *model.ChatLog, labels prometheus.Labels) {
	for _, errorMap := range log.Error {
		for errorType, errorMessage := range errorMap {
			if errorMessage != "" {
				ms.errorsTotal.With(ms.addLabel(labels, metricsLabelErrorType, string(errorType))).Inc()
			}
		}
	}
}

// getBaseLabels creates base labels map
func (ms *MetricsService) getBaseLabels(log *model.ChatLog) prometheus.Labels {
	labels := prometheus.Labels{
		metricsBaseLabelClientID:  log.Identity.ClientID,
		metricsBaseLabelClientIDE: log.Identity.ClientIDE,
		metricsBaseLabelModel:     log.Model,
		metricsBaseLabelUser:      log.Identity.UserName,
		metricsBaseLabelLoginFrom: log.Identity.LoginFrom,
		metricsBaseLabelCaller:    log.Identity.Caller,
		metricsBaseLabelSender:    log.Identity.Sender,
	}

	if log.Identity.UserInfo != nil &&
		log.Identity.UserInfo.Department != nil &&
		log.Identity.UserInfo.EmployeeNumber != "" {
		labels[metricsBaseLabelDept1] = log.Identity.UserInfo.Department.Level1Dept
		labels[metricsBaseLabelDept2] = log.Identity.UserInfo.Department.Level2Dept
		labels[metricsBaseLabelDept3] = log.Identity.UserInfo.Department.Level3Dept
		labels[metricsBaseLabelDept4] = log.Identity.UserInfo.Department.Level4Dept
	} else {
		labels[metricsBaseLabelDept1] = ""
		labels[metricsBaseLabelDept2] = ""
		labels[metricsBaseLabelDept3] = ""
		labels[metricsBaseLabelDept4] = ""
	}

	return labels
}

// addLabel adds a new label to existing labels
func (ms *MetricsService) addLabel(baseLabels prometheus.Labels, key, value string) prometheus.Labels {
	// Copy original labels
	newLabels := make(prometheus.Labels, len(baseLabels)+1)
	for k, v := range baseLabels {
		newLabels[k] = v
	}
	// Add new label
	newLabels[key] = value
	return newLabels
}

// GetRegistry returns the Prometheus registry
func (ms *MetricsService) GetRegistry() *prometheus.Registry {
	return prometheus.DefaultRegisterer.(*prometheus.Registry)
}
