export interface Persona {
  id: string
  name: string
  title: string
  tone: string
  openingLine?: string
  voiceId?: string
  traits?: string[]
  expertise?: string[]
}

export interface Session {
  id: string
  personaId: string
  createdAt: string
}

export type MessageSender = 'user' | 'assistant' | 'system' | 'tool'

export interface Message {
  id: string
  sessionId: string
  sender: MessageSender
  content: string
  emotion?: string
  createdAt: string
}

export interface CreateSessionRequest {
  personaId: string
}

export interface SendMessageRequest {
  sessionId: string
  sender: MessageSender
  content: string
}

export interface MessageQueuedResponse {
  status: string
}

export interface ErrorResponse {
  error: string
}

export interface ASRResponse {
  sessionId: string
  text: string
  confidence: number
  duration: number
  createdAt: string
  requestId?: string
}

export interface TTSRequest {
  sessionId?: string
  text: string
  voice?: string
  speed?: number
  volume?: number
  format?: string
  language?: string
  emotion?: string
  emotionScale?: number
  enableEmotion?: boolean
}

export interface TTSResponse {
  sessionId: string
  audioUrl?: string
  duration: number
  format: string
  requestId?: string
  createdAt: string
}

export interface StreamEvent {
  event:
    | 'start'
    | 'delta'
    | 'message'
    | 'end'
    | 'heartbeat'
    | 'error'
    | 'status'
    | 'emotion'
  content?: string
  message?: string
  sessionId?: string
  finished?: boolean
  error?: string
  emotion?: string
  emotionScale?: number
  emotionConfidence?: number
}

export interface SpeechHealthResponse {
  status: string
  service: string
}

export type SpeechSocketOutgoingMessage =
  | {
      type: 'config'
      sessionId: string
      timestamp: number
      data: {
        personaId?: string
        language?: string
        voice?: string
        asrEnabled?: boolean
        ttsEnabled?: boolean
        streamMode?: boolean
      }
    }
  | {
      type: 'audio'
      sessionId: string
      timestamp: number
      data: {
        audioData: string
        format?: string
        language?: string
        isFinal?: boolean
        chunkIndex?: number
      }
    }
  | {
      type: 'text'
      sessionId: string
      timestamp: number
      data: {
        text: string
        isFinal?: boolean
      }
    }
  | {
      type: 'pong'
      sessionId: string
      timestamp: number
    }

export type SpeechSocketInfoEventType =
  | 'connected'
  | 'config'
  | 'asr'
  | 'user'
  | 'ai_delta'
  | 'ai'
  | 'tts'
  | 'emotion'

export interface SpeechSocketInfoPayload {
  type: SpeechSocketInfoEventType
  text?: string
  confidence?: number
  isFinal?: boolean
  audioData?: string
  audio?: string
  format?: string
  emotion?: string
  emotionScale?: number
  scale?: number
  emotionConfidence?: number
  [key: string]: unknown
}

export interface SpeechSocketErrorPayload {
  message: string
  code?: string | number
  [key: string]: unknown
}

export type SpeechSocketIncomingMessage =
  | {
      type: 'info'
      sessionId?: string
      data: SpeechSocketInfoPayload
    }
  | {
      type: 'error'
      sessionId?: string
      data: SpeechSocketErrorPayload
    }
  | {
      type: 'ping'
      sessionId?: string
    }
  | {
      type: 'pong'
      sessionId?: string
    }

export interface SpeechSocketRawIncomingMessage {
  Type?: string
  type?: string
  SessionId?: string
  sessionId?: string
  Data?: unknown
  data?: unknown
  [key: string]: unknown
}

export interface ApiConfig {
  baseUrl: string
}
