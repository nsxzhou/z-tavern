# Z Tavern 前端对接指南

面向前端工程师，帮助你快速接入 Z Tavern 后端，涵盖会话、AI 流式回复、语音识别/合成以及情绪联动的所有接口。本文档基于仓库 `github.com/zhouzirui/z-tavern/backend` 当前实现编写，接口实际行为可在 `internal/handler` 与 `internal/service` 目录中核对。

---

## 1. 基础信息

- **默认基址**：`http://localhost:8080`
- **统一前缀**：所有 HTTP 接口都挂载在 `/api` 下
- **认证**：当前阶段无需鉴权
- **启用条件**：
  - AI 回复：需要配置 Ark 模型与凭证，否则 `/api/stream/*` 会返回 `503 ai streaming unavailable`
  - 情绪分析：`AI_EMOTION_LLM_ENABLED=true` 时启用 LLM 情绪推断；否则回退到启发式
  - 语音能力：需配置 `SPEECH_APP_ID` 及 Token/AKSK；未配置时 REST 端点返回 500，WebSocket 退化为 501

---

## 2. 快速接入流程

1. **获取角色列表** → `GET /api/personas`
2. **创建会话** → `POST /api/session`
3. **拉起 SSE 或语音 WebSocket**，并发送用户输入
4. **渲染 AI 回复**，同步保存消息（SSE/WebSocket 已自动写入，可按需再调用 `/api/messages`）
5. **可选：语音链路**
   - 上传录音转文字 → `POST /api/speech/transcribe`
   - 文本转语音 → `POST /api/speech/synthesize` 或 WebSocket `tts`
6. **UI 情绪联动**：监听 SSE/WebSocket 下发的 `emotion` 数据

---

## 3. 全局约定

| 项                    | 说明 |
|-----------------------|------|
| 协议与编码            | HTTP/1.1，默认 `application/json`; 上传音频使用 `multipart/form-data` |
| 时间格式              | 所有时间字段均为 ISO 8601 UTC（Go `time.Time` 默认序列化） |
| 错误返回              | 统一结构 `{ "error": "message" }`；状态码依据场景返回 4xx/5xx |
| 会话标识              | `sessionId` 为后端生成的 UUID 字符串，前端需在整个对话周期内携带 |
| 语音格式              | 默认使用 `mp3`；上传支持 `mp3/wav/webm/m4a/aac` |
| 连接超时与心跳        | SSE 需支持断线重连；语音 WebSocket 每 54 秒发送 Ping，客户端需回 Pong |

---

## 4. 数据模型速览

| 模型     | 关键字段 | 说明 |
|----------|----------|------|
| Persona  | `id`, `name`, `title`, `tone`, `promptHint`, `voiceId` | 角色元数据，位于 `internal/model/persona` |
| Session  | `id`, `personaId`, `createdAt` | 会话信息，`POST /api/session` 创建 |
| Message  | `id`, `sessionId`, `sender`, `content`, `emotion`, `createdAt` | 聊天记录，SSE/WebSocket 会自动写入 |
| Emotion  | `emotion`, `scale`, `confidence`, `style` | 情绪分析结果，来自 `internal/service/emotion` |
| ASR/TTS  | 详见语音章节 | 请求/响应定义位于 `internal/model/speech` |

---

## 5. 基础 REST 接口

### 5.1 列出角色 `GET /api/personas`

- **请求**：无参数
- **响应**：`200 OK`, `Persona[]`
- **用途**：展示可选角色；创建会话需传入 `personaId`
- **注意事项**：无分页，数量通常较小；可自行缓存
- **示例**：
```ts
const personas = await fetch(`${API_BASE_URL}/api/personas`).then(res => {
  if (!res.ok) throw new Error('加载角色失败');
  return res.json() as Promise<Persona[]>;
});
```

### 5.2 创建会话 `POST /api/session`

- **请求体**：`{ "personaId": string }`
- **响应**：`201 Created`, `Session`
- **错误**：
  - `400`：缺少 `personaId` 或 persona 不存在
- **提示**：建议在前端缓存该会话 ID，后续请求均需携带

### 5.3 自助写入消息 `POST /api/messages`

- **请求体**：
  ```json
  {
    "sessionId": "uuid",
    "sender": "user" | "assistant",
    "content": "文本",
    "emotion": "可选情绪标签"
  }
  ```
- **响应**：`202 Accepted`, `{ "status": "queued" }`
- **使用场景**：需要手动补写历史（例如 SSE 断线或离线同步）
- **注意**：SSE/WebSocket 已自动写入消息，一般无需额外调用

---

## 6. SSE 文本流式接口 `GET /api/stream/{sessionID}`

- **请求条件**：
  - Query 参数必须包含 `message`（用户输入）
  - AI 服务未启用时返回 `503 ai streaming unavailable`
- **SSE 事件类型**：

| event    | content 字段含义 | 说明 |
|----------|------------------|------|
| `start`  | 提示文案         | 请求已受理，可显示加载文案 |
| `delta`  | 文本片段         | 当启用流式模型时逐块返回 token |
| `message`| 完整回复文本     | 在非流式模式下直接返回 |
| `emotion`| JSON 字符串      | 形如 `{"emotion":"happy","scale":3.5,"confidence":0.82}` |
| `end`    | `finished=true`  | 本次生成结束，可关闭连接 |
| `error`  | `error` 字段     | 发生异常，需提示用户 |

- **前端示例**：
```ts
const url = new URL(`${API_BASE_URL}/api/stream/${sessionId}`);
url.searchParams.set('message', input);
const sse = new EventSource(url);

sse.onmessage = evt => {
  const payload = JSON.parse(evt.data) as StreamResponse;
  switch (payload.event) {
    case 'delta': appendDelta(payload.content); break;
    case 'message': finalizeText(payload.content); break;
    case 'emotion': cacheEmotion(JSON.parse(payload.content)); break;
    case 'end': sse.close(); break;
  }
};

sse.onerror = () => {
  sse.close();
  notify('流式接口异常，请重试');
};
```
- **注意事项**：
  - emotion 事件可驱动 UI 表情或下游 TTS
  - SSE 断线后可按需重放（需重新发送 `message`）

---

## 7. 语音 REST 接口

### 7.1 语音转文本 `POST /api/speech/transcribe`

- **请求格式**：`multipart/form-data`
  | 字段       | 类型                | 是否必填 | 说明 |
  |------------|---------------------|----------|------|
  | `audio`    | 二进制文件          | 是       | 录音文件，扩展名用于推断格式 |
  | `sessionId`| 文本                | 否       | 未提供时后端使用 `default` |
  | `language` | 文本                | 否       | 默认 `zh-CN` |
- **响应**：`200 OK`, `ASRResponse`（包含 `text`, `confidence`, `duration` 等）
- **错误**：
  - `400`：缺少文件或表单解析失败
  - `500`：调用语音服务出错

### 7.2 语音转文本（带会话）`POST /api/speech/transcribe/{sessionID}`

- **路径参数**：强制覆盖表单内的 `sessionId`
- **适用场景**：需要确保与现有会话绑定，避免前端漏传

### 7.3 文本转语音 `POST /api/speech/synthesize`

- **请求体**：
  ```json
  {
    "sessionId": "可选",
    "text": "待合成文本",
    "voice": "可选音色",
    "speed": 1.0,
    "volume": 1.0,
    "format": "mp3",
    "language": "zh-CN",
    "enableEmotion": true,
    "emotion": "happy",
    "emotionScale": 3.5
  }
  ```
- **响应**：
  - 若返回二进制音频：`Content-Type: audio/{format}`，需将 `res.blob()` 转成 URL
  - 否则返回 `TTSResponse` JSON（包含 `audioData` 或错误信息）
- **情绪联动**：
  - 只有当 `enableEmotion=true` 且 `emotion` 非空时才会向火山引擎传递情绪参数
  - 建议复用最近一次 SSE/WebSocket 下发的情绪结果

### 7.4 文本转语音（带会话）`POST /api/speech/synthesize/{sessionID}`

- **说明**：路径参数覆盖请求体的 `sessionId`
- **用途**：明确绑定到某个对话会话，便于服务端日志追踪

### 7.5 语音服务健康检查 `GET /api/speech/health`

- **响应**：`200 OK`, `{ "status": "healthy", "service": "speech" }`
- **用途**：前端启动时检测语音链路是否可用

---

## 8. 语音 WebSocket 接口 `GET /api/speech/ws/{sessionID}`

### 8.1 建连要求

- 需同时启用 AI、语音、聊天服务，否则返回 `501 speech websocket not available`
- 成功握手后立即收到 `type: "connected"` 消息
- 服务器每 54 秒发送 `Ping`，客户端需 `Pong` 响应并刷新超时时间

### 8.2 入站消息格式（前端 → 后端）

所有消息外层统一：`{ "type": string, "sessionId": string, "data": any, "timestamp": number }`

| type      | data 结构 | 说明 |
|-----------|-----------|------|
| `audio`   | `{ audioData, format, language, isFinal, chunkIndex }` | 发送音频分片，`audioData` 为 Base64 |
| `text`    | `{ text, isFinal, confidence? }` | 直接发送文本消息 |
| `config`  | `{ personaId?, language?, voice?, asrEnabled?, ttsEnabled?, streamMode? }` | 动态配置本次会话 |

### 8.3 出站消息格式（后端 → 前端）

包裹形式：`{ "type": "result", "sessionId": string, "data": { ... }, "timestamp": number }`

| data.type    | 负载字段 | 说明 |
|--------------|----------|------|
| `connected`  | `{ persona, language }` | 握手成功通知 |
| `asr`        | `{ text, confidence, isFinal }` | 语音识别结果 |
| `user`       | `{ text }` | 回显保存的用户文本 |
| `ai_delta`   | `{ text }` | 流式 AI 增量输出 |
| `ai`         | `{ text, isFinal }` | AI 最终回复 |
| `emotion`    | `{ emotion, scale, confidence }` | 情绪分析结果 |
| `tts`        | `{ audioData, format, isFinal, emotion?, scale?, error? }` | 合成后的音频数据 |
| `config`     | `{ persona, language, voice, asr, tts, streamMode }` | 配置变更回执 |
| `error`      | `{ message }` | 处理失败信息 |

### 8.4 情绪驱动的 TTS

- WebSocket 模式下，后端会自动调用 `ComputeEmotionParameters` 将最新情绪映射为 TTS 参数
- 若当前音色不支持情绪或情绪为 `neutral`，会自动关闭情绪开关

---

## 9. 情绪与音色联动说明

1. **SSE**：在 `emotion` 事件中得到 `{emotion, scale, confidence}`
2. **REST 合成**：前端需要显式传入 `enableEmotion + emotion + emotionScale`
3. **WebSocket 合成**：后端根据最新情绪自动填写 TTS 请求
4. **降级策略**：若音色不支持情绪或情绪分析关闭，后端会忽略情绪参数并保持中性语气

---

## 10. 调试与故障排查

- **常见 HTTP 状态码**：
  - `400`：参数缺失或格式错误
  - `404`：会话不存在
  - `500`：调用外部服务失败（语音、AI）
  - `503`：AI 尚未启用
  - `501`：语音 WebSocket 未启用
- **日志定位**：后端控制台会打印 `[stream]`、`[speech]`、`[websocket]` 前缀日志，可据此排查
- **建议工具**：
  - SSE 调试可使用浏览器或 `curl -N`
  - WebSocket 调试可使用 `wscat` / Chrome DevTools
- **情绪验证**：确保环境变量开启情绪 LLM 并观察 SSE/WebSocket 是否收到 `emotion`

---

如需进一步确认接口细节，可直接查阅对应 Handler（`internal/handler/...`）与 Service 实现，或与服务端同学沟通配置开关。祝集成顺利！
