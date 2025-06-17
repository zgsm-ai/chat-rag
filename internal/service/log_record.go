package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/logger"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
	"go.uber.org/zap"
)

const systemClassificationPrompt = `Classify the LAST USER QUESTION in this conversation into ONE of the following EXACT categories based on the user's intention (respond ONLY with the exact category name, no extra text):

- CodeWriting: Writing or generating code to implement functionality
- BugFixing: Fixing errors, bugs, or unexpected behavior in existing code
- CodeUnderstanding: Understanding how code works or asking about programming concepts
- CodeRefactoring: Improving code readability, structure, or maintainability without changing its functionality
- DesignDiscussion: Discussing software design, architecture, or best practices
- DocumentationHelp: Asking about writing or understanding documentation, comments, or code explanations
- EnvironmentHelp: Setting up or troubleshooting the development environment, dependencies, or tools
- ToolUsage: Questions about using development tools, IDEs, debuggers, or plugins
- GeneralQuestion: Any question unrelated to code or development tasks`

const userClassificationPrompt = `
Respond ONLY with one of these exact category names:
- "CodeWriting"
- "BugFixing"
- "CodeUnderstanding"
- "CodeRefactoring"
- "DesignDiscussion"
- "DocumentationHelp"
- "EnvironmentHelp"
- "ToolUsage"
- "GeneralQuestion"

Do not include any extra text, just the exact matching category name.`

// validCategories is a documentation string listing all accepted log categories
const validCategoriesStr = "CodeWriting,BugFixing,CodeUnderstanding,CodeRefactoring,DesignDiscussion,DocumentationHelp,EnvironmentHelp,ToolUsage,GeneralQuestion"

// LogRecordInterface defines the interface for the logger service
type LogRecordInterface interface {
	// Start starts the logger service
	Start() error
	// Stop stops the logger service
	Stop()
	// LogAsync logs a chat completion asynchronously
	LogAsync(logs *model.ChatLog, headers *http.Header)
	// LogSync logs a chat completion synchronously
	SetMetricsService(metricsService MetricsInterface)
}

// LoggerRecordService handles logging operations
type LoggerRecordService struct {
	logFilePath     string // Permanent storage log directory path
	tempLogFilePath string // Temporary log file path
	lokiEndpoint    string
	scanInterval    time.Duration
	metricsService  MetricsInterface
	llmEndpoint     string
	classifyModel   string
	llmClient       client.LLMInterface

	logChan  chan *model.ChatLog
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex

	processorStarted bool
}

// sanitizeFilename cleans a string to make it safe for use in file/folder names
func (ls *LoggerRecordService) sanitizeFilename(name string, defaultName string) string {
	if name == "" {
		return defaultName
	}

	// Remove invalid characters for both Windows and Linux
	invalidChars := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|", "\x00", "\n", "\r", "\t"}
	// Also replace any non-printable ASCII characters
	for i := 0; i < 32; i++ {
		invalidChars = append(invalidChars, string(rune(i)))
	}
	for _, c := range invalidChars {
		name = strings.ReplaceAll(name, c, "")
	}

	// Limit length to 255 bytes for Linux compatibility
	if len(name) > 255 {
		name = name[:255]
	}

	return name
}

// NewLogRecordService creates a new logger service
func NewLogRecordService(config config.Config) LogRecordInterface {
	// Create temp directory under logFilePath for temporary log files
	tempLogDir := filepath.Join(config.LogFilePath, "temp")

	return &LoggerRecordService{
		logFilePath:     config.LogFilePath, // Permanent storage directory
		tempLogFilePath: tempLogDir,         // Temporary logs directory
		lokiEndpoint:    config.LokiEndpoint,
		scanInterval:    time.Duration(config.LogScanIntervalSec) * time.Second,
		llmEndpoint:     config.LLMEndpoint,
		classifyModel:   config.ClassifyModel,
		logChan:         make(chan *model.ChatLog, 1000),
		stopChan:        make(chan struct{}),
	}
}

// SetMetricsService sets the metrics service for the logger
func (ls *LoggerRecordService) SetMetricsService(metricsService MetricsInterface) {
	ls.metricsService = metricsService
}

// Start starts the logger service
func (ls *LoggerRecordService) Start() error {
	logger.Info("==> Start logger")
	// Ensure permanent log directory exists
	if err := os.MkdirAll(ls.logFilePath, 0755); err != nil {
		return fmt.Errorf("failed to create permanent log directory: %w", err)
	}

	// Ensure temp log directory exists
	if err := os.MkdirAll(filepath.Dir(ls.tempLogFilePath), 0755); err != nil {
		return fmt.Errorf("failed to create temp log directory: %w", err)
	}

	// Start log writer goroutine
	ls.wg.Add(1)
	go ls.logWriter()

	return nil
}

// Stop stops the logger service
func (ls *LoggerRecordService) Stop() {
	close(ls.stopChan)
	close(ls.logChan)
	ls.wg.Wait()
}

// LogAsync logs a chat completion asynchronously
func (ls *LoggerRecordService) LogAsync(logs *model.ChatLog, headers *http.Header) {
	llmClient, err := client.NewLLMClient(ls.llmEndpoint, ls.classifyModel, headers)
	if err != nil {
		logger.Error("Failed to create LLM client",
			zap.String("operation", "LogAsync"),
			zap.Error(err),
		)
		return
	}

	ls.llmClient = llmClient
	select {
	case ls.logChan <- logs:
	default:
		// Channel is full, log synchronously to avoid blocking
		ls.logSync(logs)
	}

	if !ls.processorStarted {
		ls.mu.Lock()
		defer ls.mu.Unlock()
		if !ls.processorStarted {
			ls.processorStarted = true
			ls.wg.Add(1)
			go ls.logProcessor()
		}
	}
}

// logWriter writes logs to file
func (ls *LoggerRecordService) logWriter() {
	defer ls.wg.Done()

	for {
		select {
		case log := <-ls.logChan:
			if log != nil {
				ls.logSync(log)
			}
		case <-ls.stopChan:
			// Process remaining logs
			for len(ls.logChan) > 0 {
				log := <-ls.logChan
				if log != nil {
					ls.logSync(log)
				}
			}
			return
		}
	}
}

// writeLogToFile writes log content to specified file path
func (ls *LoggerRecordService) writeLogToFile(filePath string, content string, mode int) error {
	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Open file with specified mode
	file, err := os.OpenFile(filePath, mode, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Convert to raw bytes to avoid any string escaping
	contentBytes := []byte(content)
	contentBytes = append(contentBytes, '\n') // Add newline as raw byte

	// Write content as raw bytes
	if _, err := file.Write(contentBytes); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// generateRandomNumber creates a 6-digit random number from 100000 to 999999
func (ls *LoggerRecordService) generateRandomNumber() int {
	return rand.Intn(900000) + 100000
}

// logSync writes a log entry to temp file synchronously
func (ls *LoggerRecordService) logSync(logs *model.ChatLog) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	// Create timestamped filename
	datePart := logs.Timestamp.Format("20060102")
	timePart := logs.Timestamp.Format("150405")
	username := ls.sanitizeFilename(logs.Identity.UserName, "unknown")
	randNum := ls.generateRandomNumber()
	filename := fmt.Sprintf("%s-%s-%s-%d.log", datePart, timePart, username, randNum)
	filePath := filepath.Join(ls.tempLogFilePath, filename)

	logJSON, err := logs.ToJSON()
	if err != nil {
		logger.Error("Failed to marshal log",
			zap.Error(err),
		)
		return
	}

	if err := ls.writeLogToFile(filePath, logJSON, os.O_CREATE|os.O_WRONLY); err != nil {
		logger.Error("Failed to write temp log",
			zap.Error(err),
		)
	}
}

// logProcessor processes logs periodically
func (ls *LoggerRecordService) logProcessor() {
	logger.Info("==> start logProcessor")
	defer ls.wg.Done()

	ticker := time.NewTicker(ls.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ls.processLogs()
		case <-ls.stopChan:
			// Process logs one last time before stopping
			ls.processLogs()
			return
		}
	}
}

// processLogs reads logs from files one by one, processes each, and uploads to Loki
func (ls *LoggerRecordService) processLogs() {
	// Get list of log files
	files, err := os.ReadDir(ls.tempLogFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		logger.Error("Failed to list log files",
			zap.Error(err),
		)
		return
	}

	if len(files) == 0 {
		return
	}

	// Process each file one by one
	for _, file := range files {
		// Skip files that don't end with .log or .json
		name := file.Name()
		if !strings.HasSuffix(name, ".log") && !strings.HasSuffix(name, ".json") {
			continue
		}
		filePath := filepath.Join(ls.tempLogFilePath, name)
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			logger.Error("Failed to read log file",
				zap.String("filename", file.Name()),
				zap.Error(err),
			)
			continue
		}

		chatLog, err := model.FromJSON(string(fileContent))
		if err != nil {
			logger.Error("Failed to parse log file",
				zap.String("filename", file.Name()),
				zap.Error(err),
			)
			continue
		}

		// Classify log
		if chatLog.Category == "" {
			// Ensure headers are set before classification
			chatLog.Category = ls.classifyLog(chatLog)

			// Update temp log file with category info
			logJSON, err := chatLog.ToJSON()
			if err != nil {
				logger.Error("Failed to marshal updated log",
					zap.Error(err),
				)
				continue
			}
			if err := ls.writeLogToFile(filePath, logJSON, os.O_WRONLY|os.O_TRUNC); err != nil {
				logger.Error("Failed to update temp log file",
					zap.Error(err),
				)
				continue
			}
		}

		// Upload single log to Loki
		if success := ls.uploadToLoki(chatLog); !success {
			logger.Error("Loki upload failed, keeping log file",
				zap.String("filename", file.Name()),
			)
			continue
		}

		logger.Info("Log uploaded to Loki",
			zap.String("filename", file.Name()),
		)

		// Record metrics if metrics service is available
		if ls.metricsService != nil {
			ls.metricsService.RecordChatLog(chatLog)
		}

		// Save to permanent storage
		ls.saveLogToPermanentStorage(chatLog)

		// Delete processed temp file
		ls.deleteTempLogFile(filepath.Join(ls.tempLogFilePath, file.Name()))
	}
}

// classifyLog classifies a single log entry
func (ls *LoggerRecordService) classifyLog(logs *model.ChatLog) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userMessages := utils.GetUserMsgs(logs.CompressedPrompt)
	userMessages = append(userMessages, types.Message{
		Role:    types.RoleUser,
		Content: userClassificationPrompt,
	})

	category, err := ls.llmClient.GenerateContent(ctx, systemClassificationPrompt, userMessages)
	if err != nil {
		logger.Error("Failed to classify log",
			zap.Error(err),
		)
		return "unknown"
	}

	validatedCategory := ls.validateCategory(category)
	logger.Info("Log classification result",
		zap.String("category", validatedCategory),
	)

	return validatedCategory
}

// validateCategory checks if the LLM generated category is valid, returns "extra" if not
func (ls *LoggerRecordService) validateCategory(category string) string {
	valid := strings.Split(validCategoriesStr, ",")
	for _, v := range valid {
		if category == v {
			return category
		}
	}

	logger.Debug("Invalid category detected",
		zap.String("category", category),
	)
	return "extra"
}

// uploadSingleLog uploads a single log to Loki
func (ls *LoggerRecordService) uploadToLoki(chatLog *model.ChatLog) bool {
	lokiStream := model.CreateLokiStream(chatLog)
	lokiBatch := model.LogBatch{Streams: []model.LogStream{*lokiStream}}
	jsonData, err := json.Marshal(lokiBatch)
	if err != nil {
		logger.Error("Failed to marshal Loki data",
			zap.String("operation", "uploadToLoki"),
			zap.Error(err),
		)
		return false
	}

	req, err := http.NewRequest(http.MethodPost, ls.lokiEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("Failed to create Loki request",
			zap.String("operation", "uploadToLoki"),
			zap.Error(err),
		)
		return false
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to upload to Loki",
			zap.String("operation", "uploadToLoki"),
			zap.Error(err),
		)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if err != nil {
			logger.Error("Loki upload failed",
				zap.String("operation", "uploadToLoki"),
				zap.Int("status", resp.StatusCode),
				zap.Error(err),
			)
		} else {
			logger.Error("Loki upload failed",
				zap.String("operation", "uploadToLoki"),
				zap.Int("status", resp.StatusCode),
				zap.String("response", string(body)),
			)
		}
		return false
	}

	return true
}

// saveLogToPermanentStorage saves a single log to permanent storage
func (ls *LoggerRecordService) saveLogToPermanentStorage(chatLog *model.ChatLog) {
	if chatLog == nil {
		logger.Error("Invalid log or missing required identity fields")
		return
	}

	// Directory structure: year-month/day/username
	yearMonth := chatLog.Timestamp.Format("2006-01")
	day := chatLog.Timestamp.Format("02")
	// Get and sanitize username for filesystem usage
	username := ls.sanitizeFilename(chatLog.Identity.UserName, "unknown")

	// Create hierarchical directory path
	dateDir := filepath.Join(ls.logFilePath, yearMonth, day, username)

	// Timestamp for filename: yyyymmdd-HHMMSS_requestID.log
	timestamp := chatLog.Timestamp.Format("20060102-150405")
	requestId := chatLog.Identity.RequestID
	if requestId == "" {
		requestId = "null"
	}
	filename := fmt.Sprintf("%s_%s_%d.log", timestamp, requestId, ls.generateRandomNumber())

	// Full file path
	logFile := filepath.Join(dateDir, filename)

	logJSON, err := chatLog.ToJSON()
	if err != nil {
		logger.Error("Failed to marshal log for permanent storage",
			zap.Error(err),
		)
		return
	}

	// Create new file instead of appending
	if err := ls.writeLogToFile(logFile, logJSON, os.O_CREATE|os.O_WRONLY); err != nil {
		logger.Error("Failed to write log to permanent storage",
			zap.Error(err),
		)
	}
}

// deleteTempLogFile deletes a single temp log file
func (ls *LoggerRecordService) deleteTempLogFile(filePath string) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if err := os.Remove(filePath); err != nil {
		logger.Error("Failed to remove temp log file",
			zap.String("filename", filepath.Base(filePath)),
			zap.Error(err),
		)
	}
}
