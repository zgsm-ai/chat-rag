# Chat-RAG: OpenAI Compatible Chat Completion API with RAG Compression

A high-performance, maintainable microservice built with Gin framework that provides OpenAI-compatible `/v1/chat/completions` endpoint with intelligent prompt compression using RAG (Retrieval-Augmented Generation).

## Features

- **OpenAI Compatibility**: Fully compatible with OpenAI's chat completions API
- **Intelligent Compression**: Automatically compresses long prompts using semantic search and summarization
- **Streaming Support**: Supports both streaming and non-streaming responses
- **Token Management**: Built-in token counting and threshold-based compression
- **Comprehensive Logging**: Detailed logging with async processing and Loki integration
- **Design Patterns**: Implements Strategy, Factory, and Decorator patterns for maintainability
- **High Performance**: Built with Gin framework with proper dependency injection

## Architecture

### Core Components

1. **Handler Layer**: HTTP request handling with OpenAI compatibility
2. **Logic Layer**: Business logic implementation with comprehensive logging
3. **Strategy Layer**: Pluggable prompt processing strategies (direct vs compression)
4. **Client Layer**: External service communication (LLM, Semantic Search)
5. **Service Layer**: Background services (logging, classification, Loki upload)
6. **Model Layer**: Data structures and logging models

### Design Patterns

- **Strategy Pattern**: Different prompt processing strategies based on token count
- **Factory Pattern**: Creates appropriate processors and clients
- **Decorator Pattern**: Middleware for logging and metrics

## Quick Start

### Prerequisites

- Go 1.22+

### Installation

```bash
# Clone the repository
git clone https://github.com/zgsm-ai/chat-rag.git
cd chat-rag

# Bootstrap the project (installs tools, generates code, builds)
make bootstrap

# Or step by step:
make setup         # Generate API code and download deps
make build         # Build the application
```

### Configuration

Edit `etc/chat-api.yaml`:

```yaml
Name: chat-rag
Host: 0.0.0.0
Port: 8080

# Model endpoints
MainModelEndpoint: "http://localhost:8000/v1/chat/completions"
SummaryModelEndpoint: "http://localhost:8001/v1/chat/completions"

# Compression settings
TokenThreshold: 5000
EnableCompression: true

# Semantic search
SemanticApiEndpoint: "http://localhost:8002/codebase-indexer/api/v1/semantics"
TopK: 5

# Logging
LogFilePath: "logs/chat-rag.log"
LokiEndpoint: "http://localhost:3100/loki/api/v1/push"
LogBatchSize: 100
LogScanIntervalSec: 60

# Models
SummaryModel: "deepseek-chat"
```

### Running

```bash
# Run with default config
make run

# Run with custom config
make run-config CONFIG=path/to/your/config.yaml

# Development mode with auto-reload (requires air)
make install-air
make dev
```

## API Usage

### Basic Chat Completion

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {"role": "user", "content": "Hello, how are you?"}
    ],
    "stream": false
  }'
```

### With RAG Context

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {"role": "user", "content": "Explain this code function"}
    ],
    "client_id": "user123",
    "project_path": "/path/to/project",
    "stream": false
  }'
```

### Streaming Response

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {"role": "user", "content": "Write a Python function"}
    ],
    "stream": true
  }'
```

## How It Works

### Compression Flow

1. **Token Analysis**: Count tokens in incoming messages
2. **Threshold Check**: If tokens > threshold, trigger compression
3. **Semantic Search**: Query codebase for relevant context using latest user message
4. **Summarization**: Use LLM to compress context + history + query
5. **Final Assembly**: Combine system prompt + summary + latest user message
6. **LLM Generation**: Send to main model and return response

### Logging Pipeline

1. **Request Logging**: Log each request with metrics (non-blocking)
2. **File Storage**: Write logs to local file system
3. **Background Processing**: Periodic scan and classification using LLM
4. **Loki Upload**: Batch upload classified logs to Loki with labels

## Project Structure

```
chat-rag/
├── etc/                   # Configuration files
│   └── chat-api.yaml     # Service configuration
├── deploy/               # Deployment configurations
├── internal/             # Internal packages
│   ├── bootstrap/        # Service context (DI container)
│   ├── client/           # External service clients
│   │   ├── llm.go       # LangChain-Go LLM client
│   │   ├── semantic.go  # Semantic search client
│   ├── config/          # Configuration structures
│   │   ├── config.go
│   │   └── loader.go
│   ├── handler/         # HTTP handlers
│   ├── logic/          # Business logic
│   ├── model/          # Data models
│   ├── service/        # Background services
│   ├── strategy/       # Strategy pattern implementations
│   ├── tokenizer/      # Token counting utilities
│   ├── types/          # Generated type definitions
│   ├── utils/          # Utility functions
│   └── logger/         # Logging utilities
├── logs/               # Log files (created at runtime)
├── Makefile           # Build and development commands
├── main.go           # Application entry point
└── README.md         # This file
```

## Development

### Available Commands

```bash
make help           # Show all available commands
make build          # Build the application
make run            # Run with default config
make test           # Run tests
make fmt            # Format code
make vet            # Vet code
make clean          # Clean build artifacts
make api-gen        # Regenerate API code
make deps           # Update dependencies
```

### Docker Image

```bash
# Docker build
make docker-build

# Build and push Docker image
make docker-release VERSION=v1.0.0
```

### Adding New Features

1. **New API Endpoints**: Update `api/chat.api` and run `make api-gen`
2. **New Strategies**: Implement `strategy.PromptProcessor` interface
3. **New Clients**: Add to `internal/client/` with proper error handling
4. **New Services**: Add to `internal/service/` with lifecycle management

## Configuration Options

| Option               | Description                        | Default     |
| -------------------- | ---------------------------------- | ----------- |
| `TokenThreshold`     | Token count to trigger compression | 32000       |
| `TopK`               | Number of semantic search results  | 5           |
| `LogScanIntervalSec` | Log processing interval            | 10          |
| `SummaryModel`       | Model for summarization            | deepseek-v3 |

## Monitoring and Observability

### Metrics Logged

- Request/response latencies
- Token counts (original vs compressed)
- Compression ratios
- Error rates
- Semantic search performance
- Model inference times

### Log Categories

- `code_generation`: Creating new code or projects
- `bug_fixing`: Debugging or fixing issues
- `exploration`: Asking questions about code
- `documentation`: Querying documentation
- `optimization`: Performance improvements

## Dependencies

### Core Dependencies

- **gin**: Web framework and microservice toolkit
- **tiktoken-go**: Token counting (with fallback)
- **uuid**: Request ID generation

### External Services

- **Main LLM**: Primary model for chat completions
- **Summary LLM**: DeepSeek v3 for compression
- **Semantic Search**: Codebase indexer API
- **Loki**: Log aggregation and storage
