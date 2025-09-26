# Repository Guidelines

## 项目结构与架构设计

### 分层架构原则

项目采用严格的分层架构，各层职责明确分离：

```
internal/
├── model/          # 数据模型层
│   ├── persona/   # 角色相关模型
│   ├── chat/      # 聊天相关模型
│   └── speech/    # 语音相关模型
├── service/       # 业务逻辑层
│   ├── ai/        # AI服务
│   ├── chat/      # 聊天服务
│   └── speech/    # 语音服务
├── handler/       # HTTP处理层
│   ├── persona/   # 角色处理器
│   ├── chat/      # 聊天处理器
│   ├── speech/    # 语音处理器
│   ├── stream/    # 流式响应处理器
│   └── router.go  # 路由组装
├── middleware/    # 中间件层
├── config/        # 配置管理
└── pkg/
    └── utils/     # 公共工具
```

### 各层职责说明

- **Model层**: 只定义数据结构，不包含业务逻辑
- **Service层**: 纯业务逻辑，不包含HTTP处理代码
- **Handler层**: HTTP请求响应处理，调用Service执行业务逻辑
- **Middleware层**: 跨切面功能（CORS、认证、日志等）
- **Utils层**: 可复用的工具函数

### 重要架构约定

1. **严格的依赖方向**: Handler → Service → Model
2. **Service层不得包含HTTP相关代码**: 如`http.ResponseWriter`、路由注册等
3. **Model层保持纯净**: 只定义数据结构，不包含业务方法
4. **Handler层保持薄层**: 只做请求响应转换，复杂逻辑下沉到Service

### 实时语音链路约定

- 语音 REST 与 WebSocket 背后统一依赖 `internal/service/speech`，Handler 层只做参数拆装
- `SpeechService` 接口需同时提供 `TranscribeAudio/TranscribeBuffer/SynthesizeSpeech/SynthesizeToBuffer`
- WebSocket 处理器需要 `chat.Service` 与 persona store 以维持会话上下文；消息类型限定为 `audio/text/config`
- 未配置语音或 AI 服务时，WebSocket 路由必须优雅降级为 501 或错误提示，避免前端阻塞
- WebSocket 输出通过 `result` 事件描述 `asr`、`ai_delta`、`ai`、`tts` 等阶段，客户端需按 `type` 字段解析

## 开发规范

### 代码组织原则

- Go 模块位于 `github.com/zhouzirui/z-tavern/backend`
- 入口 `cmd/api/main.go` 负责装配服务和启动HTTP服务器
- 新功能按层次放置，确保职责单一
- 公共配置集中在 `internal/config` 包
- 使用依赖注入模式，避免全局变量

### 构建、测试与开发命令

- `go run ./cmd/api`：本地启动服务，默认监听 `:8080`
- `go build ./cmd/api`：构建发布可执行文件
- `go test ./...`：运行所有单元测试
- `make build`：使用Makefile构建
- `make test`：使用Makefile运行测试
- `make lint`：代码检查

### 编码风格与命名约定

- 使用 Go 默认格式，提交前执行 `go fmt ./...`
- 导出标识符采用驼峰命名
- 包级错误遵循 `ErrXYZ` 模式
- 构造函数统一为 `New` 或 `NewType`
- Service构造函数命名为 `NewService`
- Handler构造函数命名为 `New`

### 测试指南

- 优先编写表驱动测试
- Service层可独立测试，不依赖HTTP
- Handler层使用 `httptest` 验证HTTP响应
- 合并前运行 `go test ./... -race` 捕获并发问题

## 新功能开发流程

### 1. 添加新业务功能

1. **定义Model**: 在 `internal/model/` 创建数据结构
2. **实现Service**: 在 `internal/service/` 实现业务逻辑
3. **创建Handler**: 在 `internal/handler/` 处理HTTP请求
4. **注册路由**: 在相应handler中注册路由
5. **更新Router**: 在 `router.go` 中组装新handler

### 2. 添加新的HTTP端点

```go
// 1. 在handler包中实现
func (h *Handler) HandleNewEndpoint(w http.ResponseWriter, r *http.Request) {
    // HTTP请求处理逻辑
    result, err := h.service.DoSomething(r.Context(), params)
    if err != nil {
        utils.RespondError(w, http.StatusInternalServerError, err.Error())
        return
    }
    utils.RespondJSON(w, http.StatusOK, result)
}

// 2. 注册路由
func (h *Handler) RegisterRoutes(r chi.Router) {
    r.Post("/new-endpoint", h.HandleNewEndpoint)
}
```

### 3. 错误处理规范

- Service层返回具体的业务错误
- Handler层处理错误并返回适当的HTTP状态码
- 使用 `pkg/utils` 中的响应工具函数
- 记录详细错误日志用于调试

## 配置与环境

### 环境变量配置

核心配置由 `internal/config` 解析，支持的环境变量：

- `PORT`: 监听端口
- `ARK_API_KEY`: AI服务密钥
- `Model`: AI模型ID
- 更多配置参考 `configs/README.md`

### 部署说明

- 使用 `make build` 构建生产版本
- 支持Docker部署（参考Makefile）
- 确保所有环境变量正确配置

## 提交与拉取请求规范

- 遵循 Conventional Commits 格式
- PR需关联问题，描述用户可见变更
- 附上测试证据（测试输出或功能截图）
- CI全绿后再请求评审

## 架构演进指导

### 当前架构优势

1. **清晰的分层**: 易于理解和维护
2. **职责分离**: 每层只关注自己的责任
3. **可测试性**: Service层可独立测试
4. **可扩展性**: 容易添加新功能

### 未来扩展方向

- 考虑引入Repository模式替换内存存储
- 添加更多中间件支持（认证、限流等）
- 考虑引入事件驱动架构
- 支持微服务拆分

## 火山引擎语音接口接入备注

- **TTS 示例(`volcengine_binary_demo/volcengine/binary/main.go`)**: 文档推荐对大模型音色使用 V3 流式端点（`wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream` 等），并指出 `encoding=wav` 在流式场景不支持。当前示例仍默认 `v1/ws_binary` 且 `encoding` 默认值为 `wav`，接入时需改为 V3 接口并使用文档支持的编码（如 `pcm` 或 `ogg_opus`）。
- **ASR 示例(`sauc_go`)**:
  - 握手请求需要携带 `X-Api-Connect-Id`（文档建议用于排查），当前 `request/header.go` 仅设置 `X-Api-Resource-Id/X-Api-Access-Key/X-Api-App-Key`。
  - WebSocket 音频分片必须使用二进制帧发送，`client/client.go` 现用 `websocket.TextMessage` 会违背「二进制协议」约束，需要改为 `websocket.BinaryMessage`。
  - 文档定义 Full Client Request 的 `Message type specific flags` 为 `0b0000`，示例默认 `common.POS_SEQUENCE` 并额外写入 `sequence` 字段，建议切换到无序列化标记，避免和协议不一致。

> 参考文档：大模型语音合成 API（https://www.volcengine.com/docs/6561/1257584），大模型流式语音识别 API（https://www.volcengine.com/docs/6561/1354869）。
