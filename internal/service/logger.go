package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

// LoggerService handles logging operations
type LoggerService struct {
	logFilePath     string // Permanent storage log directory path
	tempLogFilePath string // Temporary log file path
	lokiEndpoint    string
	batchSize       int
	scanInterval    time.Duration
	llmClient       *client.LLMClient

	logChan  chan *model.ChatLog
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

// NewLoggerService creates a new logger service
func NewLoggerService(logFilePath, lokiEndpoint string, batchSize int, scanIntervalSec int, llmClient *client.LLMClient) *LoggerService {
	// Create temp directory under logFilePath for temporary log files
	tempLogFilePath := filepath.Join(logFilePath, "temp", "chat-rag-temp.log")

	return &LoggerService{
		logFilePath:     logFilePath,     // Permanent storage directory
		tempLogFilePath: tempLogFilePath, // Temporary log file
		lokiEndpoint:    lokiEndpoint,
		batchSize:       batchSize,
		scanInterval:    time.Duration(scanIntervalSec) * time.Second,
		llmClient:       llmClient,
		logChan:         make(chan *model.ChatLog, 1000),
		stopChan:        make(chan struct{}),
	}
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

	// Start log processor goroutine
	ls.wg.Add(1)
	go ls.logProcessor()

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

// logSync writes a log entry to temp file synchronously
func (ls *LoggerService) logSync(logs *model.ChatLog) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	// Write to temp file
	file, err := os.OpenFile(ls.tempLogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to open temp log file: %v\n", err)
		return
	}
	defer file.Close()

	logJSON, err := logs.ToJSON()
	if err != nil {
		fmt.Printf("Failed to marshal log: %v\n", err)
		return
	}

	if _, err := file.WriteString(logJSON + "\n"); err != nil {
		fmt.Printf("Failed to write log: %v\n", err)
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

// processLogs reads logs from file, classifies them, and uploads to Loki
func (ls *LoggerService) processLogs() {
	fmt.Println("==> processLogs")
	// Read logs from temp file
	logs, err := ls.readLogsFromFile()
	if err != nil {
		fmt.Printf("Failed to read logs: %v\n", err)
		return
	}

	if len(logs) == 0 {
		return
	}

	// Classify logs
	ls.classifyLogs(logs)

	// Upload to Loki in batches
	success := ls.uploadToLoki(logs)
	if !success {
		fmt.Println("Loki upload failed, keeping logs in temp file for retry")
		return
	}

	// Save logs to permanent storage before clearing temp file
	ls.saveToPermanentStorage(logs)

	// Clear processed logs from temp file
	ls.clearLogFile()
}

// readLogsFromFile reads all logs from the temp log file
func (ls *LoggerService) readLogsFromFile() ([]*model.ChatLog, error) {
	fmt.Println("==> readLogsFromFile")
	file, err := os.Open(ls.tempLogFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*model.ChatLog{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var logs []*model.ChatLog
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		log, err := model.FromJSON(line)
		if err != nil {
			fmt.Printf("Failed to parse log line: %v\n", err)
			continue
		}

		logs = append(logs, log)
	}

	return logs, scanner.Err()
}

// classifyLogs classifies logs using LLM
func (ls *LoggerService) classifyLogs(logs []*model.ChatLog) {
	fmt.Println("==> classifyLogs")
	for _, log := range logs {
		if log.Category != "" {
			continue // Already classified
		}

		category := ls.classifyLog(log)
		log.Category = category
	}
}

// classifyLog classifies a single log entry
func (ls *LoggerService) classifyLog(logs *model.ChatLog) string {
	prompt := fmt.Sprintf(`Classify the following chat interaction into one of these categories:
- Code Generation: Creating new code or projects
- Bug Fixing: Debugging or fixing issues
- Code Explanation: Asking questions about code or concepts
- Documentation: Querying documentation or explanations

Context:
- Original Prompt Sample: %s

Please respond with only the category name.`,
		logs.OriginalPromptSample)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var classifyMessages []types.Message
	classifyMessages = append(classifyMessages, types.Message{
		Role:    "system",
		Content: prompt,
	})

	category, err := ls.llmClient.GenerateContent(ctx, "", classifyMessages)
	if err != nil {
		fmt.Printf("Failed to classify log: %v\n", err)
		return "unknown"
	}

	log.Printf("==> [classifyLog] category: %s \n", category)

	// Clean up the response
	category = cleanCategory(category)
	return category
}

// uploadToLoki uploads logs to Loki in batches
func (ls *LoggerService) uploadToLoki(logs []*model.ChatLog) bool {
	fmt.Println("==> uploadToLoki")
	for i := 0; i < len(logs); i += ls.batchSize {
		end := i + ls.batchSize
		if end > len(logs) {
			end = len(logs)
		}

		batch := logs[i:end]
		if !ls.uploadBatch(batch) {
			return false // Return failure if any batch fails
		}
	}
	return true // All batches succeeded
}

// uploadBatch uploads a single batch to Loki
func (ls *LoggerService) uploadBatch(logs []*model.ChatLog) bool {
	lokiBatch := model.CreateLokiBatch(logs)

	jsonData, err := json.Marshal(lokiBatch)
	if err != nil {
		fmt.Printf("Failed to marshal Loki batch: %v\n", err)
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

	return true // Upload succeeded
}

// saveToPermanentStorage saves logs to permanent storage with date-based directory structure
func (ls *LoggerService) saveToPermanentStorage(logs []*model.ChatLog) {
	fmt.Println("==> saveToPermanentStorage")
	if len(logs) == 0 {
		return
	}

	now := time.Now()
	yearMonth := now.Format("2006-01")
	dateStr := now.Format("2006-01-02")

	// Create year-month subdirectory
	yearMonthDir := filepath.Join(ls.logFilePath, yearMonth)
	if err := os.MkdirAll(yearMonthDir, 0755); err != nil {
		fmt.Printf("Failed to create year-month directory: %v\n", err)
		return
	}

	// Create daily log file path
	dailyLogFile := filepath.Join(yearMonthDir, fmt.Sprintf("%s.log", dateStr))

	// 追加写入日志到当天的文件
	file, err := os.OpenFile(dailyLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to open permanent log file: %v\n", err)
		return
	}
	defer file.Close()

	for _, log := range logs {
		logJSON, err := log.ToJSON()
		if err != nil {
			fmt.Printf("Failed to marshal log for permanent storage: %v\n", err)
			continue
		}

		if _, err := file.WriteString(logJSON + "\n"); err != nil {
			fmt.Printf("Failed to write log to permanent storage: %v\n", err)
		}
	}

	fmt.Printf("Saved %d logs to permanent storage: %s\n", len(logs), dailyLogFile)
}

// clearLogFile clears the temp log file after processing
func (ls *LoggerService) clearLogFile() {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if err := os.Truncate(ls.tempLogFilePath, 0); err != nil {
		fmt.Printf("Failed to clear temp log file: %v\n", err)
	}
}

// Helper function to clean category response
func cleanCategory(category string) string {
	category = strings.ToLower(strings.TrimSpace(category))

	validCategories := map[string]bool{
		"code_generation": true,
		"bug_fixing":      true,
		"exploration":     true,
		"documentation":   true,
		"optimization":    true,
	}

	if validCategories[category] {
		return category
	}

	return "unknown"
}
