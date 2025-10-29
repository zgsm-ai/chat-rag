# Chat-RAG 🚀

<div align="center">

[![Go版本](https://img.shields.io/badge/Go-1.24.2-blue.svg)](https://golang.org/doc/go1.24) [![许可证](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE) [![Docker](https://img.shields.io/badge/docker-available-blue.svg)](Dockerfile) [![构建状态](https://img.shields.io/badge/build-passing-brightgreen.svg)](#)

[English](./README.md) | [中文](#chinese)

</div>

## 🎯 项目概述

Chat-RAG 是一个高性能、企业级的聊天服务，结合了大语言模型（LLM）与检索增强生成（RAG）功能。它为现代 AI 应用提供智能上下文处理、工具集成和流式响应功能。

### 核心特性

- **🧠 智能上下文处理**：先进的提示工程，支持上下文压缩和过滤
- **🔧 工具集成**：无缝集成语义搜索、代码定义查询和知识库查询
- **⚡ 流式支持**：通过服务器发送事件（SSE）实现实时流式响应
- **🛡️ 企业安全**：基于 JWT 的身份验证和请求验证
- **📊 全面监控**：内置指标和日志记录，支持 Prometheus
- **🔄 多模态支持**：支持各种 LLM 模型和函数调用
- **🚀 高性能**：优化的低延迟响应和高吞吐量

## 🏗️ 架构设计

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   API 网关      │───▶│  聊天处理器     │───▶│  提示引擎       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   身份验证      │    │  LLM 客户端     │    │  工具执行器     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   指标监控      │    │  Redis 缓存     │    │  搜索工具       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 🚀 快速开始

### 环境要求

- Go 1.24.2 或更高版本
- Redis 6.0+（可选，用于缓存）
- Docker（可选，用于容器化部署）

### 安装步骤

```bash
# 克隆仓库
git clone https://github.com/zgsm-ai/chat-rag.git
cd chat-rag

# 安装依赖
make deps

# 构建应用
make build

# 使用默认配置运行
make run
```

### Docker 部署

```bash
# 构建 Docker 镜像
make docker-build

# 运行容器
make docker-run
```

## ⚙️ 配置说明

服务通过 YAML 文件进行配置。查看 [`etc/chat-api.yaml`](etc/chat-api.yaml) 了解默认配置：

```yaml
Name: chat-rag
Host: 0.0.0.0
Port: 8888

# LLM 配置
LLM:
  Endpoint: "http://127.0.0.1:30616/v1/chat/completions"
  FuncCallingModels: ["gpt-4", "claude-3"]

# Redis 配置
Redis:
  Addr: "127.0.0.1:6379"
  Password: ""
  DB: 0

# 工具配置
Tools:
  SemanticSearch:
    SearchEndpoint: "http://localhost:9001/api/v1/search/semantic"
    TopK: 50
    ScoreThreshold: 0.7
```

## 📡 API 端点

### 聊天完成

```http
POST /chat-rag/api/v1/chat/completions
Content-Type: application/json
Authorization: Bearer <token>

{
  "model": "gpt-4",
  "messages": [
    {
      "role": "user",
      "content": "今天天气怎么样？"
    }
  ],
  "stream": true,
  "extra_body": {
    "prompt_mode": "rag_compress"
  }
}
```

### 请求状态

```http
GET /chat-rag/api/v1/chat/requests/{requestId}/status
```

### 指标监控

```http
GET /metrics
```

## 🔧 开发指南

### 项目结构

```
chat-rag/
├── internal/
│   ├── handler/          # HTTP 处理器
│   ├── logic/           # 业务逻辑
│   ├── client/          # 外部服务客户端
│   ├── promptflow/      # 提示处理管道
│   ├── functions/       # 工具执行引擎
│   └── config/          # 配置管理
├── etc/                 # 配置文件
├── test/               # 测试文件
└── deploy/             # 部署配置
```

### 可用命令

```bash
make help              # 显示可用命令
make build            # 构建应用
make test             # 运行测试
make fmt              # 格式化代码
make vet              # 检查代码
make docker-build     # 构建 Docker 镜像
make dev              # 运行开发服务器（支持热重载）
```

### 测试

```bash
# 运行所有测试
make test

# 运行特定测试
go test -v ./internal/logic/

# 带覆盖率运行
go test -cover ./...
```

## 🔍 高级功能

### 上下文压缩

智能上下文压缩处理长对话：

```yaml
ContextCompressConfig:
  EnableCompress: true
  TokenThreshold: 100000
  SummaryModel: "qwen2.5-coder-32b"
  RecentUserMsgUsedNums: 3
```

### 工具集成

支持多种搜索和分析工具：

- **语义搜索**：基于向量的代码和文档搜索
- **定义搜索**：代码定义查询
- **引用搜索**：代码引用分析
- **知识搜索**：文档知识库查询

### 基于代理的处理

可配置的代理匹配，用于专门任务：

```yaml
AgentsMatch:
  - AgentName: "strict"
    MatchKey: "a strict strategic workflow controller"
  - AgentName: "code"
    MatchKey: "a highly skilled software engineer"
```

## 📊 监控和可观测性

### 指标监控

服务在 `/metrics` 端点暴露 Prometheus 指标：

- 请求计数和延迟
- Token 使用统计
- 工具执行指标
- 错误率和类型

### 日志记录

使用 Zap 记录器进行结构化日志记录：

- 请求/响应日志记录
- 错误跟踪
- 性能指标
- 调试信息

## 🔒 安全特性

- 基于 JWT 的身份验证
- 请求验证和清理
- 速率限制支持
- 安全头部处理

## 🚢 部署方案

### 生产部署

```bash
# 构建生产版本
CGO_ENABLED=0 GOOS=linux go build -o chat-rag .

# 使用生产配置运行
./chat-rag -f etc/prod.yaml
```

### Kubernetes 部署

查看 [`deploy/`](deploy/) 目录中的 Kubernetes 清单和 Helm 图表。

## 🤝 贡献指南

1. Fork 仓库
2. 创建功能分支（`git checkout -b feature/amazing-feature`）
3. 提交更改（`git commit -m 'Add some amazing feature'`）
4. 推送到分支（`git push origin feature/amazing-feature`）
5. 打开拉取请求

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🆘 支持

如需支持和提问：
- 在 GitHub 仓库中创建问题
- 联系维护者

---

<div align="center">
  <b>⭐ 如果这个项目对你有帮助，请给我们一个星标！</b>
</div>