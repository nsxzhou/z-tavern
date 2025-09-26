# Z Tavern 后端接口前端集成手册

> 本文档面向前端开发者，详尽说明如何与 Z Tavern 后端服务交互。内容覆盖 REST 接口、Server-Sent Events (SSE) 流、语音 REST/WebSocket 管道、字段语义、错误处理以及前端实现示例。所有信息均基于仓库 `github.com/zhouzirui/z-tavern/backend` 的最新代码（参考路径如 `internal/handler/router.go:28` 等）。

## 1. 运行条件与全局约定

### 1.1 服务基础信息
- **基础 URL**：默认监听 `http://localhost:8080`，可通过环境变量 `PORT` 覆盖（`internal/config/config.go:55`）。部署时请将 `API_BASE_URL` 配置到前端环境中。
- **前缀**：所有 HTTP 接口均挂载在 `/api` 前缀下（`internal/handler/router.go:42`）。例如获取角色列表的完整路径是 `/api/personas`。
- **CORS**：服务端已开启宽松跨域策略（`internal/middleware/cors.go:9`），允许任意来源进行 `GET/POST/PUT/DELETE/OPTIONS` 请求，前端无需额外代理。

### 1.2 功能开关与前置条件
- **AI 服务**：若要使用流式或非流式生成回复，需正确配置 ARK 凭证（`ARK_API_KEY` + `Model` 或 `ARK_ACCESS_KEY/ARK_SECRET_KEY`，见 `internal/config/config.go:103`）。未配置时 `/api/stream/{sessionID}` 会退化为心跳流。
- **语音服务**：需提供 `SPEECH_APP_ID` 与 `SPEECH_ACCESS_TOKEN`（或兼容的 AK/SK），否则 `/api/speech/*` 依旧可访问但会返回 501（WebSocket）或 `speechSvc` 为空导致 500。参见 `cmd/api/main.go:59-86`。
- **Streaming 选项**：`ARK_STREAM` 控制是否启用 AI 流式输出；若关闭则 `/api/stream/{sessionID}` 返回一次性消息（`internal/service/ai/llm_service.go:48`）。

### 1.3 通用请求与响应约定
- **请求体编码**：除语音上传使用 `multipart/form-data` 外，其他接口均采用 `application/json`。
- **时间戳**：所有时间字段使用 ISO 8601 UTC 字符串（Go `time.Time` 默认序列化）。
- **错误格式**：统一返回 `{"error": "message"}`（参见 `pkg/utils/response.go:20` 及各 handler），状态码依场景而定。
- **成功响应**：JSON 结构随模型变化；语音合成成功可能返回二进制音频流（`internal/handler/speech/handler.go:137`）。

### 1.4 数据模型概览
| 模型 | 关键字段 | 对应结构体 |
| ---- | -------- | ---------- |
| Persona | `id`, `name`, `title`, `promptHint`, 可选 `voiceId` 等 | `internal/model/persona/persona.go` |
| Session | `id`, `personaId`, `createdAt` | `internal/model/chat/session.go` |
| Message | `id`, `sessionId`, `sender`, `content`, `emotion`, `createdAt` | `internal/model/chat/message.go` |
| ASR/TTS | 见语音章节 | `internal/model/speech/*.go` |

建议在前端定义 TypeScript 接口与上述字段保持一致，避免字段拼写错误。

## 2. 前端交互总流程
1. **拉取角色**：`GET /api/personas` 显示可选角色。
2. **创建会话**：`POST /api/session` 并记录后端生成的 `sessionId`。
3. **发送消息**：
   - 若只需保存：`POST /api/messages`。
   - 若需实时回复：通过 `GET /api/stream/{sessionId}?message=` 发起 SSE，或使用语音 WebSocket。
4. **处理 AI 响应**：监听 SSE `delta/message/end` 或 WebSocket `result` 事件。
5. **语音能力（可选）**：使用 `/api/speech/*` 进行 ASR/TTS，或建立 `/api/speech/ws/{sessionId}` 进行音视频回路。
6. **持久化与 UI 更新**：将消息列表与音频资源同步到状态管理。

以下章节逐一拆解各接口。

## 3. REST 接口详解

### 3.1 获取角色列表：`GET /api/personas`
- **位置**：`internal/handler/persona/handler.go:25`。
- **请求**：无参数。支持缓存（可由前端自行控制）。
- **响应**：`200 OK`，返回 `Persona[]`。示例：
```json
[
  {
    "id": "harry-potter",
    "name": "哈利·波特",
    "title": "勇敢的魔法师",
    "tone": "冒险、温暖、友善",
    "promptHint": "保持少年感与忠诚...",
    "openingLine": "欢迎来到霍格沃茨的角落...",
    "voiceId": "hogwarts-young-hero",
    "traits": ["勇敢", "忠诚"],
    "expertise": ["防御黑魔法", "魁地奇"]
  }
]
```
- **前端示例**：
```ts
const personas = await fetch(`${API_BASE_URL}/api/personas`, {
  method: 'GET',
}).then(res => {
  if (!res.ok) throw new Error('加载 personas 失败');
  return res.json() as Promise<Persona[]>;
});
```
- **用途**：用于渲染角色选择，下游请求需携带 `personaId`。

### 3.2 创建会话：`POST /api/session`
- **位置**：`internal/handler/chat/handler.go:31`。
- **请求体**：`{"personaId": string}`。
- **校验**：
  - 缺少 `personaId` → `400 Bad Request`（`"personaId is required"`）。
  - 无效角色 → `400 Bad Request`（`"persona not found"`）。
- **响应**：`201 Created`，返回 `Session`。
```json
{
  "id": "21f1df4d-...",
  "personaId": "socrates",
  "createdAt": "2024-05-30T12:03:30.123456Z"
}
```
- **前端示例**：
```ts
async function createSession(personaId: string): Promise<Session> {
  const res = await fetch(`${API_BASE_URL}/api/session`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ personaId }),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error ?? 'Session 创建失败');
  }
  return res.json();
}
```
- **注意**：后端会预创建空的消息数组，之后的消息调用需复用同一 `sessionId`。

### 3.3 保存消息：`POST /api/messages`
- **位置**：`internal/handler/chat/handler.go:49`。
- **请求体**：
```json
{
  "sessionId": "uuid",
  "sender": "user" | "assistant",
  "content": "纯文本",
  "emotion": "可选情绪标签"
}
```
- **校验**：
  - JSON 解析失败 → `400`（`"invalid request body"`）。
  - 未找到会话 → `404`（`"session not found"`）。
  - 其他内部错误 → `500`（同样返回 `{ "error": "..." }`）。
- **响应**：`202 Accepted` + `{ "status": "queued" }`。
- **前端用途**：存档消息或在收到 SSE 后补写消息历史。注意返回状态为 `202`，需特殊判断。
- **示例**：
```ts
await fetch(`${API_BASE_URL}/api/messages`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ sessionId, sender: 'user', content }),
});
```

### 3.4 文本流式回复：`GET /api/stream/{sessionID}`
- **位置**：`internal/handler/router.go:52` + `internal/handler/stream/handler.go`。
- **请求方式**：`GET`，必须附加查询参数 `message`。示例：`/api/stream/21f1...?message=你好`。
- **预置逻辑**：
  - 当 `aiService` 不可用或 `message` 为空时，退化到心跳 SSE（`handleHeartbeatStream`）。
  - 正常情况下推送 AI 回复并回写聊天记录。
- **SSE 事件格式**：所有事件以 JSON 作为 `data`，其中 `event` 字段标识类型。
  - `start`：收到请求后立即发送，`content` 提示生成中。
  - `delta`：当启用流式模型时，每个增量 token 触发。
  - `message`：完整回复文本。
  - `end`：生成完成。
  - `error`：处理失败时返回，`error` 字段含信息。
- **心跳模式**（无 AI）：发送 `{"event": "status"}`，随后每 8 秒发送 `"heartbeat"`（`internal/handler/router.go:88`）。

- **前端实现要点**：
  - **EventSource**：
    ```ts
    const url = new URL(`${API_BASE_URL}/api/stream/${sessionId}`);
    url.searchParams.set('message', input);
    const sse = new EventSource(url);

    sse.onmessage = (event) => {
      const payload = JSON.parse(event.data) as StreamResponse;
      switch (payload.event) {
        case 'delta': appendPartial(payload.content); break;
        case 'message': finalize(payload.content); break;
        case 'end': sse.close(); break;
      }
    };
    sse.onerror = () => { sse.close(); showError(); };
    ```
  - **Fetch + ReadableStream**：适用于需附加 Header 的场景，可用 `fetch` 后逐行解析 `data:` 前缀。
  - **容错**：若收到 `error`，应提示用户并允许重试。可根据 HTTP 状态判定是否重连。

- **副作用**：接口内部会把用户消息与助手回复分别写入聊天服务（`SaveMessage` 调用位于 `internal/handler/stream/handler.go:66` 和 :91）。因此前端无需额外写入历史。

### 3.5 语音转文本：`POST /api/speech/transcribe`
- **位置**：`internal/handler/speech/handler.go:50`。
- **请求**：`multipart/form-data`
  - 字段 `audio`：音频文件二进制（必填）。
  - 字段 `sessionId`：可选；未提供时默认 `default`。
  - 字段 `language`：可选，默认 `zh-CN`。
- **自动推断格式**：根据文件扩展名推断 `format`（`inferAudioFormat` 支持 `mp3/wav/webm/m4a/aac`）。
- **响应**：`200 OK`，返回 `ASRResponse`：
```json
{
  "sessionId": "test",
  "text": "识别结果",
  "confidence": 0.91,
  "duration": 2300,
  "createdAt": "2024-05-30T12:00:00Z"
}
```
- **异常**：解析失败、缺少文件 → `400`；识别失败 → `500`。
- **前端示例**：
```ts
const form = new FormData();
form.append('audio', file);
form.append('sessionId', sessionId);
form.append('language', 'zh-CN');
const res = await fetch(`${API_BASE_URL}/api/speech/transcribe`, { method: 'POST', body: form });
```
- **备注**：若希望与会话绑定，可改用 `/api/speech/transcribe/{sessionId}`（下节）。

### 3.6 带会话的语音转文本：`POST /api/speech/transcribe/{sessionID}`
- **位置**：`internal/handler/speech/handler.go:55`。
- **区别**：路径参数覆盖任何表单内传入的 `sessionId`，避免前端漏填（测试验证于 `handler_test.go:39`）。

### 3.7 文本转语音：`POST /api/speech/synthesize`
- **位置**：`internal/handler/speech/handler.go:74`。
- **请求体**：`application/json`
```json
{
  "sessionId": "可选",
  "text": "需要合成的文本",
  "voice": "可选音色",
  "speed": 1.0,
  "volume": 1.0,
  "format": "mp3",
  "language": "zh-CN"
}
```
- **必填**：`text`。若未指定 `sessionId`，后端默认 `default`。
- **响应**：
  - 若 `speechSvc` 返回 `AudioData` 字节 → 后端以 `audio/{format}` 输出，附带 `Content-Disposition` 头（见 `handler.go:137`）。
  - 若无音频 → 返回 JSON `TTSResponse`（包含 `audioUrl` 等）。
- **前端处理技巧**：
```ts
const res = await fetch(`${API_BASE_URL}/api/speech/synthesize`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ sessionId, text, voice: persona.voiceId, format: 'mp3' }),
});
const contentType = res.headers.get('Content-Type') ?? '';
if (contentType.startsWith('audio/')) {
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  audioEl.src = url;
} else {
  const payload = await res.json();
  // 处理 payload.audioUrl 或错误
}
```
- **变体**：`POST /api/speech/synthesize/{sessionID}` 会强制使用路径中的 `sessionID`。

### 3.8 语音健康检查：`GET /api/speech/health`
- **位置**：`internal/handler/speech/handler.go:162`。
- **响应**：`200` + `{ "status": "healthy", "service": "speech" }`。可用于前端启动时探测语音服务是否上线。

## 4. 语音 WebSocket 详解：`GET /api/speech/ws/{sessionID}`

### 4.1 建立连接
- **位置**：`internal/handler/speech/websocket.go:54`。
- **前置条件**：AI、语音、聊天服务均已初始化（`websocketAvailable` 返回 true）。否则该路径直接返回 `501 Not Implemented`（`handler_test.go:68`）。
- **握手**：标准 WebSocket，建议在 `wss` 环境中部署。连接成功后，后端立即发送 `Type: "result"`，数据中 `type: "connected"`。

### 4.2 入站消息格式（前端 → 后端）
所有消息封装为：
```json
{
  "type": "audio" | "text" | "config",
  "sessionId": "可选，必须与 URL 保持一致",
  "data": { ... },
  "timestamp": 1717040000
}
```

#### 4.2.1 音频消息 `type = "audio"`
- 对应结构：`AudioMessage`（`websocket.go:86`）。
- `data` 字段应包含：
```json
{
  "audioData": "<base64 编码的音频块>",
  "format": "pcm" | "wav" | "mp3" ...,
  "language": "zh-CN",
  "isFinal": true | false,
  "chunkIndex": 0
}
```
- **要求**：`audioData` 必须是 base64 字符串，服务端会自动解码为字节流（`handleAudioMessage`）。当 `isFinal=true` 或链路处于非流式模式（`streamMode=false`）时触发识别。

#### 4.2.2 文本消息 `type = "text"`
- 对应结构：`TextMessage`。
- `data` 示例：`{"text": "你好", "isFinal": true, "messageType": "manual"}`。
- 用于直接提交文本而不上传音频。

#### 4.2.3 配置消息 `type = "config"`
- 对应结构：`ConfigMessage`。
- 支持字段：
  - `personaId`：切换 AI 角色。
  - `language`：更新 ASR/TTS 语言。
  - `voice`：指定特定音色。
  - `asrEnabled` / `ttsEnabled` / `streamMode`：开关布尔值。
- 服务端会回写 `result`，其中 `type: "config"` 反映当前状态。

### 4.3 出站消息格式（后端 → 前端）
- 封装结构：`outgoingMessage`（`websocket.go:110`）。所有成功消息的 `Type` 固定为 `"result"`，错误为 `"error"`。
- `data.type` 取值说明：
  - `connected`：建立连接成功。
  - `asr`：ASR 结果，含 `text`、`confidence`、`isFinal`。
  - `user`：回显用户文本。
  - `ai_delta`：AI 流式增量片段。
  - `ai`：完整 AI 回复，`isFinal: true`。
  - `tts`：TTS 输出，包含 `audioData`（base64）与 `format`。
  - `config`：配置更新确认。
- 错误消息：`{"Type":"error","Data":{"message":"..."}}`。
- **心跳**：服务端每 54 秒发送一次 `Ping`（`pingLoop`），前端需处理 `pong` 自动维持连接。

### 4.4 前端实现注意事项
1. 使用 `WebSocket` 原生 API：
```ts
const ws = new WebSocket(`${WS_BASE_URL}/api/speech/ws/${sessionId}`);
ws.onmessage = (event) => {
  const payload = JSON.parse(event.data) as ServerMessage;
  if (payload.Type === 'result') {
    const info = payload.Data as ResultPayload;
    switch (info.type) {
      case 'ai_delta': append(info.text); break;
      case 'tts': playBase64(info.audioData, info.format); break;
    }
  } else {
    console.error(payload.Data.message);
  }
};
```
2. 发送音频：录音后将 `ArrayBuffer` 转为 `Uint8Array`，再用 `btoa`/`Buffer.from(...).toString('base64')` 编码。
3. 若希望分片上传，记得在最后一个片段设置 `isFinal=true` 以触发识别。
4. 若语音服务不可用，HTTP 握手直接返回 501，需降级到纯文本模式。

## 5. 错误处理与重试策略
- **REST**：对 `res.ok` 进行判断，若失败解析 `{ error }` 并转化为用户友好的提示。
- **SSE**：若连接在 `eventSource.onerror` 中断，可退避重连；收到 `event: "error"` 时终止并提示。
- **WebSocket**：监听 `close` 事件，结合 `code` 判断是服务器主动断开还是网络问题。必要时指数退避重连，并在重连后发送 `config` 消息恢复上下文。
- **语音上传**：由于 `multipart` 请求可能较大，建议在前端限制音频长度并在进度条完成后再提交。

## 6. 测试与调试建议
1. **本地 Mock**：可通过 `curl` 验证接口：
   - `curl http://localhost:8080/api/personas`
   - `curl -X POST http://localhost:8080/api/session -H 'Content-Type: application/json' -d '{"personaId":"socrates"}'`
2. **SSE 调试**：使用浏览器 DevTools > Network > EventStream 观察事件序列。
3. **WebSocket 调试**：推荐使用 `wscat` / Chrome DevTools > Network > WS，直接发送 JSON 消息验证。
4. **日志定位**：后端在关键路径输出 `log.Printf`（例如 `websocket.go:145`/`187`），部署时应关注服务端日志了解语音链路状态。

## 7. 常见问题 FAQ
- **Q: 为什么 `/api/stream/{sessionId}` 只返回心跳？**
  - A: 检查 AI 凭证是否配置，或请求 URL 是否遗漏 `message` 参数。
- **Q: 语音接口返回 500？**
  - A: 查看后端日志确认是否无法连接火山引擎，或音频格式不受支持。必要时显式指定 `language/format`。
- **Q: WebSocket 收到 `session mismatch` 错误？**
  - A: 前端消息中的 `sessionId` 与 URL 不一致；保持两者一致或省略消息内的 `sessionId` 字段。
- **Q: TTS 返回 JSON 而不是音频？**
  - A: 说明服务端没有返回内联音频，可检查 `TTSResponse.AudioData` 是否为空；需要改用 `audioUrl` 字段或后端配置。

---

以上即为后端接口在前端的完整使用指南。建议在项目前期就将这些约定固化为 TypeScript 类型与封装函数，确保调用一致、错误集中处理。