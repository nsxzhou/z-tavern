# Z Tavern Backend

Z Tavern Backend 是一个围绕角色扮演体验打造的 Go 服务端，采用严格的分层架构设计，提供人物设定管理、会话记录、AI 回复生成、语音识别合成以及 Server-Sent Events (SSE) 实时推送能力。

## ✨ 主要特性

- **🎭 人格化对话**: 会话创建前校验 persona 合法性，确保上下文与角色设定匹配
- **🤖 AI 集成**: 通过 CloudWeGo Eino 对接 Volcengine Ark 模型，既支持一次性回复也支持流式增量
- **🎙️ 语音功能**: REST 端点覆盖 ASR/TTS，WebSocket 支持语音→文本→AI→语音的闭环交互
- **⚡ 实时推送**: SSE 与 WebSocket 双通道，输出 delta、配置更新及语音数据
- **🏗️ 模块化架构**: 分层清晰、配套单元测试，易于维护和扩展

## ✅ 当前功能状态

| 模块 | 能力 | 依赖条件 |
|------|------|----------|
| Persona & 会话 | 列表查询、校验 persona 后创建会话、记录消息 | 无外部依赖 |
| SSE 聊天 | `/api/stream/{sessionID}` 输出 start/delta/message/end 事件并保存对话 | 配置 Ark 模型凭证获取真实回复；缺省时退化为心跳流 |
| 语音 REST | `/api/speech/transcribe`、`/api/speech/synthesize` | 需配置火山语音 BaseURL 与密钥 |
| 语音 WebSocket | `/api/speech/ws/{sessionID}` 处理 audio/text/config 消息，串联 ASR→AI→TTS | 需同时配置 Ark 与语音凭证；缺失时路由返回 501 |
| 测试回归 | `go test ./...` 覆盖聊天、流式、语音关键逻辑 | Go 1.24+ 环境 |

## 🏛️ 架构设计

### 分层架构

```
cmd/api/            # 程序入口
├── main.go         # 服务装配与启动

internal/
├── model/          # 数据模型层
│   ├── persona/   # 角色模型
│   ├── chat/      # 聊天模型
│   └── speech/    # 语音模型
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
├── middleware/    # 中间件
├── config/        # 配置管理
└── pkg/
    └── utils/     # 工具函数

configs/           # 配置文档
```

### 架构原则

- **职责分离**: 每层只关注自己的责任
- **依赖方向**: Handler → Service → Model
- **纯净分层**: Service层不包含HTTP代码，Model层只定义数据结构
- **依赖注入**: 避免全局变量，通过构造函数注入依赖

## 🚀 快速开始

### 环境准备

- Go 1.24+
- 可选：`golangci-lint`、Docker

### 安装与运行

```bash
# 1. 克隆项目
git clone <repo-url>
cd z-tavern/backend

# 2. 安装依赖
make install-deps

# 3. 配置环境变量（可选）
cp .env.example .env
# 编辑 .env 文件，配置 AI 和语音服务参数

# 4. 运行开发服务
make run
# 或指定端口
PORT=3000 go run ./cmd/api
```

首次启动时若缺少外部凭证，系统会按以下策略降级：

- **未配置 Ark**: AI 服务跳过初始化，SSE/WebSocket 仅返回心跳或错误提示
- **未配置语音服务**: 语音 REST 与 WebSocket 返回 501 Not Implemented

建议先完成基础会话与 SSE 测试，待获取真实密钥后再启用语音与 WebSocket 链路。

## ⚙️ 配置说明

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | 监听端口 | `8080` |
| `ARK_API_KEY` | Volcengine Ark API密钥 | `""` |
| `Model` | AI模型ID | `""` |
| `ARK_BASE_URL` | Ark服务地址 | `https://ark.cn-beijing.volces.com/api/v3` |
| `ARK_STREAM` | 是否启用流式回复 | `true` |

### 语音服务配置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `SPEECH_BASE_URL` | 语音服务 API 基础地址 | `""` |
| `SPEECH_API_KEY` | 语音服务API密钥 | `""` |
| `SPEECH_ASR_MODEL` | 语音识别模型 | `""` |
| `SPEECH_TTS_VOICE` | 语音合成声音 | `""` |

更多配置选项参见 `configs/README.md`。

## 📡 API 接口

### 核心端点

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/personas` | 获取可用角色列表 |
| `POST` | `/api/session` | 创建新会话 |
| `POST` | `/api/messages` | 发送消息 |
| `GET` | `/api/stream/{sessionID}` | SSE流式AI响应 |

### 语音功能

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/speech/transcribe` | 语音转文字 |
| `POST` | `/api/speech/synthesize` | 文字转语音 |
| `GET` | `/api/speech/ws/{sessionID}` | WebSocket语音交互 |

### 使用示例

```bash
# 创建会话
curl -X POST http://localhost:8080/api/session \
  -H "Content-Type: application/json" \
  -d '{"personaId": "harry-potter"}'

# 发送消息
curl -X POST http://localhost:8080/api/messages \
  -H "Content-Type: application/json" \
  -d '{"sessionId": "xxx", "sender": "user", "content": "你好"}'

# SSE流式响应
curl "http://localhost:8080/api/stream/xxx?message=你好"

# WebSocket 语音交互（示例使用 wscat）
wscat -c ws://localhost:8080/api/speech/ws/xxx
# 发送配置
> {"type":"config","data":{"language":"zh-CN","ttsEnabled":true}}
# 发送文本消息
> {"type":"text","data":{"text":"给我讲个故事"}}
```

> 提示：语音交互需要将音频数据编码为 Base64 放入 `audioData` 字段，或按客户端协议拆分二进制帧。

## 🛠️ 开发指南

### 常用命令

```bash
# 开发
make run          # 启动开发服务
make build        # 构建可执行文件
make test         # 运行测试
make race         # 竞态检测
make lint         # 代码检查
make fmt          # 格式化代码

# CI检查
make ci           # 完整检查流程
```

### 添加新功能

1. **定义Model**: 在 `internal/model/` 创建数据模型
2. **实现Service**: 在 `internal/service/` 编写业务逻辑
3. **创建Handler**: 在 `internal/handler/` 处理HTTP请求
4. **注册路由**: 在handler中注册路由，在router.go中组装
5. **编写测试**: 为Service和Handler编写单元测试

### 代码规范

- 使用 `go fmt` 格式化代码
- 遵循 Go 命名约定
- Service构造函数命名为 `NewService`
- Handler构造函数命名为 `New`
- 错误变量使用 `ErrXXX` 模式

## 🧪 测试

```bash
# 运行所有测试（受限环境可指定本地缓存目录）
GOCACHE=$(pwd)/.gocache go test ./...

# 带覆盖率的测试
make test-coverage

# 竞态检测
make race

# 性能测试
make bench
```

> 测试结束后可删除 `.gocache` 以保持工作区整洁。

## 📦 部署

### Docker部署

```bash
# 构建镜像
make docker-build

# 运行容器
make docker-run
```

### 二进制部署

```bash
# 构建
make build

# 运行
./bin/z-tavern-backend
```

## 🤝 贡献指南

1. Fork 本仓库
2. 创建特性分支: `git checkout -b feature/amazing-feature`
3. 提交更改: `git commit -m 'Add amazing feature'`
4. 推送分支: `git push origin feature/amazing-feature`
5. 创建 Pull Request

请确保：
- 遵循代码规范
- 编写测试
- 更新文档
- CI检查通过

详细开发规范参见 [AGENTS.md](./AGENTS.md)。

## 📄 许可证

本项目采用 [MIT License](LICENSE)。

## 🏗️ 技术栈

- **框架**: Go 1.24 + Chi Router
- **AI**: CloudWeGo Eino + Volcengine Ark
- **语音**: Volcengine Speech API
- **架构**: 分层架构 + 依赖注入
- **通信**: HTTP + Server-Sent Events + WebSocket

## 📈 项目状态

- ✅ 基础聊天功能
- ✅ AI集成与流式响应
- ✅ 语音识别与合成
- ✅ 分层架构重构
- 🚧 认证与权限
- 🚧 数据持久化
- 🚧 微服务拆分
