package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/types"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

const systemClassificationPrompt = `Classify the LAST USER QUESTION in this conversation into one of these exact categories (respond ONLY with one of these exact names):
- CodeGeneration: Creating new code or projects
- BugFixing: Debugging or fixing issues
- CodeExplanation: Asking questions about code or concepts
- Documentation: Querying documentation or explanations or writing documentation
- OtherQuestions: Asking questions about anything else`

const userClassificationPrompt = `
Respond ONLY with one of these exact category names:
- "CodeGeneration"
- "BugFixing"
- "CodeExplanation"
- "Documentation"
- "OtherQuestions"

Do not include any extra text, just the exact matching category name.`

// validCategories is a documentation string listing all accepted log categories
const validCategoriesStr = "CodeGeneration,BugFixing,CodeExplanation,Documentation,OtherQuestions"

// LoggerService handles logging operations
type LoggerService struct {
	logFilePath     string // Permanent storage log directory path
	tempLogFilePath string // Temporary log file path
	lokiEndpoint    string
	scanInterval    time.Duration
	metricsService  *MetricsService
	llmEndpoint     string
	classifyModel   string
	llmClient       *client.LLMClient

	logChan  chan *model.ChatLog
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex

	processorStarted bool
}

// sanitizeFilename cleans a string to make it safe for use in file/folder names
func (ls *LoggerService) sanitizeFilename(name string, defaultName string) string {
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
		name = strings.ReplaceAll(name, c, "_")
	}

	// Limit length to 255 bytes for Linux compatibility
	if len(name) > 255 {
		name = name[:255]
	}

	return name
}

// NewLoggerService creates a new logger service
func NewLoggerService(config config.Config) *LoggerService {
	// Create temp directory under logFilePath for temporary log files
	tempLogDir := filepath.Join(config.LogFilePath, "temp")

	return &LoggerService{
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
func (ls *LoggerService) SetMetricsService(metricsService *MetricsService) {
	ls.metricsService = metricsService
}

// Start starts the logger service
func (ls *LoggerService) Start() error {
	fmt.Println("==> Starting logger")
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
func (ls *LoggerService) Stop() {
	close(ls.stopChan)
	close(ls.logChan)
	ls.wg.Wait()
}

// LogAsync logs a chat completion asynchronously
func (ls *LoggerService) LogAsync(logs *model.ChatLog, headers *http.Header) {
	llmClient, err := client.NewLLMClient(ls.llmEndpoint, ls.classifyModel, headers)
	if err != nil {
		log.Printf("[LogAsync] Failed to create LLM client: %v\n", err)
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
func (ls *LoggerService) logWriter() {
	fmt.Println("==> start logWriter")
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
func (ls *LoggerService) writeLogToFile(filePath string, content string, mode int) error {
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

// logSync writes a log entry to temp file synchronously
func (ls *LoggerService) logSync(logs *model.ChatLog) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	// Create timestamped filename
	datePart := logs.Timestamp.Format("200601")
	timePart := logs.Timestamp.Format("150405")
	filename := fmt.Sprintf("%s-%s.log", datePart, timePart)
	filePath := filepath.Join(ls.tempLogFilePath, filename)

	logJSON, err := logs.ToJSON()
	if err != nil {
		fmt.Printf("Failed to marshal log: %v\n", err)
		return
	}

	if err := ls.writeLogToFile(filePath, logJSON, os.O_CREATE|os.O_WRONLY); err != nil {
		fmt.Printf("Failed to write temp log: %v\n", err)
	}
}

// logProcessor processes logs periodically
func (ls *LoggerService) logProcessor() {
	fmt.Println("==> start logProcessor")
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
func (ls *LoggerService) processLogs() {
	// Get list of log files
	files, err := os.ReadDir(ls.tempLogFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		fmt.Printf("Failed to list log files: %v\n", err)
		return
	}

	if len(files) == 0 {
		return
	}

	// Process each file one by one
	for _, file := range files {
		filePath := filepath.Join(ls.tempLogFilePath, file.Name())
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Failed to read log file %s: %v\n", file.Name(), err)
			continue
		}

		chatLog, err := model.FromJSON(string(fileContent))
		if err != nil {
			fmt.Printf("Failed to parse log file %s: %v\n", file.Name(), err)
			continue
		}

		// Classify log
		if chatLog.Category == "" {
			// Ensure headers are set before classification
			chatLog.Category = ls.classifyLog(chatLog)

			// Update temp log file with category info
			logJSON, err := chatLog.ToJSON()
			if err != nil {
				fmt.Printf("Failed to marshal updated log: %v\n", err)
				continue
			}
			if err := ls.writeLogToFile(filePath, logJSON, os.O_WRONLY|os.O_TRUNC); err != nil {
				fmt.Printf("Failed to update temp log file: %v\n", err)
				continue
			}
		}

		// Upload single log to Loki
		if success := ls.uploadToLoki(chatLog); !success {
			fmt.Printf("Loki upload failed for file %s, keeping log file\n", file.Name())
			continue
		}

		log.Printf("[processLogs] %s uploaded to loki \n", file.Name())

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
func (ls *LoggerService) classifyLog(logs *model.ChatLog) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userMessages := utils.GetUserMsgs(logs.CompressedPrompt)
	userMessages = append(userMessages, types.Message{
		Role:    types.RoleUser,
		Content: userClassificationPrompt,
	})

	category, err := ls.llmClient.GenerateContent(ctx, systemClassificationPrompt, userMessages)
	if err != nil {
		fmt.Printf("Failed to classify log: %v\n", err)
		return "unknown"
	}

	validatedCategory := ls.validateCategory(category)
	log.Printf("[classifyLog] category: %s \n", validatedCategory)

	return validatedCategory
}

// validateCategory checks if the LLM generated category is valid, returns "extra" if not
func (ls *LoggerService) validateCategory(category string) string {
	valid := strings.Split(validCategoriesStr, ",")
	for _, v := range valid {
		if category == v {
			return category
		}
	}

	log.Printf("[validateCategory] invalid category: %s \n", category)
	return "extra"
}

// uploadSingleLog uploads a single log to Loki
func (ls *LoggerService) uploadToLoki(chatLog *model.ChatLog) bool {
	lokiStream := model.CreateLokiStream(chatLog)
	lokiBatch := model.LogBatch{Streams: []model.LogStream{*lokiStream}}
	jsonData, err := json.Marshal(lokiBatch)
	if err != nil {
		log.Printf("[uploadToLoki] Failed to marshal Loki data: %v\n", err)
		return false
	}

	req, err := http.NewRequest(http.MethodPost, ls.lokiEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[uploadToLoki] Failed to create Loki request: %v\n", err)
		return false
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[uploadToLoki] Failed to upload to Loki: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if err != nil {
			log.Printf("[uploadToLoki] Loki upload failed with status: %d, failed to read response body: %v\n", resp.StatusCode, err)
		} else {
			log.Printf("[uploadToLoki] Loki upload failed with status: %d, response: %q\n", resp.StatusCode, string(body))
		}
		return false
	}

	return true
}

// saveLogToPermanentStorage saves a single log to permanent storage
func (ls *LoggerService) saveLogToPermanentStorage(chatLog *model.ChatLog) {
	if chatLog == nil {
		log.Printf("Invalid log or missing required identity fields\n")
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
	filename := fmt.Sprintf("%s_%s.log", timestamp, requestId)

	// Full file path
	logFile := filepath.Join(dateDir, filename)

	logJSON, err := chatLog.ToJSON()
	if err != nil {
		log.Printf("Failed to marshal log for permanent storage: %v\n", err)
		return
	}

	// Create new file instead of appending
	if err := ls.writeLogToFile(logFile, logJSON, os.O_CREATE|os.O_WRONLY); err != nil {
		log.Printf("Failed to write log to permanent storage: %v\n", err)
	}
}

// deleteTempLogFile deletes a single temp log file
func (ls *LoggerService) deleteTempLogFile(filePath string) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if err := os.Remove(filePath); err != nil {
		fmt.Printf("Failed to remove temp log file %s: %v\n", filepath.Base(filePath), err)
	}
}
