# Prometheus Metrics 集成

本项目已集成 Prometheus metrics 功能，用于监控聊天完成请求的各种指标。

## 功能特性

### 1. 自动指标收集

- 在处理日志时自动收集指标数据
- 在 `uploadToLoki` 之后进行指标上报
- 基于 `model.ChatLog` 中的数据

### 2. 支持的指标

#### 请求指标

- `chat_rag_requests_total`: 聊天完成请求总数
  - 标签: `client_id`, `model`, `category`

#### Token 指标

- `chat_rag_original_tokens_total`: 原始 token 总数
  - 标签: `client_id`, `model`, `token_type` (system/user/all)
- `chat_rag_compressed_tokens_total`: 压缩后 token 总数
  - 标签: `client_id`, `model`, `token_type` (system/user/all)

#### 压缩指标

- `chat_rag_compression_ratio`: 压缩比分布
  - 标签: `client_id`, `model`
- `chat_rag_compression_triggered_total`: 触发压缩的请求总数
  - 标签: `client_id`, `model`
- `chat_rag_user_prompt_compressed_total`: 用户提示被压缩的请求总数
  - 标签: `client_id`, `model`

#### 延迟指标

- `chat_rag_semantic_latency_ms`: 语义处理延迟（毫秒）
  - 标签: `client_id`, `model`
- `chat_rag_summary_latency_ms`: 摘要处理延迟（毫秒）
  - 标签: `client_id`, `model`
- `chat_rag_main_model_latency_ms`: 主模型处理延迟（毫秒）
  - 标签: `client_id`, `model`
- `chat_rag_total_latency_ms`: 总处理延迟（毫秒）
  - 标签: `client_id`, `model`

#### 响应指标

- `chat_rag_response_tokens_total`: 响应 token 总数
  - 标签: `client_id`, `model`

#### 错误指标

- `chat_rag_errors_total`: 错误总数
  - 标签: `client_id`, `model`, `error_type`

## 使用方法

### 1. 访问 Metrics 端点

启动服务后，可以通过以下 URL 访问 Prometheus metrics：

```
GET http://localhost:8080/metrics
```

### 2. Prometheus 配置

在 Prometheus 配置文件中添加以下 job：

```yaml
scrape_configs:
  - job_name: "chat-rag"
    static_configs:
      - targets: ["localhost:8080"]
    metrics_path: "/metrics"
    scrape_interval: 15s
```

### 3. 示例查询

#### 查看请求总数

```promql
chat_rag_requests_total
```

#### 查看压缩比分布

```promql
histogram_quantile(0.95, chat_rag_compression_ratio_bucket)
```

#### 查看平均延迟

```promql
rate(chat_rag_total_latency_ms_sum[5m]) / rate(chat_rag_total_latency_ms_count[5m])
```

#### 按客户端查看请求量

```promql
sum(rate(chat_rag_requests_total[5m])) by (client_id)
```

## 架构说明

### 组件结构

1. **MetricsService**: 负责 Prometheus 指标的定义和记录
2. **LoggerService**: 集成 MetricsService，在处理日志时自动上报指标
3. **MetricsHandler**: 提供 `/metrics` HTTP 端点

### 集成流程

1. 在 `ServiceContext` 中初始化 `MetricsService`
2. 将 `MetricsService` 注入到 `LoggerService` 中
3. 在 `LoggerService.processLogs()` 中，`uploadToLoki` 成功后调用 `metricsService.RecordChatLog()`
4. 通过 `/metrics` 端点暴露指标给 Prometheus

## 注意事项

1. **性能影响**: 指标收集对性能影响很小，但在高并发场景下建议监控内存使用
2. **标签基数**: 避免使用高基数标签（如 request_id），以防止内存泄漏
3. **数据保留**: Prometheus 默认保留 15 天数据，可根据需要调整
4. **安全性**: 生产环境中建议对 `/metrics` 端点进行访问控制

## 故障排除

### 常见问题

1. **指标不更新**

   - 检查 LoggerService 是否正常运行
   - 确认日志文件是否被正确处理
   - 验证 Loki 上传是否成功

2. **内存使用过高**

   - 检查标签基数是否过高
   - 考虑减少 histogram buckets 数量

3. **Prometheus 无法抓取**
   - 确认服务端口是否正确
   - 检查防火墙设置
   - 验证 `/metrics` 端点是否可访问
