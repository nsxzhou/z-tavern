# Repository Guidelines

## 项目总览

- 服务定位：面向角色扮演体验的 Go 后端，提供会话管理、AI 回复、语音识别与语音合成能力。
- 核心能力：Persona 校验与上下文管理、SSE 文本流式输出、语音 REST/WebSocket 闭环、可选 Ark 大模型与火山语音集成。
- 当前状态：所有对话与语音数据存储于内存，适用于原型和前端联调；认证、持久化、分布式部署仍在规划阶段。

## 架构与分层

```
cmd/api/            # 程序入口
├── main.go         # 装配配置、服务与路由

internal/
├── config/         # 配置加载与校验
├── handler/        # HTTP 与 WebSocket 处理层
│   ├── persona/    # 角色管理接口
│   ├── chat/       # 会话与消息接口
│   ├── speech/     # 语音 REST + WebSocket
│   ├── stream/     # SSE 流式回复
│   └── router.go   # 统一路由装配
├── middleware/     # CORS、日志等通用中间件
├── model/          # 数据模型（persona/chat/speech）
└── service/        # 业务逻辑（ai/chat/speech）

pkg/utils/          # SSE、响应封装等通用工具
config.toml         # 示例配置（可选）
docs/               # 深入文档，如接口对接手册
```

### 依赖约束

1. 依赖方向保持 Handler → Service → Model，不允许反向引用。
2. Service 层只关注业务流程，不出现 `http.ResponseWriter`、路由注册或 UI 语义。
3. Model 层保持纯净，只定义结构体与常量；跨模块逻辑放在 Service。
4. Handler 保持薄层，负责参数解析、错误映射、调用 Service。

## 核心模块说明

- `internal/service/ai`：基于 CloudWeGo Eino 组装 Ark 对话链路，支持串流与非串流；`StreamingEnabled` 受 `ARK_STREAM` 控制。
- `internal/service/chat`：内存态会话/消息存储，提供创建、写入与读取接口；错误集中在 `ErrPersonaRequired` 与 `ErrSessionNotFound`。
- `internal/service/speech`：封装火山语音 ASR/TTS 的 REST 与缓冲区调用，HTTP 与 WebSocket 共享实现。
- `internal/handler/stream`：包装 SSE 输出，处理 `start/delta/message/end/error` 事件并回写聊天记录；若 AI 未启用则路由直接返回 `503 ai streaming unavailable`。
- `internal/handler/speech`：导出 `/transcribe`、`/synthesize` 系列端点、健康检查以及 `/ws/{sessionID}` WebSocket，支持带会话 ID 的路由变体。

## 实时语音链路约定

- REST 端点：
  - `POST /api/speech/transcribe` 与 `POST /api/speech/transcribe/{sessionID}`（`multipart/form-data`）。
  - `POST /api/speech/synthesize` 与 `POST /api/speech/synthesize/{sessionID}`（`application/json`）。
  - `GET /api/speech/health` 返回 `{status:"healthy"}`。
- WebSocket：`GET /api/speech/ws/{sessionID}` 在 AI、语音、聊天服务均启用时可用，否则返回 501。
  - 入站消息类型：`audio`（Base64 音频分片）、`text`（纯文本）、`config`（动态开关/语言/音色）。
  - 出站统一封装在 `result` 事件内，`data.type` 可为 `connected/asr/user/ai_delta/ai/tts/config`，错误通过 `Type:"error"` 发送。
  - 服务端每 54 秒发送 Ping，客户端需响应 Pong 维持连接。
- 参考文档详见 `docs/backend_api_frontend_usage.md`。

## 配置清单

### HTTP 服务

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `PORT` | 监听端口，支持 `:8080` 或 `127.0.0.1:8080` | `:8080` |

### Ark / AI

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `ARK_API_KEY` | Ark API Key（或使用 AK/SK 组合） | `""` |
| `ARK_ACCESS_KEY` / `ARK_SECRET_KEY` | Ark AK/SK | `""` |
| `Model` | 默认模型 ID | `""` |
| `ARK_BASE_URL` | Ark 接口地址 | `https://ark.cn-beijing.volces.com/api/v3` |
| `ARK_REGION` | 区域 | `cn-beijing` |
| `ARK_TEMPERATURE` / `ARK_TOP_P` | 采样参数，可选 | - |
| `ARK_MAX_TOKENS` | 最大生成 Token，可选 | - |
| `ARK_STREAM` | 是否启用 SSE 流式输出 | `true` |

AI 服务在满足 `Model` 与任一凭证组合（API Key 或 AK/SK）时启用，否则相关路由返回 `503 ai streaming unavailable`。

### 语音服务

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `SPEECH_APP_ID` | 火山语音应用 ID | `""` |
| `SPEECH_ACCESS_TOKEN` / `SPEECH_API_KEY` | 火山语音 Token；未提供时回退至 `ARK_API_KEY` | `""` |
| `SPEECH_ACCESS_KEY` / `SPEECH_SECRET_KEY` | 火山语音 AK/SK；未提供时回退至 Ark AK/SK | `""` |
| `SPEECH_BASE_URL` | 语音服务地址（REST） | `""` |
| `SPEECH_REGION` | 区域 | `cn-beijing` |
| `SPEECH_ASR_MODEL` | 识别模型 ID | `""` |
| `SPEECH_ASR_LANGUAGE` | 识别语言 | `zh-CN` |
| `SPEECH_TTS_VOICE` | 合成音色 | `""` |
| `SPEECH_TTS_LANGUAGE` | 合成语言 | `zh-CN` |
| `SPEECH_TTS_SPEED` | 合成语速 | `1.0` |
| `SPEECH_TTS_VOLUME` | 合成音量 | `1.0` |
| `SPEECH_TIMEOUT` | 调用超时时间（秒） | `30` |

当 `SPEECH_APP_ID` 与 Token 缺失时，语音 REST 将仍然暴露但内部服务为 `nil`，WebSocket 会退化到 `501 speech websocket not available`。

## 开发流程与规范

1. **需求梳理**：确认改动落在哪一层，优先复用现有 Service 能力。
2. **模型更新**：必要时在 `internal/model` 定义数据结构，保持纯粹的字段声明。
3. **业务实现**：在对应 Service 注入依赖、编写逻辑，并补充测试（优先表驱动）。
4. **接口接入**：Handler 只做参数解析与错误映射；新的 HTTP 路由通过 `RegisterRoutes` 注册并在 `router.go` 汇总。
5. **配置与开关**：涉及外部依赖时更新 `internal/config` 与文档说明。
6. **验证**：执行 `go test ./...`，必要时跑 `make lint`、`make race`。
7. **文档**：更新 README、AGENTS 或 `docs/`，保持前后端协同信息一致。

### 常用命令

```bash
make run           # 启动开发服务（go run 包装）
make build         # 构建可执行文件
make test          # 运行全部测试
make race          # 并发竞态检测
make lint          # 代码静态检查
make fmt           # gofmt + goimports
```

> 在沙盒环境下可使用 `GOCACHE=$(pwd)/.gocache go test ./...` 避免默认缓存路径写入失败。

## 提交与评审规范

- 提交信息遵循 Conventional Commits（如 `feat: ...`, `fix: ...`, `docs: ...`）。
- PR 需要说明用户可见变更，并附上测试结果截图或命令输出。
- 确保 CI（`make ci`）通过后再请求评审。

## 架构演进方向

- 引入持久化层（可能的 Repository 模式）以替代当前内存存储，支持多实例部署。
- 增强认证、权限及速率限制中间件。
- 优化语音链路：升级到火山引擎 V3 流式接口、统一编码格式。
- 探索事件驱动或队列机制，为语音/AI 延迟任务提供缓冲。

## 更多资料

- 接口细节与前端协作规范：`docs/backend_api_frontend_usage.md`
- 路线规划：`ROADMAP.md`
- 运行示例配置：`config.toml`
