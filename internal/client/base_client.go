package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RequestBuilderStrategy defines the request builder strategy interface
type RequestBuilderStrategy[TRequest any] interface {
	// BuildRequest converts a specific type of request to a generic HTTP request
	BuildRequest(req TRequest) Request
}

// ReadyRequestBuilder defines the Ready request builder strategy interface
type ReadyRequestBuilder interface {
	// BuildReadyRequest converts ReadyRequest to a generic HTTP request
	BuildReadyRequest(req ReadyRequest) Request
}

// ResponseHandlerStrategy defines the response handler strategy interface
type ResponseHandlerStrategy[TResponse any] interface {
	// HandleResponse processes HTTP response and converts to specific type of response
	HandleResponse(resp *http.Response) (TResponse, error)
}

// ReadyResponseHandler defines the Ready response handler strategy interface
type ReadyResponseHandler interface {
	// HandleReadyResponse processes HTTP response for Ready check
	HandleReadyResponse(resp *http.Response) (bool, error)
}

// BaseClientInterface defines the universal base client interface
type BaseClientInterface[TRequest any, TResponse any] interface {
	// Search executes search request
	Search(ctx context.Context, req TRequest) (TResponse, error)
	// CheckReady checks if service is available
	CheckReady(ctx context.Context, req ReadyRequest) (bool, error)
}

// BaseClient universal base client implementation
type BaseClient[TRequest any, TResponse any] struct {
	searchClient         *HTTPClient
	readyClient          *HTTPClient
	requestBuilder       RequestBuilderStrategy[TRequest]
	readyRequestBuilder  ReadyRequestBuilder
	responseHandler      ResponseHandlerStrategy[TResponse]
	readyResponseHandler ReadyResponseHandler
}

// BaseClientConfig base client configuration
type BaseClientConfig struct {
	SearchEndpoint string
	ReadyEndpoint  string
	SearchTimeout  time.Duration
	ReadyTimeout   time.Duration
}

// NewBaseClient creates a new base client instance
func NewBaseClient[TRequest any, TResponse any](
	config BaseClientConfig,
	requestBuilder RequestBuilderStrategy[TRequest],
	readyRequestBuilder ReadyRequestBuilder,
	responseHandler ResponseHandlerStrategy[TResponse],
	readyResponseHandler ReadyResponseHandler,
) *BaseClient[TRequest, TResponse] {

	searchConfig := HTTPClientConfig{
		Timeout: config.SearchTimeout,
	}
	readyConfig := HTTPClientConfig{
		Timeout: config.ReadyTimeout,
	}

	return &BaseClient[TRequest, TResponse]{
		searchClient:         NewHTTPClient(config.SearchEndpoint, searchConfig),
		readyClient:          NewHTTPClient(config.ReadyEndpoint, readyConfig),
		requestBuilder:       requestBuilder,
		readyRequestBuilder:  readyRequestBuilder,
		responseHandler:      responseHandler,
		readyResponseHandler: readyResponseHandler,
	}
}

// Search implements search method
func (c *BaseClient[TRequest, TResponse]) Search(ctx context.Context, req TRequest) (TResponse, error) {
	httpReq := c.requestBuilder.BuildRequest(req)
	resp, err := c.searchClient.DoRequest(ctx, httpReq)
	if err != nil {
		var zero TResponse
		return zero, err
	}
	defer resp.Body.Close()

	return c.responseHandler.HandleResponse(resp)
}

// CheckReady implements service availability check method
func (c *BaseClient[TRequest, TResponse]) CheckReady(ctx context.Context, req ReadyRequest) (bool, error) {
	httpReq := c.readyRequestBuilder.BuildReadyRequest(req)
	resp, err := c.readyClient.DoRequest(ctx, httpReq)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return c.readyResponseHandler.HandleReadyResponse(resp)
}

// StringResponseHandler string response handler strategy
type StringResponseHandler struct{}

func (h *StringResponseHandler) HandleResponse(resp *http.Response) (string, error) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		respBody := ""
		if body != nil {
			respBody = string(body)
		}
		return "", fmt.Errorf(
			"request failed! status: %d, response:%s, url: %s",
			resp.StatusCode, respBody, resp.Request.URL.String(),
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

func (h *StringResponseHandler) HandleReadyResponse(resp *http.Response) (bool, error) {
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %v", err)
	}

	return false, fmt.Errorf("code: %d, body: %s", resp.StatusCode, body)
}
