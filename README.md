# Z Tavern Backend

Z Tavern Backend 是一个围绕角色扮演体验打造的 Go 服务端，采用分层架构组合会话管理、AI 回复、语音识别与语音合成能力，支持 REST、SSE 与 WebSocket 多种交互模式。

## ✨ 主要特性

- **🎭 人格化对话**：在创建会话前校验 persona 合法性，自动拼装上下文并保留最近历史对话。
- **🤖 AI 集成**：基于 CloudWeGo Eino 对接 Volcengine Ark 模型，可按配置切换流式（SSE delta）与非流式响应。
- **🎙️ 语音链路**：REST 覆盖 ASR/TTS，全链路 WebSocket 处理 `audio/text/config` 消息，输出 `ai_delta/tts` 等阶段事件。
- **⚡ 实时推送**：SSE 输出结构化事件（`start/delta/message/end/error`），客户端易于消费。
- **🏗️ 模块化架构**：Handler-Service-Model 严格分层，内存态存储便于原型迭代，配套表驱动测试与 Makefile 工具链。

## ✅ 当前功能状态

| 模块 | 能力 | 依赖条件 |
|------|------|----------|
| Persona & 会话 | 列表查询、校验 persona 后创建会话、记录消息 | 无外部依赖 |
| SSE 聊天 | `/api/stream/{sessionID}` 输出 start/delta/message/end 事件并回写对话 | 需配置 Ark 凭证；缺失时返回 `503 ai streaming unavailable` |
| 语音 REST | `/api/speech/transcribe`、`/api/speech/synthesize` 及 session 变体、健康检查 | 需配置火山语音 AppID + Token（未配置时端点存在但调用失败） |
| 语音 WebSocket | `/api/speech/ws/{sessionID}` 处理 audio/text/config，串联 ASR→AI→TTS | 需同时配置 Ark 与语音凭证；缺失任一时路由返回 501 |
| 测试回归 | `go test ./...` 覆盖聊天、流式、语音关键逻辑 | Go 1.24+ 环境 |

## 🏛️ 架构设计

```
cmd/api/            # 程序入口与服务装配

internal/
├── config/         # 环境变量解析与配置结构
├── handler/        # HTTP & WebSocket 处理层
│   ├── persona/    # 角色接口
│   ├── chat/       # 会话与消息接口
│   ├── speech/     # 语音 REST + WS
│   ├── stream/     # SSE AI 回复
│   └── router.go   # 路由注册
├── middleware/     # CORS / 日志等中间件
├── model/          # Persona / Chat / Speech 数据模型
└── service/        # 业务服务：ai / chat / speech

pkg/utils/          # SSE、响应写入等通用工具
config.toml         # 示例配置
ROADMAP.md          # 路线图
```

- 依赖方向固定为 Handler → Service → Model。
- Service 负责业务逻辑，不触碰 HTTP 细节；Model 仅定义结构体；Handler 只做参数校验与错误映射。
- 详细仓库规范见 [`AGENTS.md`](./AGENTS.md)。

## 🚀 快速开始

### 环境准备

- Go 1.24 或更高版本
- 可选：`golangci-lint`、Docker

### 安装与运行

```bash
# 1. 克隆项目
git clone <repo-url>
cd z-tavern/backend

# 2. 安装依赖
make install-deps

# 3. 配置环境变量（可选）
cp .env.example .env   # 如需，请按需新增相关键值
# 或直接导出环境变量，例如：
export ARK_API_KEY=xxx
export Model=ep-xx

# 4. 启动服务
make run
# 或指定端口
PORT=3000 go run ./cmd/api
```

> 未配置 Ark 时，AI 服务跳过初始化，`/api/stream/*` 将返回 503；未配置语音凭证时，语音 WebSocket 返回 501，REST 端点会返回业务错误。

## ⚙️ 配置说明

### 服务器

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | 监听端口，支持 `:8080` / `127.0.0.1:8080` 格式 | `:8080` |

### Ark / AI

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `ARK_API_KEY` | Ark API Key（或使用 AK/SK） | `""` |
| `ARK_ACCESS_KEY` / `ARK_SECRET_KEY` | Ark AK/SK | `""` |
| `Model` | 默认模型 ID | `""` |
| `ARK_BASE_URL` | Ark 接口地址 | `https://ark.cn-beijing.volces.com/api/v3` |
| `ARK_REGION` | 区域 | `cn-beijing` |
| `ARK_TEMPERATURE` / `ARK_TOP_P` | 采样参数，可选 | - |
| `ARK_MAX_TOKENS` | 最大生成 Token，可选 | - |
| `ARK_STREAM` | 是否启用 SSE 流式输出 | `true` |

AI 服务需满足 `Model` + (`ARK_API_KEY` 或 `ARK_ACCESS_KEY`+`ARK_SECRET_KEY`) 才会启用。

### 语音服务

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `SPEECH_APP_ID` | 火山语音应用 ID | `""` |
| `SPEECH_ACCESS_TOKEN` / `SPEECH_API_KEY` | 火山语音 Token；缺省时回退 Ark API Key | `""` |
| `SPEECH_ACCESS_KEY` / `SPEECH_SECRET_KEY` | 火山语音 AK/SK；缺省时回退 Ark AK/SK | `""` |
| `SPEECH_BASE_URL` | 语音 REST 地址 | `""` |
| `SPEECH_REGION` | 区域 | `cn-beijing` |
| `SPEECH_ASR_MODEL` | 语音识别模型 | `""` |
| `SPEECH_ASR_LANGUAGE` | 识别语言 | `zh-CN` |
| `SPEECH_TTS_VOICE` | 合成音色 | `""` |
| `SPEECH_TTS_LANGUAGE` | 合成语言 | `zh-CN` |
| `SPEECH_TTS_SPEED` | 合成语速 | `1.0` |
| `SPEECH_TTS_VOLUME` | 合成音量 | `1.0` |
| `SPEECH_TIMEOUT` | 语音请求超时（秒） | `30` |

更多可选项详见 `internal/config/config.go`。

## 📡 API 接口

### 核心 REST

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/personas` | 获取角色列表 |
| `POST` | `/api/session` | 创建新会话（需 personaId） |
| `POST` | `/api/messages` | 保存会话消息 |
| `GET` | `/api/stream/{sessionID}` | SSE AI 回复（需 `message` 查询参数） |

### 语音 REST

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/speech/transcribe` | 语音转文本（multipart），可带 `sessionId` 字段 |
| `POST` | `/api/speech/transcribe/{sessionID}` | 指定会话的语音转文本 |
| `POST` | `/api/speech/synthesize` | 文本转语音（JSON），支持指定音色/语速/格式 |
| `POST` | `/api/speech/synthesize/{sessionID}` | 指定会话的语音合成 |
| `GET` | `/api/speech/health` | 语音服务健康检查 |

### 语音 WebSocket

- `GET /api/speech/ws/{sessionID}`：在 AI 与语音服务可用时启用。
- 入站消息 `type`：`audio`（Base64 音频片段）、`text`（纯文本）、`config`（动态切换 persona/voice 及开关）。
- 出站消息统一封装为 `{"Type":"result","Data":{"type":...}}`，`data.type` 取值包括 `connected/asr/user/ai_delta/ai/tts/config`；错误通过 `Type:"error"` 下发。

更多字段说明、前端示例与调试手册参见 `docs/backend_api_frontend_usage.md`。

## 使用示例

```bash
# 创建会话
curl -X POST http://localhost:8080/api/session \
  -H "Content-Type: application/json" \
  -d '{"personaId": "harry-potter"}'

# 发送消息
curl -X POST http://localhost:8080/api/messages \
  -H "Content-Type: application/json" \
  -d '{"sessionId": "xxx", "sender": "user", "content": "你好"}'

# SSE 流式响应
curl "http://localhost:8080/api/stream/xxx?message=你好"

# 上传语音进行识别
curl -X POST http://localhost:8080/api/speech/transcribe/xxx \
  -F "audio=@16k16bit.wav" \
  -F "language=zh-CN"

# WebSocket 语音交互（使用 wscat）
wscat -c ws://localhost:8080/api/speech/ws/xxx
> {"type":"config","data":{"language":"zh-CN","ttsEnabled":true}}
> {"type":"text","data":{"text":"给我讲个故事"}}
```

> 语音 WebSocket 音频分片需先编码为 Base64 放在 `audioData` 字段；或根据客户端实现拆分二进制帧。

## 🛠️ 开发指南

1. 明确需求落在哪一层，优先复用现有 Service 能力。
2. 如需新数据结构，先在 `internal/model` 添加，再扩展 Service。
3. Handler 中只做参数解析、调用 Service、统一错误输出。
4. 更新 `internal/config` 以支持新的环境变量，并同步文档。
5. 编写/更新测试：Service 推荐表驱动，Handler 使用 `httptest`。
6. 运行 `make test`、必要时执行 `make lint`、`make race`。
7. 更新文档（README、AGENTS、docs/），保持前后端协同。

常用命令：

```bash
make run          # 启动开发服务
make build        # 构建可执行文件
make test         # 运行测试
make race         # 竞态检测
make lint         # 代码检查（自动回退 go vet）
make fmt          # gofmt
make ci           # CI 全流程（fmt + vet + lint + test + race）
```

## 🧪 测试

```bash
# 运行所有测试
GOCACHE=$(pwd)/.gocache go test ./...

# 带覆盖率
make test-coverage

# 竞态检测
make race

# 性能基准
make bench
```

测试结束后可删除 `.gocache` 清理缓存。

## 📦 部署

```bash
# Docker 构建与运行
make docker-build
make docker-run

# 二进制部署
make build
./bin/z-tavern-backend
```

## 🤝 贡献指南

1. Fork 本仓库
2. 创建特性分支：`git checkout -b feature/amazing-feature`
3. 提交更改：`git commit -m 'feat: add amazing feature'`
4. 推送分支：`git push origin feature/amazing-feature`
5. 创建 Pull Request

请确保：
- 遵循 Conventional Commits
- 编写/更新测试
- 更新相关文档
- CI 全绿后请求评审

更多规范详见 [`AGENTS.md`](./AGENTS.md)。

## 📄 许可证

本项目采用 [MIT License](LICENSE)。

## 🏗️ 技术栈

- Go 1.24+
- Chi Router
- CloudWeGo Eino + Volcengine Ark
- Volcengine Speech API
- Server-Sent Events / WebSocket

## 📈 项目状态

- ✅ 基础聊天功能
- ✅ AI 集成与流式响应
- ✅ 语音识别与合成
- ✅ 分层架构重构
- 🚧 认证与权限
- 🚧 数据持久化
- 🚧 微服务拆分
