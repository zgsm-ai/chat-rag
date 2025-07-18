package functions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ExecutionResult struct {
	ToolName string      `json:"tool_name"`
	Output   interface{} `json:"output"`
	Success  bool        `json:"success"`
	Error    string      `json:"error,omitempty"`
}

type ToolExecutor struct {
	httpClient *http.Client
	authToken  string
}

func NewToolExecutor() *ToolExecutor {
	return &ToolExecutor{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (e *ToolExecutor) Execute(ctx context.Context, tool *Tool, params map[string]interface{}) (*ExecutionResult, error) {
	req, err := e.buildRequest(ctx, tool, params)
	if err != nil {
		return nil, err
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return &ExecutionResult{
			ToolName: tool.Name,
			Success:  false,
			Error:    err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return &ExecutionResult{
			ToolName: tool.Name,
			Success:  false,
			Error:    fmt.Sprintf("failed to decode response: %v", err),
		}, nil
	}

	return &ExecutionResult{
		ToolName: tool.Name,
		Output:   result,
		Success:  resp.StatusCode >= 200 && resp.StatusCode < 300,
	}, nil
}

func (e *ToolExecutor) buildRequest(ctx context.Context, tool *Tool, params map[string]interface{}) (*http.Request, error) {
	var req *http.Request
	var err error

	switch tool.Method {
	case "GET":
		req, err = e.buildGETRequest(ctx, tool, params)
	case "POST":
		req, err = e.buildPOSTRequest(ctx, tool, params)
	default:
		return nil, fmt.Errorf("unsupported method: %s", tool.Method)
	}

	if err != nil {
		return nil, err
	}

	if tool.Auth != nil {
		switch tool.Auth.Type {
		case "bearer":
			req.Header.Add("Authorization", "Bearer "+e.authToken)
		}
	}

	return req, nil
}

func (e *ToolExecutor) buildGETRequest(ctx context.Context, tool *Tool, params map[string]interface{}) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", tool.Endpoint, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	for _, param := range tool.Parameters {
		if val, ok := params[param.Name]; ok && param.In == "query" {
			q.Add(param.Name, fmt.Sprintf("%v", val))
		}
	}
	req.URL.RawQuery = q.Encode()

	return req, nil
}

func (e *ToolExecutor) buildPOSTRequest(ctx context.Context, tool *Tool, params map[string]interface{}) (*http.Request, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tool.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}
