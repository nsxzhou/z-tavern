# Z Tavern 运行指南与架构概览

## 如何运行程序

### 前置条件
- 已安装 Go 1.24 或更高版本、Node.js 20 或更高版本。
- 推荐在根目录执行 `git status` 确认工作区状态，必要时复制示例配置（例如 `backend/config.toml`）。
- 若需对接外部 AI/语音服务，请提前准备火山引擎 Ark、语音相关凭证，并通过环境变量注入。

### 后端（`backend/`）
1. 进入目录并安装依赖：
   ```bash
   cd backend
   make install-deps
   ```
2. 配置环境变量（示例）：
   ```bash
   export ARK_API_KEY="<your-ark-api-key>"
   export Model="ep-xxx"           # 默认模型 ID
   export PORT=3000                 # 可选，默认为 :8080
   export SPEECH_APP_ID="<speech-app-id>"
   export SPEECH_ACCESS_TOKEN="<speech-token>"
   ```
   - 未配置 Ark 相关变量时，AI 流式接口会返回 503。
   - 未配置语音凭证时，语音 WebSocket 返回 501，REST 语音接口会报业务错误。
3. 启动服务：
   ```bash
   make run
   # 或直接运行入口
   PORT=3000 go run ./cmd/api
   ```
4. 常用辅助命令：
   ```bash
   make test          # 运行单元测试
   make lint          # 静态分析（回退 go vet）
   make build         # 构建二进制到 bin/
   make race          # 竞态检测
   make ci            # fmt + vet + lint + test + race
   ```

### 前端（`frontend/`）
1. 安装依赖并启动开发服务器：
   ```bash
   cd frontend
   npm install
   npm run dev
   ```
2. 默认后端 API 地址为 `http://localhost:8080/api`，若有变动或使用代理，请在 `frontend/.env.local` 中设置：
   ```bash
   VITE_API_BASE_URL="http://localhost:3000/api"
   ```
3. 构建与校验命令：
   ```bash
   npm run build      # 产出生产构建
   npm run preview    # 本地预览 dist 产物
   npm run lint       # 运行 ESLint
   ```
4. 浏览器访问 `http://localhost:5173`（或 Vite 输出的地址），并允许麦克风权限以测试语音链路。

### 联调建议
- 确认后端已监听正确端口，并在浏览器 DevTools 的 Network 面板验证 `/api/personas`、`/api/stream/:sessionId` 是否可达。
- SSE/语音链路涉及长连接，建议在本地测试前关闭代理工具。
- 若需使用 Docker，可执行 `make docker-build`、`make docker-run`（后端）。

## 架构设计

### 系统概览
- 平台由后端 Go 服务与前端 React SPA 组成，核心能力包括角色化对话、SSE 流式回复以及语音识别/合成。
- 后端负责会话管理、AI 调度、语音管道与持久化；前端承担 persona 展示、消息编排、音频采集与播放。
- 数据流：用户选择角色 → 创建会话 → 发送文本/语音 → 后端调用 Doubao 获取回复并通过 SSE 推流 → 消息写入存储，必要时异步触发 TTS。

### 后端架构
- 分层目录：
  - `cmd/api/`：服务入口与依赖装配。
  - `internal/handler`：HTTP/WebSocket 路由与参数校验；细分 persona、chat、speech、stream 等模块。
  - `internal/service`：persona 会话、AI 编排、语音流程等业务逻辑；依赖模型与外部客户端。
  - `internal/model`：领域模型与数据结构定义。
  - `pkg/utils`：SSE 推送、响应写入等通用工具。
- 依赖方向保持 Handler → Service → Model，业务实现屏蔽 HTTP 细节，便于测试。
- 配置通过环境变量或 `config.toml` 注入，支持 Ark AI、语音、端口等开关。
- 支持 REST（会话/消息）、SSE（实时回复）与 WebSocket（语音全链路），并通过 Makefile 提供测试、构建、部署脚本。

### 前端架构
- 技术栈：Vite + React + TypeScript + Tailwind CSS。
- 目录要点：
  - `src/api`：封装 REST、SSE、语音 WebSocket 客户端。
  - `src/components/chat`：角色面板、对话时间线、输入区等 UI 组件。
  - `src/hooks/useChatOrchestrator`：集中管理 persona、会话、流式状态、语音状态。
  - `src/context`：主题上下文与本地存储同步。
  - `src/utils/persona.ts`：角色元数据工具函数。
- UI 采用玻璃拟态风格（Glassmorphism），通过 Tailwind Token 控制主题；支持深浅色切换。

---
