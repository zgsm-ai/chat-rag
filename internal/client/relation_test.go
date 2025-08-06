package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewRelationClient(t *testing.T) {
	endpoint := "http://localhost:8002/v1/relation"
	clientInterface := NewRelationClient(endpoint)

	if clientInterface == nil {
		t.Fatal("NewRelationClient returned nil")
	}

	// Type assertion to access concrete implementation
	client, ok := clientInterface.(*RelationClient)
	if !ok {
		t.Fatal("NewRelationClient did not return *RelationClient")
	}

	if client.httpClient == nil {
		t.Fatal("HTTP client is nil")
	}
}

func TestRelationClient_Search_Success(t *testing.T) {
	// Mock response data
	mockResponse := RelationData{
		Results: []RelationNode{
			{
				Content:  "func main() { ... }",
				NodeType: "definition",
				FilePath: "src/main.go",
				Position: Position{
					StartLine:   1,
					StartColumn: 1,
					EndLine:     5,
					EndColumn:   2,
				},
				Children: []RelationNode{
					{
						Content:  "import \"fmt\"",
						NodeType: "reference",
						FilePath: "src/utils.go",
						Position: Position{
							StartLine:   1,
							StartColumn: 1,
							EndLine:     1,
							EndColumn:   14,
						},
						Children: []RelationNode{},
					},
				},
			},
		},
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != "GET" {
			t.Errorf("Expected GET method, got %s", r.Method)
		}

		// Verify authorization header
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected Authorization header 'Bearer test-token', got '%s'", r.Header.Get("Authorization"))
		}

		// Verify query parameters
		if r.URL.Query().Get("clientId") != "test-client" {
			t.Errorf("Expected clientId 'test-client', got '%s'", r.URL.Query().Get("clientId"))
		}

		if r.URL.Query().Get("codebasePath") != "/test/project" {
			t.Errorf("Expected codebasePath '/test/project', got '%s'", r.URL.Query().Get("codebasePath"))
		}

		if r.URL.Query().Get("filePath") != "src/main.go" {
			t.Errorf("Expected filePath 'src/main.go', got '%s'", r.URL.Query().Get("filePath"))
		}

		if r.URL.Query().Get("startLine") != "1" {
			t.Errorf("Expected startLine '1', got '%s'", r.URL.Query().Get("startLine"))
		}

		if r.URL.Query().Get("startColumn") != "1" {
			t.Errorf("Expected startColumn '1', got '%s'", r.URL.Query().Get("startColumn"))
		}

		if r.URL.Query().Get("endLine") != "5" {
			t.Errorf("Expected endLine '5', got '%s'", r.URL.Query().Get("endLine"))
		}

		if r.URL.Query().Get("endColumn") != "2" {
			t.Errorf("Expected endColumn '2', got '%s'", r.URL.Query().Get("endColumn"))
		}

		// Send mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	// Create client with mock server URL
	client := NewRelationClient(server.URL)

	// Test request
	req := RelationRequest{
		ClientId:       "test-client",
		CodebasePath:   "/test/project",
		FilePath:       "src/main.go",
		StartLine:      1,
		StartColumn:    1,
		EndLine:        5,
		EndColumn:      2,
		SymbolName:     "main",
		IncludeContent: 1,
		MaxLayer:       3,
		Authorization:  "Bearer test-token",
	}

	ctx := context.Background()
	resp, err := client.Search(ctx, req)

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Response is nil")
	}

	if len(resp.Results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(resp.Results))
	}

	// Verify first result
	if resp.Results[0].Content != "func main() { ... }" {
		t.Errorf("Expected content 'func main() { ... }', got '%s'", resp.Results[0].Content)
	}

	if resp.Results[0].NodeType != "definition" {
		t.Errorf("Expected nodeType 'definition', got '%s'", resp.Results[0].NodeType)
	}

	if resp.Results[0].FilePath != "src/main.go" {
		t.Errorf("Expected filePath 'src/main.go', got '%s'", resp.Results[0].FilePath)
	}

	if resp.Results[0].Position.StartLine != 1 {
		t.Errorf("Expected startLine 1, got %d", resp.Results[0].Position.StartLine)
	}

	if len(resp.Results[0].Children) != 1 {
		t.Errorf("Expected 1 child, got %d", len(resp.Results[0].Children))
	}
}

func TestRelationClient_Search_EmptyResults(t *testing.T) {
	// Mock empty response
	mockResponse := RelationData{
		Results: []RelationNode{},
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := NewRelationClient(server.URL)

	req := RelationRequest{
		ClientId:      "test-client",
		CodebasePath:  "/test/project",
		FilePath:      "src/empty.go",
		StartLine:     1,
		StartColumn:   1,
		EndLine:       1,
		EndColumn:     1,
		Authorization: "Bearer test-token",
	}

	ctx := context.Background()
	resp, err := client.Search(ctx, req)

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Response is nil")
	}

	if len(resp.Results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(resp.Results))
	}
}

func TestRelationClient_Search_HTTPError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewRelationClient(server.URL)

	req := RelationRequest{
		ClientId:      "test-client",
		CodebasePath:  "/test/project",
		FilePath:      "src/main.go",
		StartLine:     1,
		StartColumn:   1,
		EndLine:       5,
		EndColumn:     2,
		Authorization: "Bearer test-token",
	}

	ctx := context.Background()
	_, err := client.Search(ctx, req)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	expectedError := "relation search failed! status: 500"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestRelationClient_Search_InvalidJSON(t *testing.T) {
	// Create mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewRelationClient(server.URL)

	req := RelationRequest{
		ClientId:      "test-client",
		CodebasePath:  "/test/project",
		FilePath:      "src/main.go",
		StartLine:     1,
		StartColumn:   1,
		EndLine:       5,
		EndColumn:     2,
		Authorization: "Bearer test-token",
	}

	ctx := context.Background()
	_, err := client.Search(ctx, req)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	expectedError := "failed to decode response"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestRelationClient_Search_ContextCancellation(t *testing.T) {
	// Create mock server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(RelationData{Results: []RelationNode{}})
	}))
	defer server.Close()

	client := NewRelationClient(server.URL)

	req := RelationRequest{
		ClientId:      "test-client",
		CodebasePath:  "/test/project",
		FilePath:      "src/main.go",
		StartLine:     1,
		StartColumn:   1,
		EndLine:       5,
		EndColumn:     2,
		Authorization: "Bearer test-token",
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Search(ctx, req)

	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}

	expectedError := "failed to execute request"
	if !strings.HasPrefix(err.Error(), expectedError) {
		t.Errorf("Expected error to start with '%s', got '%s'", expectedError, err.Error())
	}
}

func TestRelationClient_Search_InvalidURL(t *testing.T) {
	// Create client with invalid URL
	client := NewRelationClient("invalid-url")

	req := RelationRequest{
		ClientId:      "test-client",
		CodebasePath:  "/test/project",
		FilePath:      "src/main.go",
		StartLine:     1,
		StartColumn:   1,
		EndLine:       5,
		EndColumn:     2,
		Authorization: "Bearer test-token",
	}

	ctx := context.Background()
	_, err := client.Search(ctx, req)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	expectedError := "failed to parse endpoint URL"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestRelationRequest_QueryParameters(t *testing.T) {
	// Create mock server to verify query parameters
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		// Verify required parameters
		if query.Get("clientId") != "test-client" {
			t.Errorf("Expected clientId 'test-client', got '%s'", query.Get("clientId"))
		}

		if query.Get("codebasePath") != "/test/project" {
			t.Errorf("Expected codebasePath '/test/project', got '%s'", query.Get("codebasePath"))
		}

		if query.Get("filePath") != "src/main.go" {
			t.Errorf("Expected filePath 'src/main.go', got '%s'", query.Get("filePath"))
		}

		if query.Get("startLine") != "10" {
			t.Errorf("Expected startLine '10', got '%s'", query.Get("startLine"))
		}

		if query.Get("startColumn") != "5" {
			t.Errorf("Expected startColumn '5', got '%s'", query.Get("startColumn"))
		}

		if query.Get("endLine") != "20" {
			t.Errorf("Expected endLine '20', got '%s'", query.Get("endLine"))
		}

		if query.Get("endColumn") != "10" {
			t.Errorf("Expected endColumn '10', got '%s'", query.Get("endColumn"))
		}

		// Verify optional parameters
		if query.Get("symbolName") != "main" {
			t.Errorf("Expected symbolName 'main', got '%s'", query.Get("symbolName"))
		}

		if query.Get("includeContent") != "1" {
			t.Errorf("Expected includeContent '1', got '%s'", query.Get("includeContent"))
		}

		if query.Get("maxLayer") != "5" {
			t.Errorf("Expected maxLayer '5', got '%s'", query.Get("maxLayer"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(RelationData{Results: []RelationNode{}})
	}))
	defer server.Close()

	client := NewRelationClient(server.URL)

	req := RelationRequest{
		ClientId:       "test-client",
		CodebasePath:   "/test/project",
		FilePath:       "src/main.go",
		StartLine:      10,
		StartColumn:    5,
		EndLine:        20,
		EndColumn:      10,
		SymbolName:     "main",
		IncludeContent: 1,
		MaxLayer:       5,
		Authorization:  "Bearer test-token",
	}

	ctx := context.Background()
	_, err := client.Search(ctx, req)

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
}

func TestRelationRequest_JSONSerialization(t *testing.T) {
	req := RelationRequest{
		ClientId:       "test-client-123",
		CodebasePath:   "/home/user/project",
		FilePath:       "src/main.go",
		StartLine:      1,
		StartColumn:    1,
		EndLine:        10,
		EndColumn:      2,
		SymbolName:     "main",
		IncludeContent: 1,
		MaxLayer:       3,
		Authorization:  "Bearer token",
	}

	// Test JSON marshaling
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled RelationRequest
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	if unmarshaled.ClientId != req.ClientId ||
		unmarshaled.CodebasePath != req.CodebasePath ||
		unmarshaled.FilePath != req.FilePath ||
		unmarshaled.StartLine != req.StartLine ||
		unmarshaled.StartColumn != req.StartColumn ||
		unmarshaled.EndLine != req.EndLine ||
		unmarshaled.EndColumn != req.EndColumn ||
		unmarshaled.SymbolName != req.SymbolName ||
		unmarshaled.IncludeContent != req.IncludeContent ||
		unmarshaled.MaxLayer != req.MaxLayer {
		t.Errorf("Unmarshaled request doesn't match original: %+v != %+v", unmarshaled, req)
	}
}

func TestRelationData_JSONSerialization(t *testing.T) {
	resp := RelationData{
		Results: []RelationNode{
			{
				Content:  "func main() { ... }",
				NodeType: "definition",
				FilePath: "src/main.go",
				Position: Position{
					StartLine:   1,
					StartColumn: 1,
					EndLine:     5,
					EndColumn:   2,
				},
				Children: []RelationNode{
					{
						Content:  "import \"fmt\"",
						NodeType: "reference",
						FilePath: "src/utils.go",
						Position: Position{
							StartLine:   1,
							StartColumn: 1,
							EndLine:     1,
							EndColumn:   14,
						},
						Children: []RelationNode{},
					},
				},
			},
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled RelationData
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(unmarshaled.Results) != len(resp.Results) {
		t.Errorf("Expected %d results, got %d", len(resp.Results), len(unmarshaled.Results))
	}

	// Verify first result
	if unmarshaled.Results[0].Content != resp.Results[0].Content ||
		unmarshaled.Results[0].NodeType != resp.Results[0].NodeType ||
		unmarshaled.Results[0].FilePath != resp.Results[0].FilePath ||
		unmarshaled.Results[0].Position.StartLine != resp.Results[0].Position.StartLine ||
		unmarshaled.Results[0].Position.StartColumn != resp.Results[0].Position.StartColumn ||
		unmarshaled.Results[0].Position.EndLine != resp.Results[0].Position.EndLine ||
		unmarshaled.Results[0].Position.EndColumn != resp.Results[0].Position.EndColumn {
		t.Errorf("Result doesn't match: %+v != %+v", unmarshaled.Results[0], resp.Results[0])
	}

	if len(unmarshaled.Results[0].Children) != len(resp.Results[0].Children) {
		t.Errorf("Expected %d children, got %d", len(resp.Results[0].Children), len(unmarshaled.Results[0].Children))
	}
}

func TestRelationClient_Search_DifferentStatusCodes(t *testing.T) {
	testCases := []struct {
		name        string
		statusCode  int
		expectError bool
	}{
		{"BadRequest", http.StatusBadRequest, true},
		{"Unauthorized", http.StatusUnauthorized, true},
		{"Forbidden", http.StatusForbidden, true},
		{"NotFound", http.StatusNotFound, true},
		{"InternalServerError", http.StatusInternalServerError, true},
		{"ServiceUnavailable", http.StatusServiceUnavailable, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, tc.name, tc.statusCode)
			}))
			defer server.Close()

			client := NewRelationClient(server.URL)

			req := RelationRequest{
				ClientId:      "test-client",
				CodebasePath:  "/test/project",
				FilePath:      "src/main.go",
				StartLine:     1,
				StartColumn:   1,
				EndLine:       5,
				EndColumn:     2,
				Authorization: "Bearer test-token",
			}

			ctx := context.Background()
			_, err := client.Search(ctx, req)

			if tc.expectError && err == nil {
				t.Errorf("Expected error for status %d, got nil", tc.statusCode)
			}

			if tc.expectError && err != nil {
				expectedError := fmt.Sprintf("relation search failed! status: %d", tc.statusCode)
				if !strings.Contains(err.Error(), expectedError) {
					t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
				}
			}
		})
	}
}

func TestRelationClient_Search_OptionalParameters(t *testing.T) {
	// Test with only required parameters
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		// Verify required parameters are present
		if query.Get("clientId") == "" {
			t.Error("clientId is required")
		}

		if query.Get("codebasePath") == "" {
			t.Error("codebasePath is required")
		}

		if query.Get("filePath") == "" {
			t.Error("filePath is required")
		}

		if query.Get("startLine") == "" {
			t.Error("startLine is required")
		}

		if query.Get("startColumn") == "" {
			t.Error("startColumn is required")
		}

		if query.Get("endLine") == "" {
			t.Error("endLine is required")
		}

		if query.Get("endColumn") == "" {
			t.Error("endColumn is required")
		}

		// Verify optional parameters are not present
		if query.Get("symbolName") != "" {
			t.Error("symbolName should not be present")
		}

		if query.Get("includeContent") != "" {
			t.Error("includeContent should not be present")
		}

		if query.Get("maxLayer") != "" {
			t.Error("maxLayer should not be present")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(RelationData{Results: []RelationNode{}})
	}))
	defer server.Close()

	client := NewRelationClient(server.URL)

	req := RelationRequest{
		ClientId:      "test-client",
		CodebasePath:  "/test/project",
		FilePath:      "src/main.go",
		StartLine:     1,
		StartColumn:   1,
		EndLine:       5,
		EndColumn:     2,
		Authorization: "Bearer test-token",
	}

	ctx := context.Background()
	_, err := client.Search(ctx, req)

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
}

func ExampleRelationClient_Search() {
	// Create a new relation client
	client := NewRelationClient("http://localhost:8002/codebase-indexer/api/v1/search/relation")

	// Prepare request
	req := RelationRequest{
		ClientId:       "client-123",
		CodebasePath:   "/home/user/myproject",
		FilePath:       "src/main.go",
		StartLine:      10,
		StartColumn:    5,
		EndLine:        15,
		EndColumn:      10,
		SymbolName:     "main",
		IncludeContent: 1,
		MaxLayer:       3,
		Authorization:  "Bearer your-jwt-token",
	}

	// Execute search
	ctx := context.Background()
	result, err := client.Search(ctx, req)
	if err != nil {
		fmt.Printf("Search failed: %v\n", err)
		return
	}

	// Process results
	for _, node := range result.Results {
		fmt.Printf("Found %s in %s at line %d\n", node.NodeType, node.FilePath, node.Position.StartLine)
		for _, child := range node.Children {
			fmt.Printf("  References %s in %s\n", child.NodeType, child.FilePath)
		}
	}
}
