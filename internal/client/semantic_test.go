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

func TestNewSemanticClient(t *testing.T) {
	endpoint := "http://localhost:8002/v1/semantic"
	clientInterface := NewSemanticClient(endpoint)

	if clientInterface == nil {
		t.Fatal("NewSemanticClient returned nil")
	}

	// Type assertion to access concrete implementation
	client, ok := clientInterface.(*SemanticClient)
	if !ok {
		t.Fatal("NewSemanticClient did not return *SemanticClient")
	}

	if client.endpoint != endpoint {
		t.Errorf("Expected endpoint %s, got %s", endpoint, client.endpoint)
	}

	if client.httpClient == nil {
		t.Fatal("HTTP client is nil")
	}

	if client.httpClient.Timeout != 3*time.Second {
		t.Errorf("Expected timeout 3s, got %v", client.httpClient.Timeout)
	}
}

func TestSemanticClient_Search_Success(t *testing.T) {
	// Mock response data
	mockResponse := SemanticData{
		Results: []SemanticResult{
			{
				Content:  "This is a test content",
				Score:    0.95,
				FilePath: "/path/to/file1.go",
			},
			{
				Content:  "Another test content",
				Score:    0.87,
				FilePath: "/path/to/file2.go",
			},
		},
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		// Verify content type
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Verify request body
		var req SemanticRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		expectedReq := SemanticRequest{
			ClientId:     "test-client",
			CodebasePath: "/test/project",
			Query:        "test query",
			TopK:         5,
		}

		// Compare individual fields to avoid struct equality failures due to private fields
		if req.ClientId != expectedReq.ClientId ||
			req.CodebasePath != expectedReq.CodebasePath ||
			req.Query != expectedReq.Query ||
			req.TopK != expectedReq.TopK {
			t.Errorf("Expected request fields %+v, got %+v", expectedReq, req)
		}

		// Send mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	// Create client with mock server URL
	client := NewSemanticClient(server.URL)

	// Test request
	req := SemanticRequest{
		ClientId:     "test-client",
		CodebasePath: "/test/project",
		Query:        "test query",
		TopK:         5,
	}

	ctx := context.Background()
	resp, err := client.Search(ctx, req)

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if resp == nil {
		t.Fatal("Response is nil")
	}

	if len(resp.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(resp.Results))
	}

	// Verify first result
	if resp.Results[0].Content != "This is a test content" {
		t.Errorf("Expected content 'This is a test content', got '%s'", resp.Results[0].Content)
	}

	if resp.Results[0].Score != 0.95 {
		t.Errorf("Expected score 0.95, got %f", resp.Results[0].Score)
	}

	if resp.Results[0].FilePath != "/path/to/file1.go" {
		t.Errorf("Expected file path '/path/to/file1.go', got '%s'", resp.Results[0].FilePath)
	}

}

func TestSemanticClient_Search_EmptyResults(t *testing.T) {
	// Mock empty response
	mockResponse := SemanticData{
		Results: []SemanticResult{},
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := NewSemanticClient(server.URL)

	req := SemanticRequest{
		ClientId:     "test-client",
		CodebasePath: "/test/project",
		Query:        "no results query",
		TopK:         5,
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

func TestSemanticClient_Search_HTTPError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewSemanticClient(server.URL)

	req := SemanticRequest{
		ClientId:     "test-client",
		CodebasePath: "/test/project",
		Query:        "test query",
		TopK:         5,
	}

	ctx := context.Background()
	_, err := client.Search(ctx, req)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	expectedError := "semantic search failed with status: 500"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestSemanticClient_Search_InvalidJSON(t *testing.T) {
	// Create mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewSemanticClient(server.URL)

	req := SemanticRequest{
		ClientId:     "test-client",
		CodebasePath: "/test/project",
		Query:        "test query",
		TopK:         5,
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

func TestSemanticClient_Search_ContextCancellation(t *testing.T) {
	// Create mock server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SemanticData{Results: []SemanticResult{}})
	}))
	defer server.Close()

	client := NewSemanticClient(server.URL)

	req := SemanticRequest{
		ClientId:     "test-client",
		CodebasePath: "/test/project",
		Query:        "test query",
		TopK:         5,
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Search(ctx, req)

	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}

	expectedError := "failed to execute request"
	// Use strings.HasPrefix instead of Contains for more precise error checking
	if !strings.HasPrefix(err.Error(), expectedError) {
		t.Errorf("Expected error to start with '%s', got '%s'", expectedError, err.Error())
	}
}

func TestSemanticClient_Search_InvalidURL(t *testing.T) {
	// Create client with invalid URL
	client := NewSemanticClient("invalid-url")

	req := SemanticRequest{
		ClientId:     "test-client",
		CodebasePath: "/test/project",
		Query:        "test query",
		TopK:         5,
	}

	ctx := context.Background()
	_, err := client.Search(ctx, req)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	expectedError := "failed to execute request"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

func TestSemanticRequest_JSONSerialization(t *testing.T) {
	req := SemanticRequest{
		ClientId:     "test-client-123",
		CodebasePath: "/home/user/project",
		Query:        "search for functions",
		TopK:         10,
	}

	// Test JSON marshaling
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled SemanticRequest
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	if unmarshaled != req {
		t.Errorf("Unmarshaled request doesn't match original: %+v != %+v", unmarshaled, req)
	}
}

func TestSemanticData_JSONSerialization(t *testing.T) {
	resp := SemanticData{
		Results: []SemanticResult{
			{
				Content:  "func main() { ... }",
				Score:    0.98,
				FilePath: "/src/main.go",
			},
			{
				Content:  "func helper() { ... }",
				Score:    0.85,
				FilePath: "/src/utils.go",
			},
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled SemanticData
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(unmarshaled.Results) != len(resp.Results) {
		t.Errorf("Expected %d results, got %d", len(resp.Results), len(unmarshaled.Results))
	}

	for i, result := range resp.Results {
		if unmarshaled.Results[i].Content != result.Content ||
			unmarshaled.Results[i].Score != result.Score ||
			unmarshaled.Results[i].FilePath != result.FilePath {
			t.Errorf("Result %d doesn't match: %+v != %+v", i, unmarshaled.Results[i], result)
		}
	}
}

func TestSemanticClient_Search_LargeResponse(t *testing.T) {
	// Create a large mock response
	results := make([]SemanticResult, 50)
	for i := 0; i < 50; i++ {
		results[i] = SemanticResult{
			Content:  strings.Repeat("test content ", 10),
			Score:    float64(i) / 50.0,
			FilePath: "/path/to/file" + string(rune(i+48)) + ".go", // ASCII 48 = '0'
		}
	}

	mockResponse := SemanticData{
		Results: results,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := NewSemanticClient(server.URL)

	req := SemanticRequest{
		ClientId:     "test-client",
		CodebasePath: "/test/project",
		Query:        "large response test",
		TopK:         50,
	}

	ctx := context.Background()
	resp, err := client.Search(ctx, req)

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(resp.Results) != 50 {
		t.Errorf("Expected 50 results, got %d", len(resp.Results))
	}
}

func TestSemanticClient_Search_DifferentStatusCodes(t *testing.T) {
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

			client := NewSemanticClient(server.URL)

			req := SemanticRequest{
				ClientId:     "test-client",
				CodebasePath: "/test/project",
				Query:        "test query",
				TopK:         5,
			}

			ctx := context.Background()
			_, err := client.Search(ctx, req)

			if tc.expectError && err == nil {
				t.Errorf("Expected error for status %d, got nil", tc.statusCode)
			}

			if tc.expectError && err != nil {
				expectedError := fmt.Sprintf("semantic search failed with status: %d", tc.statusCode)
				if !strings.Contains(err.Error(), expectedError) {
					t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
				}
			}
		})
	}
}

func TestSemanticClient_Search_EdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		request  SemanticRequest
		response SemanticData
	}{
		{
			name: "EmptyQuery",
			request: SemanticRequest{
				ClientId:     "test-client",
				CodebasePath: "/test/project",
				Query:        "",
				TopK:         5,
			},
			response: SemanticData{Results: []SemanticResult{}},
		},
		{
			name: "ZeroTopK",
			request: SemanticRequest{
				ClientId:     "test-client",
				CodebasePath: "/test/project",
				Query:        "test query",
				TopK:         0,
			},
			response: SemanticData{Results: []SemanticResult{}},
		},
		{
			name: "LargeTopK",
			request: SemanticRequest{
				ClientId:     "test-client",
				CodebasePath: "/test/project",
				Query:        "test query",
				TopK:         1000,
			},
			response: SemanticData{Results: []SemanticResult{}},
		},
		{
			name: "SpecialCharactersInQuery",
			request: SemanticRequest{
				ClientId:     "test-client",
				CodebasePath: "/test/project",
				Query:        "test query with special chars: !@#$%^&*(){}[]|\\:;\"'<>?,./",
				TopK:         5,
			},
			response: SemanticData{Results: []SemanticResult{}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(tc.response)
			}))
			defer server.Close()

			client := NewSemanticClient(server.URL)

			ctx := context.Background()
			resp, err := client.Search(ctx, tc.request)

			if err != nil {
				t.Errorf("Unexpected error for %s: %v", tc.name, err)
			}

			if resp == nil {
				t.Errorf("Response is nil for %s", tc.name)
			}
		})
	}
}

// Benchmark tests
func BenchmarkSemanticClient_Search(b *testing.B) {
	mockResponse := SemanticData{
		Results: []SemanticResult{
			{
				Content:  "Benchmark test content",
				Score:    0.95,
				FilePath: "/path/to/benchmark.go",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := NewSemanticClient(server.URL)

	req := SemanticRequest{
		ClientId:     "benchmark-client",
		CodebasePath: "/benchmark/project",
		Query:        "benchmark query",
		TopK:         1,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Search(ctx, req)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

func BenchmarkSemanticClient_Search_LargeResponse(b *testing.B) {
	// Create a large response for benchmarking
	results := make([]SemanticResult, 100)
	for i := 0; i < 100; i++ {
		results[i] = SemanticResult{
			Content:  strings.Repeat("benchmark content ", 20),
			Score:    float64(i) / 100.0,
			FilePath: "/benchmark/file" + string(rune(i+48)) + ".go",
		}
	}

	mockResponse := SemanticData{
		Results: results,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := NewSemanticClient(server.URL)

	req := SemanticRequest{
		ClientId:     "benchmark-client",
		CodebasePath: "/benchmark/project",
		Query:        "large benchmark query",
		TopK:         100,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Search(ctx, req)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

// Example test to demonstrate usage
func ExampleSemanticClient_Search() {
	// Create a semantic client
	client := NewSemanticClient("http://localhost:8002/v1/semantic")

	// Prepare search request
	req := SemanticRequest{
		ClientId:     "example-client",
		CodebasePath: "/path/to/project",
		Query:        "find function definitions",
		TopK:         10,
	}

	// Perform search
	ctx := context.Background()
	resp, err := client.Search(ctx, req)
	if err != nil {
		// Handle error
		return
	}

	// Process results
	for _, result := range resp.Results {
		_ = result.Content  // Use the content
		_ = result.Score    // Use the score
		_ = result.FilePath // Use the file path
	}
}
