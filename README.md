# Chat-RAG 🚀

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.24.2-blue.svg)](https://golang.org/doc/go1.24) [![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE) [![Docker](https://img.shields.io/badge/docker-available-blue.svg)](Dockerfile) [![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)](#)

[English](#english) | [中文](./README.zh-CN.md)

</div>

## 🎯 Overview

Chat-RAG is a high-performance, enterprise-grade chat service that combines Large Language Models (LLM) with Retrieval-Augmented Generation (RAG) capabilities. It provides intelligent context processing, tool integration, and streaming responses for modern AI applications.

### Key Features

- **🧠 Intelligent Context Processing**: Advanced prompt engineering with context compression and filtering
- **🔧 Tool Integration**: Seamless integration with semantic search, code definition lookup, and knowledge base queries
- **⚡ Streaming Support**: Real-time streaming responses with Server-Sent Events (SSE)
- **🛡️ Enterprise Security**: JWT-based authentication and request validation
- **📊 Comprehensive Monitoring**: Built-in metrics and logging with Prometheus support
- **🔄 Multi-Modal Support**: Support for various LLM models and function calling
- **🚀 High Performance**: Optimized for low-latency responses and high throughput

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   API Gateway   │───▶│  Chat Handler   │───▶│  Prompt Engine  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Authentication│    │  LLM Client     │    │  Tool Executor  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Metrics       │    │  Redis Cache    │    │  Search Tools   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 🚀 Quick Start

### Prerequisites

- Go 1.24.2 or higher
- Redis 6.0+ (optional, for caching)
- Docker (optional, for containerized deployment)

### Installation

```bash
# Clone the repository
git clone https://github.com/zgsm-ai/chat-rag.git
cd chat-rag

# Install dependencies
make deps

# Build the application
make build

# Run with default configuration
make run
```

### Docker Deployment

```bash
# Build Docker image
make docker-build

# Run container
make docker-run
```

## ⚙️ Configuration

The service is configured via YAML files. See [`etc/chat-api.yaml`](etc/chat-api.yaml) for the default configuration:

```yaml
Name: chat-rag
Host: 0.0.0.0
Port: 8888

# LLM Configuration
LLM:
  Endpoint: "http://127.0.0.1:30616/v1/chat/completions"
  FuncCallingModels: ["gpt-4", "claude-3"]

# Redis Configuration
Redis:
  Addr: "127.0.0.1:6379"
  Password: ""
  DB: 0

# Tool Configuration
Tools:
  SemanticSearch:
    SearchEndpoint: "http://localhost:9001/api/v1/search/semantic"
    TopK: 50
    ScoreThreshold: 0.7
```

## 📡 API Endpoints

### Chat Completion

```http
POST /chat-rag/api/v1/chat/completions
Content-Type: application/json
Authorization: Bearer <token>

{
  "model": "gpt-4",
  "messages": [
    {
      "role": "user",
      "content": "What is the weather like today?"
    }
  ],
  "stream": true,
  "extra_body": {
    "prompt_mode": "rag_compress"
  }
}
```

### Request Status

```http
GET /chat-rag/api/v1/chat/requests/{requestId}/status
```

### Metrics

```http
GET /metrics
```

## 🔧 Development

### Project Structure

```
chat-rag/
├── internal/
│   ├── handler/          # HTTP handlers
│   ├── logic/           # Business logic
│   ├── client/          # External service clients
│   ├── promptflow/      # Prompt processing pipeline
│   ├── functions/       # Tool execution engine
│   └── config/          # Configuration management
├── etc/                 # Configuration files
├── test/               # Test files
└── deploy/             # Deployment configurations
```

### Available Commands

```bash
make help              # Show available commands
make build            # Build the application
make test             # Run tests
make fmt              # Format code
make vet              # Vet code
make docker-build     # Build Docker image
make dev              # Run development server with auto-reload
```

### Testing

```bash
# Run all tests
make test

# Run specific test
go test -v ./internal/logic/

# Run with coverage
go test -cover ./...
```

## 🔍 Advanced Features

### Context Compression

Intelligent context compression to handle long conversations:

```yaml
ContextCompressConfig:
  EnableCompress: true
  TokenThreshold: 100000
  SummaryModel: "qwen2.5-coder-32b"
  RecentUserMsgUsedNums: 3
```

### Tool Integration

Support for multiple search and analysis tools:

- **Semantic Search**: Vector-based code and document search
- **Definition Search**: Code definition lookup
- **Reference Search**: Code reference analysis
- **Knowledge Search**: Document knowledge base queries

### Agent-Based Processing

Configurable agent matching for specialized tasks:

```yaml
AgentsMatch:
  - AgentName: "strict"
    MatchKey: "a strict strategic workflow controller"
  - AgentName: "code"
    MatchKey: "a highly skilled software engineer"
```

## 📊 Monitoring & Observability

### Metrics

The service exposes Prometheus metrics at `/metrics` endpoint:

- Request count and latency
- Token usage statistics
- Tool execution metrics
- Error rates and types

### Logging

Structured logging with Zap logger:

- Request/response logging
- Error tracking
- Performance metrics
- Debug information

## 🔒 Security

- JWT-based authentication
- Request validation and sanitization
- Rate limiting support
- Secure header handling

## 🚢 Deployment

### Production Deployment

```bash
# Build for production
CGO_ENABLED=0 GOOS=linux go build -o chat-rag .

# Run with production config
./chat-rag -f etc/prod.yaml
```

### Kubernetes Deployment

See [`deploy/`](deploy/) directory for Kubernetes manifests and Helm charts.

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

For support and questions:
- Create an issue in the GitHub repository
- Contact the maintainers

---

<div align="center">
  <b>⭐ If this project helps you, please give us a star!</b>
</div>