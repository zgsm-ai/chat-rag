package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"go.uber.org/zap"
)

type ChatMetricsReporter struct {
	ReportUrl string
	Method    string
}

func NewChatMetricsReporter(reportUrl string, method string) *ChatMetricsReporter {
	return &ChatMetricsReporter{
		ReportUrl: reportUrl,
		Method:    method,
	}
}

// Metric 表示单个指标
type Metric struct {
	Name     string  `json:"name"`
	Value    float64 `json:"value,omitempty"`
	StrValue string  `json:"str_value,omitempty"`
}

// Label 表示标签信息
type Label struct {
	TaskID             string `json:"task_id,omitempty"`
	ClientVersion      string `json:"client_version,omitempty"`
	RequestTime        string `json:"request_time,omitempty"`
	ForwardRequestTime string `json:"forward_request_time,omitempty"`
	EndTime            string `json:"end_time,omitempty"`
	Mode               string `json:"mode,omitempty"`
	Model              string `json:"model,omitempty"`
}

// MetricsReport 表示完整的指标上报数据
type MetricsReport struct {
	RequestID       string   `json:"request_id"`
	RequestMetrics  []Metric `json:"request_metrics"`
	ResponseMetrics []Metric `json:"response_metrics"`
	Label           Label    `json:"label"`
}

// ReportMetrics 上报聊天指标,errors 为了防止并发问题,单独处理
func (mr *ChatMetricsReporter) ReportMetrics(chatLog *model.ChatLog, errors ...string) {
	if mr.ReportUrl == "" {
		logger.Debug("metrics report url is empty, skip reporting")
		return
	}

	report := mr.convertChatLogToReport(chatLog, errors...)

	if err := mr.sendReport(report, chatLog.Identity.AuthToken); err != nil {
		logger.Error("failed to report metrics", zap.String("request_id", chatLog.Identity.RequestID), zap.Error(err))
	}
}

// convertChatLogToReport 将 ChatLog 转换为 MetricsReport
func (mr *ChatMetricsReporter) convertChatLogToReport(chatLog *model.ChatLog, errors ...string) *MetricsReport {
	report := &MetricsReport{
		RequestID:       mr.truncateString(chatLog.Identity.RequestID, 32),
		RequestMetrics:  mr.buildRequestMetrics(chatLog),
		ResponseMetrics: mr.buildResponseMetrics(chatLog, errors),
		Label:           mr.buildLabel(chatLog),
	}

	return report
}

// buildRequestMetrics 构建请求指标
func (mr *ChatMetricsReporter) buildRequestMetrics(chatLog *model.ChatLog) []Metric {
	metrics := make([]Metric, 0)

	// 系统提示词长度
	metrics = append(metrics, Metric{
		Name:  "system_tokens",
		Value: float64(chatLog.Tokens.Original.SystemTokens),
	})

	// 用户提示词长度
	metrics = append(metrics, Metric{
		Name:  "user_tokens",
		Value: float64(chatLog.Tokens.Original.UserTokens),
	})

	// 处理后系统提示词长度
	processedSystemTokens := 0
	if chatLog.IsPromptProceed {
		processedSystemTokens = chatLog.Tokens.Processed.SystemTokens
	}
	metrics = append(metrics, Metric{
		Name:  "processed_system_tokens",
		Value: float64(processedSystemTokens),
	})

	// 处理后用户提示词长度
	processedUserTokens := 0
	if chatLog.IsPromptProceed {
		processedUserTokens = chatLog.Tokens.Processed.UserTokens
	}
	metrics = append(metrics, Metric{
		Name:  "processed_user_tokens",
		Value: float64(processedUserTokens),
	})

	// 重试次数
	retryNum := 0 // 当前版本忽略
	// retryNum := 0
	// for _, tool := range chatLog.ToolCalls {
	// 	if tool.ResultStatus == "failed" || tool.Error != "" {
	// 		retryNum++
	// 	}
	// }
	metrics = append(metrics, Metric{
		Name:  "retry_num",
		Value: float64(retryNum),
	})

	return metrics
}

// buildResponseMetrics 构建响应指标
func (mr *ChatMetricsReporter) buildResponseMetrics(chatLog *model.ChatLog, errors []string) []Metric {
	metrics := make([]Metric, 0)

	// 首token时长 (ms)
	if chatLog.Latency.FirstTokenLatency > 0 {
		metrics = append(metrics, Metric{
			Name:  "first_token_duration",
			Value: float64(chatLog.Latency.FirstTokenLatency),
		})
	}

	// 总时长 (ms)
	metrics = append(metrics, Metric{
		Name:  "duration",
		Value: float64(chatLog.Latency.TotalLatency),
	})

	// 总提示词长度
	if chatLog.Usage.PromptTokens > 0 {
		metrics = append(metrics, Metric{
			Name:  "prompt_tokens",
			Value: float64(chatLog.Usage.PromptTokens),
		})
	}

	// 输出长度
	if chatLog.Usage.CompletionTokens > 0 {
		metrics = append(metrics, Metric{
			Name:  "completion_tokens",
			Value: float64(chatLog.Usage.CompletionTokens),
		})
	}

	// 错误类型
	if len(errors) > 0 {
		metrics = append(metrics, Metric{
			Name:     "error_code",
			StrValue: errors[0], // 只取第一个错误类型
		})
	}

	// 最耗时的chunk
	// 忽略
	// slowestChunk := int64(0)
	// for _, tool := range chatLog.ToolCalls {
	// 	if tool.Latency > slowestChunk {
	// 		slowestChunk = tool.Latency
	// 	}
	// }
	// if slowestChunk > 0 {
	// 	metrics = append(metrics, Metric{
	// 		Name:  "slow_chunk",
	// 		Value: float64(slowestChunk),
	// 	})
	// }

	return metrics
}

// buildLabel 构建标签
func (mr *ChatMetricsReporter) buildLabel(chatLog *model.ChatLog) Label {
	label := Label{
		TaskID:        mr.truncateString(chatLog.Identity.TaskID, 32),
		ClientVersion: mr.truncateString(chatLog.Identity.ClientVersion, 16),
		Model:         mr.truncateString(chatLog.Params.Model, 16),
	}

	// 请求时间 - 使用chatLog的时间戳
	if !chatLog.Timestamp.IsZero() {
		label.RequestTime = mr.truncateString(chatLog.Timestamp.Format(time.RFC3339), 32)
	}

	// 转发时间 - 如果有首token延迟，可以计算转发时间
	if chatLog.Latency.FirstTokenLatency > 0 {
		forwardTime := chatLog.Timestamp.Add(time.Duration(chatLog.Latency.FirstTokenLatency) * time.Millisecond)
		label.ForwardRequestTime = mr.truncateString(forwardTime.Format(time.RFC3339), 32)
	}

	// 结束时间 - 使用总延迟计算
	if chatLog.Latency.TotalLatency > 0 {
		endTime := chatLog.Timestamp.Add(time.Duration(chatLog.Latency.TotalLatency) * time.Millisecond)
		label.EndTime = mr.truncateString(endTime.Format(time.RFC3339), 32)
	}

	// 模式 - 从请求参数中提取
	if chatLog.Params.LlmParams.ExtraBody.Mode != "" {
		label.Mode = mr.truncateString(chatLog.Params.LlmParams.ExtraBody.Mode, 16)
	}

	return label
}

// sendReport 发送指标报告
func (mr *ChatMetricsReporter) sendReport(report *MetricsReport, authToken string) error {
	jsonData, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	req, err := http.NewRequest(mr.Method, mr.ReportUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// 添加认证头
	if authToken != "" {
		req.Header.Set("Authorization", authToken)
	}

	client := &http.Client{
		Timeout: 10 * time.Second, // 设置请求超时时间,防止大量阻塞
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	logger.Debug("metrics reported successfully",
		zap.String("request_id", report.RequestID),
	)

	return nil
}

// truncateString 截断字符串到指定长度
func (mr *ChatMetricsReporter) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
