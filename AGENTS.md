# Agent Handbook

## Project Snapshot
- **Product**: Z Tavern · Liquid Glass 前端体验层，提供角色驱动对话与语音交互的可视界面
- **Stack**: React 19 + Vite 7 + TypeScript 5.8 + Tailwind CSS 3
- **Goal**: 连接后端酒馆服务，支持角色列表、会话流式更新、语音输入与合成播放

## Core Experience
### 角色发现与筛选
- `useChatOrchestrator` 首次启动时拉取 `/personas`，并缓存到状态机
- `PersonaSidebar` 支持关键词搜索、标签过滤、随机推荐，展示心情指标
- `PersonaProfile` 展示背景、风格、提示语，向 `ChatComposer` 提供快捷提问

### 会话与流式响应
- `apiClient.createSession` 创建 `/session`，`ChatTimeline` 展示消息流
- `createStreamController` 通过 SSE 订阅 `/stream/:sessionId`，处理 `start/delta/end` 事件
- 会话状态保存在 `ConversationMap`，确保多角色切换保留上下文

### 语音管线
- `createSpeechSocket` 建立 WebSocket，向 `/speech/socket` 发送音频块并接收 ASR/AI/TTS 信息
- 麦克风数据在前端完成降采样、PCM16 编码与 WAV 封装后发送
- `ChatComposer` 控制录音生命周期，`ChatTimeline` 播放 `synthesize` 返回的音频 Blob

## Frontend Architecture
- `src/api`: REST、SSE 与语音接口封装（`apiClient`、`createStreamController`、`createSpeechSocket`）
- `src/components/chat`: 角色侧栏、会话时间线、输入器、角色档案等 UI 模块
- `src/hooks`: `useChatOrchestrator` 负责数据获取、状态管理、语音与流控制
- `src/context`: `ThemeProvider` 提供暗亮主题切换，并持久化到 `localStorage`
- `src/utils`: 角色标签生成、情绪评估、提示语拼接等纯函数工具

## Styling System
- Tailwind 配置在 `tailwind.config.js`，包含 `ztavern-*` 设计令牌与玻璃拟态背景
- `index.css` 注入基础玻璃效果、动画、滚动条样式；保持两空格缩进
- UI 组件优先组合 utility class，复杂样式封装为 CSS 自定义类

## Environment & Integration
- 默认 API 基址 `VITE_API_BASE_URL` → `http://localhost:8080/api`，可在 `.env.local` 覆盖
- 后端需实现：`/personas`、`/session`、`/messages`、`/stream/:id`、`/speech/transcribe`、`/speech/synthesize`、`/speech/health`、`/speech/socket`
- 语音流功能通常要求 HTTPS 或 `localhost`，确保浏览器可以访问麦克风

## Development Workflow
- 推荐 Node.js ≥ 20；执行 `npm install` 安装依赖
- 常用脚本：`npm run dev`（热更新预览）、`npm run build`（tsc + 产物）、`npm run lint`（ESLint）
- 本地调试语音：确认麦克风权限、检查 `speechStatusMessage` 与 `speechStatusTone`

## QA Expectations
- 人物列表加载稳定，切换角色时会话上下文保持
- 消息发送会先排队（`MessageQueuedResponse`），随后收到 SSE delta 与完整响应
- 语音开启后能看到连接/录音/转写状态，TTS 播放按钮可复播
- 提交前至少运行 `npm run lint` 与 `npm run build`，并执行一次手工对话/语音冒烟测试

## Collaboration Notes
- 通过 `apiClient` 与 hooks 管理远端交互，避免在组件内直接 `fetch`
- 扩展消息结构请同步更新 `ConversationState` 与渲染逻辑
- 保持类型安全：新增 API 时补充 `src/api/types.ts`
- 遵循 Tailwind 令牌与 `ThemeProvider` 机制，保证暗亮主题一致性
