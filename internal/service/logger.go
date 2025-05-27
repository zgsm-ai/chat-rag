package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/client"
	"github.com/zgsm-ai/chat-rag/internal/model"
)

// LoggerService handles logging operations
type LoggerService struct {
	logFilePath  string
	lokiEndpoint string
	batchSize    int
	scanInterval time.Duration
	llmClient    *client.LLMClient

	logChan  chan *model.ChatLog
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

// NewLoggerService creates a new logger service
func NewLoggerService(logFilePath, lokiEndpoint string, batchSize int, scanIntervalSec int, llmClient *client.LLMClient) *LoggerService {
	return &LoggerService{
		logFilePath:  logFilePath,
		lokiEndpoint: lokiEndpoint,
		batchSize:    batchSize,
		scanInterval: time.Duration(scanIntervalSec) * time.Second,
		llmClient:    llmClient,
		logChan:      make(chan *model.ChatLog, 1000),
		stopChan:     make(chan struct{}),
	}
}

// Start starts the logger service
func (ls *LoggerService) Start() {
	// Ensure log directory exists
	if err := os.MkdirAll(filepath.Dir(ls.logFilePath), 0755); err != nil {
		fmt.Printf("Failed to create log directory: %v\n", err)
		return
	}

	// Start log writer goroutine
	ls.wg.Add(1)
	go ls.logWriter()

	// Start log processor goroutine
	ls.wg.Add(1)
	go ls.logProcessor()
}

// Stop stops the logger service
func (ls *LoggerService) Stop() {
	close(ls.stopChan)
	close(ls.logChan)
	ls.wg.Wait()
}

// LogAsync logs a chat completion asynchronously
func (ls *LoggerService) LogAsync(log *model.ChatLog) {
	select {
	case ls.logChan <- log:
	default:
		// Channel is full, log synchronously to avoid blocking
		ls.logSync(log)
	}
}

// logWriter writes logs to file
func (ls *LoggerService) logWriter() {
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

// logSync writes a log entry to file synchronously
func (ls *LoggerService) logSync(log *model.ChatLog) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	file, err := os.OpenFile(ls.logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		return
	}
	defer file.Close()

	logJSON, err := log.ToJSON()
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
	// Read logs from file
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
	ls.uploadToLoki(logs)

	// Clear processed logs from file
	ls.clearLogFile()
}

// readLogsFromFile reads all logs from the log file
func (ls *LoggerService) readLogsFromFile() ([]*model.ChatLog, error) {
	file, err := os.Open(ls.logFilePath)
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
	for _, log := range logs {
		if log.Category != "" {
			continue // Already classified
		}

		category := ls.classifyLog(log)
		log.Category = category
	}
}

// classifyLog classifies a single log entry
func (ls *LoggerService) classifyLog(log *model.ChatLog) string {
	prompt := fmt.Sprintf(`Classify the following chat interaction into one of these categories:
- code_generation: Creating new code or projects
- bug_fixing: Debugging or fixing issues
- exploration: Asking questions about code or concepts
- documentation: Querying documentation or explanations
- optimization: Improving performance or code quality

Context:
- Client ID: %s
- Project Path: %s
- Original Prompt Sample: %s
- Compressed: %t

Please respond with only the category name.`,
		log.ClientID, log.ProjectPath, log.OriginalPromptSample, log.IsCompressed)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	category, err := ls.llmClient.SummarizeContent(ctx, prompt)
	if err != nil {
		fmt.Printf("Failed to classify log: %v\n", err)
		return "unknown"
	}

	// Clean up the response
	category = cleanCategory(category)
	return category
}

// uploadToLoki uploads logs to Loki in batches
func (ls *LoggerService) uploadToLoki(logs []*model.ChatLog) {
	for i := 0; i < len(logs); i += ls.batchSize {
		end := i + ls.batchSize
		if end > len(logs) {
			end = len(logs)
		}

		batch := logs[i:end]
		ls.uploadBatch(batch)
	}
}

// uploadBatch uploads a single batch to Loki
func (ls *LoggerService) uploadBatch(logs []*model.ChatLog) {
	lokiBatch := model.CreateLokiBatch(logs)

	jsonData, err := json.Marshal(lokiBatch)
	if err != nil {
		fmt.Printf("Failed to marshal Loki batch: %v\n", err)
		return
	}

	req, err := http.NewRequest("POST", ls.lokiEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Failed to create Loki request: %v\n", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Failed to upload to Loki: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		fmt.Printf("Loki upload failed with status: %d\n", resp.StatusCode)
	}
}

// clearLogFile clears the log file after processing
func (ls *LoggerService) clearLogFile() {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if err := os.Truncate(ls.logFilePath, 0); err != nil {
		fmt.Printf("Failed to clear log file: %v\n", err)
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
