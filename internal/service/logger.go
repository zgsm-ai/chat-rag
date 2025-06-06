package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/utils"
)

const classificationPrompt = `Classify the following chat interaction into one of these exact categories (respond ONLY with one of these exact names):
- CodeGeneration: Creating new code or projects
- BugFixing: Debugging or fixing issues
- CodeExplanation: Asking questions about code or concepts
- Documentation: Querying documentation or explanations or wirtring documentation
- OtherQuestions: Asking questions about anything else

Respond ONLY with one of these exact category names:
- "CodeGeneration"
- "BugFixing"
- "CodeExplanation"
- "Documentation"
- "OtherQuestions"

Do not include any extra text, just the exact matching category name.`

// LoggerService handles logging operations
type LoggerService struct {
	logFilePath     string // Permanent storage log directory path
	tempLogFilePath string // Temporary log file path
	lokiEndpoint    string
	scanInterval    time.Duration
	llmClient       *client.LLMClient
	metricsService  *MetricsService

	logChan  chan *model.ChatLog
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex

	processorStarted bool
}

// NewLoggerService creates a new logger service
func NewLoggerService(logFilePath, lokiEndpoint string, scanIntervalSec int, llmClient *client.LLMClient) *LoggerService {
	// Create temp directory under logFilePath for temporary log files
	tempLogDir := filepath.Join(logFilePath, "temp")

	return &LoggerService{
		logFilePath:     logFilePath, // Permanent storage directory
		tempLogFilePath: tempLogDir,  // Temporary logs directory
		lokiEndpoint:    lokiEndpoint,
		scanInterval:    time.Duration(scanIntervalSec) * time.Second,
		llmClient:       llmClient,
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
	ls.llmClient.SetHeaders(headers)
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

	category, err := ls.llmClient.GenerateContent(ctx, classificationPrompt, utils.GetUserMsgs(logs.CompressedPrompt))
	if err != nil {
		fmt.Printf("Failed to classify log: %v\n", err)
		return "unknown"
	}

	log.Printf("[classifyLog] category: %s \n", category)

	return category
}

// uploadSingleLog uploads a single log to Loki
func (ls *LoggerService) uploadToLoki(chatLog *model.ChatLog) bool {
	lokiStream := model.CreateLokiStream(chatLog)
	lokiBatch := model.LogBatch{Streams: []model.LogStream{*lokiStream}}
	jsonData, err := json.Marshal(lokiBatch)
	if err != nil {
		fmt.Printf("Failed to marshal Loki data: %v\n", err)
		return false
	}

	req, err := http.NewRequest("POST", ls.lokiEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Failed to create Loki request: %v\n", err)
		return false
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Failed to upload to Loki: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		fmt.Printf("Loki upload failed with status: %d\n", resp.StatusCode)
		return false
	}

	return true
}

// saveLogToPermanentStorage saves a single log to permanent storage
func (ls *LoggerService) saveLogToPermanentStorage(log *model.ChatLog) {
	dateStr := log.Timestamp.Format("2006-01-02")
	yearMonth := log.Timestamp.Format("2006-01")

	// Create daily log file path
	dailyLogFile := filepath.Join(ls.logFilePath, yearMonth, fmt.Sprintf("%s.log", dateStr))

	logJSON, err := log.ToJSON()
	if err != nil {
		fmt.Printf("Failed to marshal log for permanent storage: %v\n", err)
		return
	}

	if err := ls.writeLogToFile(dailyLogFile, logJSON, os.O_APPEND|os.O_CREATE|os.O_WRONLY); err != nil {
		fmt.Printf("Failed to write log to permanent storage: %v\n", err)
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
