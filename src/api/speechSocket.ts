import { apiBaseUrl } from './client'
import type {
  SpeechSocketErrorPayload,
  SpeechSocketIncomingMessage,
  SpeechSocketInfoEventType,
  SpeechSocketInfoPayload,
  SpeechSocketOutgoingMessage,
  SpeechSocketRawIncomingMessage,
} from './types'

export interface SpeechSocketHandlers {
  onOpen?: () => void
  onMessage?: (message: SpeechSocketIncomingMessage) => void
  onError?: (event: Event) => void
  onClose?: (event: CloseEvent) => void
}

export interface SpeechSocketConnection {
  send: (message: SpeechSocketOutgoingMessage) => void
  close: () => void
  readyState: () => number
}

const buildSpeechSocketUrl = (sessionId: string): string => {
  const base = apiBaseUrl?.trim() ?? ''
  const ensureAbsolute = () => {
    if (!base) {
      if (typeof window === 'undefined') {
        throw new Error('无法解析语音服务地址：缺少 API 基础地址')
      }
      return new URL('/api', window.location.origin)
    }
    try {
      return new URL(base)
    } catch (error) {
      if (typeof window === 'undefined') throw error
      return new URL(base, window.location.origin)
    }
  }

  const url = ensureAbsolute()
  const normalizedPath = url.pathname.replace(/\/$/, '')
  url.pathname = `${normalizedPath}/speech/ws/${sessionId}`
  url.search = ''
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  return url.toString()
}

const INFO_EVENT_TYPES: readonly SpeechSocketInfoEventType[] = [
  'connected',
  'config',
  'asr',
  'user',
  'ai_delta',
  'ai',
  'tts',
  'emotion',
] as const

const isInfoEventType = (value?: string): value is SpeechSocketInfoEventType =>
  typeof value === 'string' && INFO_EVENT_TYPES.includes(value as SpeechSocketInfoEventType)

const pickSessionId = (raw: SpeechSocketRawIncomingMessage): string | undefined => {
  if (typeof raw.sessionId === 'string' && raw.sessionId) return raw.sessionId
  if (typeof raw.SessionId === 'string' && raw.SessionId) return raw.SessionId
  if (typeof raw.session_id === 'string' && raw.session_id) return raw.session_id
  return undefined
}

const toLowerCase = (value?: string): string | undefined =>
  typeof value === 'string' ? value.toLowerCase() : undefined

const normalizeSpeechSocketMessage = (
  raw: SpeechSocketRawIncomingMessage,
): SpeechSocketIncomingMessage | null => {
  if (!raw || typeof raw !== 'object') return null

  const sessionId = pickSessionId(raw)
  const normalizedType = toLowerCase(raw.type) ?? toLowerCase(raw.Type)
  if (!normalizedType) return null

  if (normalizedType === 'ping' || normalizedType === 'pong') {
    return { type: normalizedType, sessionId }
  }

  if (normalizedType === 'result' || normalizedType === 'info') {
    const rawData = (raw.data ?? raw.Data) as Record<string, unknown> | undefined
    const base = rawData && typeof rawData === 'object' ? rawData : {}
    const rawEventType = toLowerCase(
      typeof base.type === 'string'
        ? (base.type as string)
        : typeof (base as { Type?: string }).Type === 'string'
        ? ((base as { Type?: string }).Type as string)
        : undefined,
    )
    const eventType = isInfoEventType(rawEventType) ? rawEventType : 'config'
    const payload: SpeechSocketInfoPayload = {
      ...(base as SpeechSocketInfoPayload),
      type: eventType,
    }
    return {
      type: 'info',
      sessionId,
      data: payload,
    }
  }

  if (normalizedType === 'error') {
    const rawError = (raw.data ?? raw.Data) as Record<string, unknown> | undefined
    const base = rawError && typeof rawError === 'object' ? rawError : {}
    const message =
      typeof base.message === 'string' && base.message
        ? (base.message as string)
        : '语音通道出现错误'
    const code = base.code
    const errorPayload: SpeechSocketErrorPayload = {
      ...(base as SpeechSocketErrorPayload),
      message,
    }
    if (typeof code === 'string' || typeof code === 'number') {
      errorPayload.code = code
    } else {
      delete errorPayload.code
    }
    return {
      type: 'error',
      sessionId,
      data: errorPayload,
    }
  }

  return null
}

export const createSpeechSocket = (
  sessionId: string,
  handlers: SpeechSocketHandlers = {},
): { connection: Promise<SpeechSocketConnection>; socket: WebSocket } => {
  const { onOpen, onMessage, onError, onClose } = handlers
  const socket = new WebSocket(buildSpeechSocketUrl(sessionId))

  const connection = new Promise<SpeechSocketConnection>((resolve, reject) => {
    const cleanup = () => {
      socket.removeEventListener('open', handleOpen)
      socket.removeEventListener('error', handleError)
    }

    const handleOpen = () => {
      onOpen?.()
      cleanup()
      resolve({
        send: (payload) => {
          if (socket.readyState !== WebSocket.OPEN) return
          socket.send(JSON.stringify(payload))
        },
        close: () => socket.close(),
        readyState: () => socket.readyState,
      })
    }

    const handleError = (event: Event) => {
      onError?.(event)
      cleanup()
      if (socket.readyState === WebSocket.CONNECTING) {
        reject(event)
      }
    }

    socket.addEventListener('open', handleOpen)
    socket.addEventListener('error', handleError)
  })

  socket.addEventListener('message', (event: MessageEvent) => {
    try {
      if (typeof event.data !== 'string') return
      const rawPayload = JSON.parse(event.data) as SpeechSocketRawIncomingMessage
      const payload = normalizeSpeechSocketMessage(rawPayload)
      if (!payload) return
      onMessage?.(payload)
    } catch (error) {
      console.warn('[speechSocket] 无法解析语音 WebSocket 消息', error)
    }
  })

  if (onError) socket.addEventListener('error', onError)
  if (onClose) socket.addEventListener('close', onClose)

  return { connection, socket }
}
