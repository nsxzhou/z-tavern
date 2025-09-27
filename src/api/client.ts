import type {
  ASRResponse,
  CreateSessionRequest,
  MessageQueuedResponse,
  Persona,
  SendMessageRequest,
  Session,
  SpeechHealthResponse,
  StreamEvent,
  TTSRequest,
  TTSResponse,
} from './types'

const baseUrl = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080/api'

export const apiBaseUrl = baseUrl

const handleResponse = async <T>(response: Response): Promise<T> => {
  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`
    try {
      const body = (await response.json()) as { error?: string }
      if (body?.error) message = body.error
    } catch {
      /* noop */
    }
    throw new Error(message)
  }
  return response.json() as Promise<T>
}

const isPersona = (input: Partial<Persona>): input is Persona =>
  Boolean(
    input &&
      typeof input.id === 'string' &&
      typeof input.name === 'string' &&
      typeof input.title === 'string' &&
      typeof input.tone === 'string' &&
      (typeof input.openingLine === 'undefined' || typeof input.openingLine === 'string'),
  )

const ensurePersonas = (payload: unknown): Persona[] => {
  if (!Array.isArray(payload)) throw new Error('后端返回的角色数据格式不正确')
  const personas = payload.filter((item): item is Persona => isPersona(item))
  if (personas.length === 0) {
    throw new Error('后端未返回任何有效角色数据')
  }
  return personas
}

export const apiClient = {
  async getPersonas(signal?: AbortSignal): Promise<Persona[]> {
    const response = await fetch(`${baseUrl}/personas`, { signal })
    const personas = await handleResponse<unknown>(response)
    return ensurePersonas(personas)
  },

  async createSession(payload: CreateSessionRequest): Promise<Session> {
    const response = await fetch(`${baseUrl}/session`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    return handleResponse<Session>(response)
  },

  async sendMessage(payload: SendMessageRequest): Promise<MessageQueuedResponse> {
    const response = await fetch(`${baseUrl}/messages`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    return handleResponse<MessageQueuedResponse>(response)
  },

  createSessionStream(sessionId: string, params?: { message?: string }): EventSource {
    const url = new URL(`${baseUrl}/stream/${sessionId}`)
    if (params?.message) url.searchParams.set('message', params.message)
    return new EventSource(url)
  },

  async transcribe(formData: FormData): Promise<ASRResponse> {
    const response = await fetch(`${baseUrl}/speech/transcribe`, {
      method: 'POST',
      body: formData,
    })
    return handleResponse<ASRResponse>(response)
  },

  async transcribeForSession(sessionId: string, formData: FormData): Promise<ASRResponse> {
    const response = await fetch(`${baseUrl}/speech/transcribe/${sessionId}`, {
      method: 'POST',
      body: formData,
    })
    return handleResponse<ASRResponse>(response)
  },

  async synthesize(payload: TTSRequest): Promise<Blob | TTSResponse> {
    const response = await fetch(`${baseUrl}/speech/synthesize`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    if (response.headers.get('content-type')?.includes('application/json')) {
      return handleResponse<TTSResponse>(response)
    }
    return response.blob()
  },

  async synthesizeForSession(sessionId: string, payload: TTSRequest): Promise<Blob | TTSResponse> {
    const response = await fetch(`${baseUrl}/speech/synthesize/${sessionId}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    if (response.headers.get('content-type')?.includes('application/json')) {
      return handleResponse<TTSResponse>(response)
    }
    return response.blob()
  },

  async speechHealth(): Promise<SpeechHealthResponse> {
    const response = await fetch(`${baseUrl}/speech/health`)
    return handleResponse<SpeechHealthResponse>(response)
  },
}

export interface StreamControllerOptions {
  message?: string
  onEvent?: (event: StreamEvent) => void
  onError?: (error: unknown) => void
}

export const createStreamController = (
  sessionId: string,
  { message, onEvent, onError }: StreamControllerOptions = {},
) => {
  const source = apiClient.createSessionStream(sessionId, message ? { message } : undefined)
  source.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data) as StreamEvent
      onEvent?.(payload)
    } catch (error) {
      onError?.(error)
    }
  }
  source.onerror = (error) => {
    onError?.(error)
    source.close()
  }
  return source
}
