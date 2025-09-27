import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  apiClient,
  createSpeechSocket,
  createStreamController,
  type Message,
  type Persona,
  type Session,
  type SpeechSocketIncomingMessage,
  type SpeechSocketOutgoingMessage,
  type SpeechSocketConnection,
  type StreamEvent,
  type TTSRequest,
} from "../api";
import { createPersonaFilterTags } from "../utils/persona";

export interface ConversationState {
  session?: Session;
  messages: ConversationMessage[];
  isStreaming: boolean;
  lastEmotion?: string;
  lastEmotionScale?: number;
  lastEmotionConfidence?: number;
}

type ConversationMap = Record<string, ConversationState>;

type ConversationMessage = Message & {
  localId: string;
  emotionScale?: number;
  emotionConfidence?: number;
};

type VoiceState =
  | "idle"
  | "connecting"
  | "recording"
  | "streaming"
  | "transcribing"
  | "error";

type SpeechSocketOutgoingPayload =
  | ({ sessionId?: string } & Omit<
      Extract<SpeechSocketOutgoingMessage, { type: "config" }>,
      "sessionId" | "timestamp"
    >)
  | ({ sessionId?: string } & Omit<
      Extract<SpeechSocketOutgoingMessage, { type: "audio" }>,
      "sessionId" | "timestamp"
    >)
  | ({ sessionId?: string } & Omit<
      Extract<SpeechSocketOutgoingMessage, { type: "text" }>,
      "sessionId" | "timestamp"
    >)
  | { type: "pong"; sessionId?: string };

type ActiveStream = {
  source: EventSource;
  sessionId: string;
  messageLocalId?: string;
};

const STREAM_FALLBACK_TIMEOUT = 8000;
const TARGET_SAMPLE_RATE = 16000;

const arrayBufferToBase64 = (buffer: ArrayBuffer): string => {
  if (typeof window === "undefined") return "";
  const bytes = new Uint8Array(buffer);
  if (bytes.length === 0) return "";
  let binary = "";
  const chunkSize = 0x8000;
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    const chunk = bytes.subarray(offset, offset + chunkSize);
    binary += String.fromCharCode(...chunk);
  }
  return window.btoa(binary);
};

const downsampleBuffer = (
  input: Float32Array,
  inputSampleRate: number,
  targetSampleRate: number
): Float32Array => {
  if (targetSampleRate >= inputSampleRate) {
    return input;
  }
  const sampleRateRatio = inputSampleRate / targetSampleRate;
  const newLength = Math.round(input.length / sampleRateRatio);
  const result = new Float32Array(newLength);
  let offsetResult = 0;
  let offsetInput = 0;

  while (offsetResult < result.length) {
    const nextOffsetInput = Math.round((offsetResult + 1) * sampleRateRatio);
    let accum = 0;
    let count = 0;
    for (let i = offsetInput; i < nextOffsetInput && i < input.length; i += 1) {
      accum += input[i];
      count += 1;
    }
    result[offsetResult] = accum / (count || 1);
    offsetResult += 1;
    offsetInput = nextOffsetInput;
  }

  return result;
};

const floatTo16BitPCM = (input: Float32Array): Int16Array => {
  const output = new Int16Array(input.length);
  for (let i = 0; i < input.length; i += 1) {
    const s = Math.max(-1, Math.min(1, input[i]));
    output[i] = s < 0 ? s * 0x8000 : s * 0x7fff;
  }
  return output;
};

const encodePCM16 = (input: Int16Array): ArrayBuffer => {
  const buffer = new ArrayBuffer(input.length * 2);
  const view = new DataView(buffer);
  for (let i = 0; i < input.length; i += 1) {
    view.setInt16(i * 2, input[i], true);
  }
  return buffer;
};

const convertToPCM16 = (
  input: Float32Array,
  inputSampleRate: number,
  targetSampleRate: number
): ArrayBuffer | null => {
  if (input.length === 0) return null;
  const normalized =
    inputSampleRate === targetSampleRate
      ? input
      : downsampleBuffer(input, inputSampleRate, targetSampleRate);
  if (!normalized || normalized.length === 0) return null;
  const pcm16 = floatTo16BitPCM(normalized);
  return encodePCM16(pcm16);
};

const wrapPCM16ToWav = (
  pcmBuffer: ArrayBuffer,
  sampleRate: number,
  channels = 1
): ArrayBuffer => {
  const pcmLength = pcmBuffer.byteLength;
  const buffer = new ArrayBuffer(44 + pcmLength);
  const view = new DataView(buffer);
  let offset = 0;

  const writeString = (value: string) => {
    for (let i = 0; i < value.length; i += 1) {
      view.setUint8(offset, value.charCodeAt(i));
      offset += 1;
    }
  };

  const bytesPerSample = 2;
  const byteRate = sampleRate * channels * bytesPerSample;
  const blockAlign = channels * bytesPerSample;

  writeString("RIFF");
  view.setUint32(offset, 36 + pcmLength, true);
  offset += 4;
  writeString("WAVE");
  writeString("fmt ");
  view.setUint32(offset, 16, true);
  offset += 4;
  view.setUint16(offset, 1, true);
  offset += 2;
  view.setUint16(offset, channels, true);
  offset += 2;
  view.setUint32(offset, sampleRate, true);
  offset += 4;
  view.setUint32(offset, byteRate, true);
  offset += 4;
  view.setUint16(offset, blockAlign, true);
  offset += 2;
  view.setUint16(offset, bytesPerSample * 8, true);
  offset += 2;
  writeString("data");
  view.setUint32(offset, pcmLength, true);
  offset += 4;

  new Uint8Array(buffer, offset).set(new Uint8Array(pcmBuffer));
  return buffer;
};

const concatenateArrayBuffers = (chunks: ArrayBuffer[]): ArrayBuffer => {
  const totalLength = chunks.reduce((acc, chunk) => acc + chunk.byteLength, 0);
  const result = new Uint8Array(totalLength);
  let offset = 0;
  chunks.forEach((chunk) => {
    result.set(new Uint8Array(chunk), offset);
    offset += chunk.byteLength;
  });
  return result.buffer;
};

const createInitialConversations = (personas: Persona[]): ConversationMap => {
  return personas.reduce<ConversationMap>((acc, persona) => {
    const openingLine =
      persona.openingLine || `${persona.name} 正在等待与你互动。`;
    const localId = `${persona.id}-intro`;
    acc[persona.id] = {
      isStreaming: false,
      lastEmotion: undefined,
      lastEmotionScale: undefined,
      lastEmotionConfidence: undefined,
      messages: [
        {
          id: localId,
          localId,
          sessionId: persona.id,
          sender: "assistant",
          content: openingLine,
          createdAt: new Date().toISOString(),
        },
      ],
    };
    return acc;
  }, {});
};

export const useChatOrchestrator = () => {
  const [personas, setPersonas] = useState<Persona[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState("");
  const [activePersonaId, setActivePersonaId] = useState<string | undefined>(
    undefined
  );
  const [composerText, setComposerText] = useState("");
  const [conversations, setConversations] = useState<ConversationMap>({});
  const conversationsRef = useRef<ConversationMap>({});
  const [voiceState, setVoiceState] = useState<VoiceState>("idle");
  const [speechError, setSpeechError] = useState<string | null>(null);
  const [ttsLoadingMessageId, setTtsLoadingMessageId] = useState<string | null>(
    null
  );
  const [ttsPlayingMessageId, setTtsPlayingMessageId] = useState<string | null>(
    null
  );
  const [speechStatusMessage, setSpeechStatusMessage] = useState("");
  const [speechStatusTone, setSpeechStatusTone] = useState<"info" | "error">(
    "info"
  );

  const abortRef = useRef<AbortController | null>(null);
  const streamsRef = useRef<Record<string, ActiveStream>>({});
  const streamFallbackTimersRef = useRef<Record<string, number>>({});
  const ttsAudioCacheRef = useRef<Map<string, string>>(new Map());
  const audioElementRef = useRef<HTMLAudioElement | null>(null);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const audioContextRef = useRef<AudioContext | null>(null);
  const audioSourceNodeRef = useRef<MediaStreamAudioSourceNode | null>(null);
  const audioProcessorNodeRef = useRef<ScriptProcessorNode | null>(null);
  const audioGainNodeRef = useRef<GainNode | null>(null);
  const pcmChunksRef = useRef<ArrayBuffer[]>([]);
  const voiceStateRef = useRef<VoiceState>("idle");
  const speechSocketRef = useRef<SpeechSocketConnection | null>(null);
  const speechSocketRawRef = useRef<WebSocket | null>(null);
  const speechModeRef = useRef<"ws" | "rest">("rest");
  const speechAvailabilityRef = useRef<"unknown" | "available" | "unavailable">(
    "unknown"
  );
  const speechChunkIndexRef = useRef(0);
  const partialTranscriptRef = useRef<string>("");
  const latestFinalTranscriptRef = useRef<string>("");
  const markFinalChunkRef = useRef(false);
  const speechAssistantMessageRef = useRef<Record<string, string | undefined>>(
    {}
  );
  const speechSessionIdRef = useRef<string | null>(null);

  const activePersona = useMemo(
    () =>
      personas.find((persona) => persona.id === activePersonaId) ?? personas[0],
    [activePersonaId, personas]
  );

  const activeConversation = activePersona
    ? conversations[activePersona.id]
    : undefined;

  const personaTags = useMemo(
    () => createPersonaFilterTags(personas),
    [personas]
  );

  useEffect(() => {
    conversationsRef.current = conversations;
  }, [conversations]);

  useEffect(() => {
    if (!speechError) return;
    if (typeof window === "undefined") return;
    const timer = window.setTimeout(() => setSpeechError(null), 4000);
    return () => window.clearTimeout(timer);
  }, [speechError]);

  useEffect(() => {
    if (voiceState !== "error") return;
    if (typeof window === "undefined") return;
    const timer = window.setTimeout(() => setVoiceState("idle"), 1800);
    return () => window.clearTimeout(timer);
  }, [voiceState]);

  useEffect(() => {
    voiceStateRef.current = voiceState;
  }, [voiceState]);

  const ensureConversation = useCallback((persona: Persona) => {
    setConversations((prev) => {
      if (prev[persona.id]) return prev;
      const next = createInitialConversations([persona]);
      return { ...prev, [persona.id]: next[persona.id] };
    });
  }, []);

  const pushMessage = useCallback(
    (personaId: string, message: ConversationMessage) => {
      setConversations((prev) => {
        const existing = prev[personaId] ?? {
          messages: [],
          isStreaming: false,
          session: undefined,
          lastEmotion: undefined,
          lastEmotionScale: undefined,
          lastEmotionConfidence: undefined,
        };
        const next: ConversationState = {
          ...existing,
          messages: [...existing.messages, message],
        };
        return { ...prev, [personaId]: next };
      });
    },
    []
  );

  const mutateMessage = useCallback(
    (
      personaId: string,
      messageLocalId: string,
      updater: (message: ConversationMessage) => ConversationMessage
    ) => {
      setConversations((prev) => {
        const existing = prev[personaId];
        if (!existing) return prev;
        const index = existing.messages.findIndex(
          (msg) => (msg.localId ?? msg.id) === messageLocalId
        );
        if (index === -1) return prev;
        const original = existing.messages[index];
        const ensured: ConversationMessage = {
          ...original,
          localId: original.localId ?? messageLocalId,
        };
        const updated = updater(ensured);
        const messages = existing.messages.slice();
        messages[index] = updated;
        return {
          ...prev,
          [personaId]: {
            ...existing,
            messages,
          },
        };
      });
    },
    []
  );

  const updateConversation = useCallback(
    (personaId: string, partial: Partial<ConversationState>) => {
      setConversations((prev) => {
        const existing = prev[personaId] ?? {
          messages: [],
          isStreaming: false,
          session: undefined,
          lastEmotion: undefined,
          lastEmotionScale: undefined,
          lastEmotionConfidence: undefined,
        };
        return { ...prev, [personaId]: { ...existing, ...partial } };
      });
    },
    []
  );

  const clearStreamFallback = useCallback((personaId: string) => {
    const timer = streamFallbackTimersRef.current[personaId];
    if (typeof timer === "number" && typeof window !== "undefined") {
      window.clearTimeout(timer);
      delete streamFallbackTimersRef.current[personaId];
    }
  }, []);

  const closeStream = useCallback(
    (personaId: string) => {
      const active = streamsRef.current[personaId];
      if (active) {
        active.source.close();
        delete streamsRef.current[personaId];
      }
      clearStreamFallback(personaId);
    },
    [clearStreamFallback]
  );

  const closeSpeechSocket = useCallback(
    (options?: { silent?: boolean; reason?: string }) => {
      if (process.env.NODE_ENV !== "production") {
        console.debug("[useChatOrchestrator] closing speech socket", options);
      }
      speechSocketRef.current?.close();
      speechSocketRef.current = null;
      const raw = speechSocketRawRef.current;
      if (raw && raw.readyState === WebSocket.OPEN) {
        raw.close();
      }
      speechSocketRawRef.current = null;
      speechModeRef.current = "rest";
      speechAssistantMessageRef.current = {};
      speechSessionIdRef.current = null;
      partialTranscriptRef.current = "";
      if (!options?.silent) {
        setSpeechStatusMessage(options?.reason ?? "语音服务未连接");
        setSpeechStatusTone(options?.reason ? "error" : "info");
      }
    },
    []
  );

  const sendSpeechMessage = useCallback(
    (payload: SpeechSocketOutgoingPayload) => {
      if (speechModeRef.current !== "ws") return;
      const connection = speechSocketRef.current;
      const readyState = connection?.readyState();
      if (!connection || readyState !== WebSocket.OPEN) {
        if (process.env.NODE_ENV !== "production") {
          console.warn("[useChatOrchestrator] skip sending WS payload", {
            readyState,
            hasConnection: Boolean(connection),
            payloadType: payload.type,
          });
        }
        return;
      }

      const sessionId = payload.sessionId ?? speechSessionIdRef.current;
      if (!sessionId) {
        if (process.env.NODE_ENV !== "production") {
          console.warn(
            "[useChatOrchestrator] missing session id for WS payload",
            {
              payloadType: payload.type,
            }
          );
        }
        return;
      }

      const timestamp = Date.now();
      let message: SpeechSocketOutgoingMessage;
      switch (payload.type) {
        case "config":
          message = {
            type: "config",
            sessionId,
            timestamp,
            data: payload.data,
          };
          break;
        case "audio":
          message = {
            type: "audio",
            sessionId,
            timestamp,
            data: payload.data,
          };
          break;
        case "text":
          message = {
            type: "text",
            sessionId,
            timestamp,
            data: payload.data,
          };
          break;
        case "pong":
        default:
          message = {
            type: "pong",
            sessionId,
            timestamp,
          };
          break;
      }

      connection.send(message);
    },
    []
  );

  const sendAudioChunk = useCallback(
    (buffer: ArrayBuffer | null, options: { isFinal?: boolean } = {}) => {
      if (speechModeRef.current !== "ws") return;
      const sessionId = speechSessionIdRef.current;
      if (!sessionId) return;
      try {
        const wavBuffer = buffer
          ? wrapPCM16ToWav(buffer, TARGET_SAMPLE_RATE)
          : null;
        const audioData = wavBuffer ? arrayBufferToBase64(wavBuffer) : "";
        const chunkIndex = speechChunkIndexRef.current++;
        const payload: SpeechSocketOutgoingPayload = {
          type: "audio",
          sessionId,
          data: {
            audioData,
            format: "wav",
            language: "zh-CN",
            isFinal: Boolean(options.isFinal),
            chunkIndex,
          },
        };
        if (process.env.NODE_ENV !== "production") {
          console.debug("[useChatOrchestrator] sending audio chunk", {
            sessionId,
            chunkIndex,
            isFinal: payload.data.isFinal,
            rawByteLength: buffer?.byteLength ?? 0,
            wavByteLength: wavBuffer?.byteLength ?? 0,
            base64Length: audioData.length,
          });
        }
        sendSpeechMessage(payload);
      } catch (error) {
        console.warn("[useChatOrchestrator] send audio chunk failed", error, {
          byteLength: buffer?.byteLength ?? 0,
          isFinal: options.isFinal,
        });
        setSpeechError("音频上传失败，语音通道已中断。");
        setVoiceState("error");
      }
    },
    [sendSpeechMessage, setSpeechError, setVoiceState]
  );

  const teardownAudioProcessing = useCallback(() => {
    if (audioProcessorNodeRef.current) {
      try {
        audioProcessorNodeRef.current.disconnect();
      } catch {
        /* noop */
      }
      audioProcessorNodeRef.current.onaudioprocess = null;
      audioProcessorNodeRef.current = null;
    }
    if (audioSourceNodeRef.current) {
      try {
        audioSourceNodeRef.current.disconnect();
      } catch {
        /* noop */
      }
      audioSourceNodeRef.current = null;
    }
    if (audioGainNodeRef.current) {
      try {
        audioGainNodeRef.current.disconnect();
      } catch {
        /* noop */
      }
      audioGainNodeRef.current = null;
    }
    if (
      audioContextRef.current &&
      audioContextRef.current.state === "running"
    ) {
      audioContextRef.current.suspend().catch(() => undefined);
    }
  }, []);

  const setupAudioProcessing = useCallback(
    async (stream: MediaStream) => {
      if (typeof window === "undefined") return;
      const contextCtor =
        window.AudioContext ??
        (
          window as typeof window & {
            webkitAudioContext?: typeof AudioContext;
          }
        ).webkitAudioContext;
      if (!contextCtor) {
        throw new Error("当前浏览器不支持 Web Audio API。");
      }

      let audioContext = audioContextRef.current;
      if (!audioContext) {
        try {
          audioContext = new contextCtor({ sampleRate: TARGET_SAMPLE_RATE });
        } catch {
          audioContext = new contextCtor();
        }
        audioContextRef.current = audioContext;
      }

      if (audioContext.state === "suspended") {
        await audioContext.resume();
      }

      const track = stream.getAudioTracks()[0];
      if (process.env.NODE_ENV !== "production") {
        console.debug("[useChatOrchestrator] audio processing setup", {
          contextSampleRate: audioContext.sampleRate,
          targetSampleRate: TARGET_SAMPLE_RATE,
          trackSettings: track?.getSettings?.(),
          mode: speechModeRef.current,
        });
      }

      const sourceNode = audioContext.createMediaStreamSource(stream);
      const processorNode = audioContext.createScriptProcessor(4096, 1, 1);
      const gainNode = audioContext.createGain();
      gainNode.gain.value = 0;

      processorNode.onaudioprocess = (event) => {
        if (
          voiceStateRef.current !== "recording" &&
          voiceStateRef.current !== "transcribing"
        ) {
          return;
        }
        const input = event.inputBuffer.getChannelData(0);
        if (input.length === 0) return;
        const pcmBuffer = convertToPCM16(
          input,
          audioContext.sampleRate,
          TARGET_SAMPLE_RATE
        );
        if (!pcmBuffer) return;
        if (
          process.env.NODE_ENV !== "production" &&
          speechModeRef.current === "ws"
        ) {
          console.debug("[useChatOrchestrator] pcm frame prepared", {
            floatLength: input.length,
            chunkIndex: speechChunkIndexRef.current,
          });
        }
        if (speechModeRef.current === "ws") {
          sendAudioChunk(pcmBuffer);
        } else {
          pcmChunksRef.current.push(pcmBuffer.slice(0));
          if (process.env.NODE_ENV !== "production") {
            console.debug("[useChatOrchestrator] buffered pcm frame for REST", {
              bufferedChunks: pcmChunksRef.current.length,
              lastChunkBytes: pcmBuffer.byteLength,
            });
          }
        }
      };

      audioSourceNodeRef.current = sourceNode;
      audioProcessorNodeRef.current = processorNode;
      audioGainNodeRef.current = gainNode;

      sourceNode.connect(processorNode);
      processorNode.connect(gainNode);
      gainNode.connect(audioContext.destination);
    },
    [sendAudioChunk]
  );

  const handleSpeechSocketMessage = useCallback(
    (persona: Persona, message: SpeechSocketIncomingMessage) => {
      if (!message) return;
      if (message.type === "error") {
        const detail =
          typeof message.data?.message === "string"
            ? message.data.message
            : "语音通道出现错误";
        console.error("[useChatOrchestrator] speech socket error", {
          detail,
          payload: message,
        });
        setSpeechError(detail);
        setSpeechStatusMessage(detail);
        setSpeechStatusTone("error");
        setVoiceState("error");
        return;
      }

      if (message.type === "pong") return;

      const activeSessionId = conversationsRef.current[persona.id]?.session?.id;
      const messageSessionId =
        message.type === "info" || message.type === "ping"
          ? message.sessionId
          : undefined;
      if (
        messageSessionId &&
        activeSessionId &&
        messageSessionId !== activeSessionId
      ) {
        if (process.env.NODE_ENV !== "production") {
          console.warn(
            "[useChatOrchestrator] ignore speech message (session mismatch)",
            {
              personaId: persona.id,
              messageSessionId,
              activeSessionId,
              messageType: message.type,
              eventType:
                message.type === "info" ? message.data?.type : undefined,
            }
          );
        }
        return;
      }

      if (message.type === "ping") {
        sendSpeechMessage({ type: "pong" });
        return;
      }
      if (message.type !== "info") return;

      const payload = message;
      const data = payload.data ?? {};

      const normalizeEmotion = (value: unknown): string | undefined =>
        typeof value === "string" && value.trim().length > 0
          ? value.trim()
          : undefined;
      const normalizeEmotionScale = (value: unknown): number | undefined =>
        typeof value === "number" && Number.isFinite(value) ? value : undefined;

      const applyEmotionUpdate = (
        emotionValue?: unknown,
        scaleValue?: unknown,
        confidenceValue?: unknown
      ) => {
        const emotion = normalizeEmotion(emotionValue);
        const scale = normalizeEmotionScale(scaleValue);
        const confidence =
          typeof confidenceValue === "number" &&
          Number.isFinite(confidenceValue)
            ? Math.max(0, Math.min(1, confidenceValue))
            : undefined;
        if (!emotion && scale === undefined && confidence === undefined) return;
        const snapshot = conversationsRef.current[persona.id];
        const nextEmotion = emotion ?? snapshot?.lastEmotion;
        const nextScale = scale ?? snapshot?.lastEmotionScale;
        const nextConfidence = confidence ?? snapshot?.lastEmotionConfidence;
        updateConversation(persona.id, {
          lastEmotion: nextEmotion,
          lastEmotionScale: nextScale,
          lastEmotionConfidence: nextConfidence,
        });
        const messageLocalId =
          speechAssistantMessageRef.current[persona.id] ??
          streamsRef.current[persona.id]?.messageLocalId;
        if (messageLocalId) {
          mutateMessage(persona.id, messageLocalId, (prev) => ({
            ...prev,
            emotion: nextEmotion ?? prev.emotion,
            emotionScale:
              nextScale !== undefined ? nextScale : prev.emotionScale,
            emotionConfidence:
              nextConfidence !== undefined
                ? nextConfidence
                : prev.emotionConfidence,
          }));
        }
      };

      if (process.env.NODE_ENV !== "production") {
        console.debug("[useChatOrchestrator] speech socket message", {
          eventType: data.type,
          sessionId: payload.sessionId,
          hasText: Boolean(data.text),
          isFinal: data.isFinal,
        });
      }

      switch (data.type) {
        case "connected": {
          setSpeechStatusMessage("语音服务已连接");
          setSpeechStatusTone("info");
          setVoiceState((prev) => (prev === "connecting" ? "idle" : prev));
          break;
        }
        case "config": {
          setSpeechStatusMessage("语音通道已准备完成");
          setSpeechStatusTone("info");
          break;
        }
        case "asr": {
          const transcript = typeof data.text === "string" ? data.text : "";
          const isFinal = Boolean(data.isFinal);
          partialTranscriptRef.current = transcript;
          if (transcript) {
            setComposerText(transcript);
            setSpeechStatusMessage(isFinal ? "语音识别完成" : "正在识别语音…");
          }
          if (isFinal) {
            latestFinalTranscriptRef.current = transcript;
            setVoiceState("streaming");
            const session = conversationsRef.current[persona.id]?.session;
            if (session) {
              sendSpeechMessage({
                type: "text",
                sessionId: session.id,
                data: {
                  text: transcript,
                  isFinal: true,
                },
              });
            }
            const timestamp = Date.now();
            const localId = `voice-user-${timestamp}`;
            pushMessage(persona.id, {
              id: localId,
              localId,
              sessionId:
                conversationsRef.current[persona.id]?.session?.id ?? persona.id,
              sender: "user",
              content: transcript,
              createdAt: new Date(timestamp).toISOString(),
            });
            speechAssistantMessageRef.current[persona.id] = undefined;
          }
          break;
        }
        case "user": {
          // acknowledge user text, no-op for now
          break;
        }
        case "ai_delta":
        case "ai": {
          const text = typeof data.text === "string" ? data.text : "";
          if (!text) break;
          const existingLocalId = speechAssistantMessageRef.current[persona.id];
          const ensureAssistantMessage = () => {
            if (existingLocalId) return existingLocalId;
            const localId = `voice-assistant-${Date.now()}`;
            speechAssistantMessageRef.current[persona.id] = localId;
            pushMessage(persona.id, {
              id: localId,
              localId,
              sessionId:
                conversationsRef.current[persona.id]?.session?.id ?? persona.id,
              sender: "assistant",
              content: "",
              createdAt: new Date().toISOString(),
            });
            return localId;
          };
          const messageId = ensureAssistantMessage();
          mutateMessage(persona.id, messageId, (prev) => ({
            ...prev,
            content: data.type === "ai" ? text : `${prev.content ?? ""}${text}`,
          }));
          if (data.type === "ai") {
            setVoiceState("idle");
            setSpeechStatusMessage("语音对话完成");
            setSpeechStatusTone("info");
            speechAssistantMessageRef.current[persona.id] = undefined;
          }
          break;
        }
        case "emotion": {
          applyEmotionUpdate(
            data.emotion,
            data.emotionScale ?? data.scale,
            data.emotionConfidence ?? data.confidence
          );
          break;
        }
        case "tts": {
          const base64Audio =
            typeof data.audioData === "string" && data.audioData.length > 0
              ? data.audioData
              : typeof data.audio === "string"
              ? data.audio
              : "";
          if (process.env.NODE_ENV !== "production") {
            console.debug("[useChatOrchestrator] received TTS payload", {
              hasAudio: base64Audio.length > 0,
              format: data.format,
            });
          }
          if (base64Audio) {
            try {
              const binary = window.atob(base64Audio);
              const len = binary.length;
              const bytes = new Uint8Array(len);
              for (let i = 0; i < len; i += 1) {
                bytes[i] = binary.charCodeAt(i);
              }
              const blob = new Blob([bytes.buffer], {
                type: data.format ? `audio/${data.format}` : "audio/mpeg",
              });
              const url = URL.createObjectURL(blob);
              if (audioElementRef.current) {
                audioElementRef.current.pause();
              }
              const audio = new Audio(url);
              audioElementRef.current = audio;
              audio.play().catch((err) => {
                console.warn("[useChatOrchestrator] TTS playback failed", err);
              });
              audio.addEventListener("ended", () => URL.revokeObjectURL(url), {
                once: true,
              });
            } catch (err) {
              console.warn(
                "[useChatOrchestrator] decode TTS payload failed",
                err
              );
            }
          }
          applyEmotionUpdate(
            data.emotion,
            data.emotionScale ?? data.scale,
            data.emotionConfidence ?? data.confidence
          );
          break;
        }
        default:
          break;
      }
    },
    [
      mutateMessage,
      pushMessage,
      sendSpeechMessage,
      setComposerText,
      setSpeechError,
      setSpeechStatusMessage,
      setSpeechStatusTone,
      setVoiceState,
      updateConversation,
    ]
  );

  const connectSpeechSocket = useCallback(
    async (persona: Persona, session: Session) => {
      const fallbackToRest = () => {
        setSpeechStatusMessage("语音服务未启用，已回退至 REST 模式");
        setSpeechStatusTone("error");
        speechModeRef.current = "rest";
      };

      if (speechModeRef.current === "ws") {
        const connection = speechSocketRef.current;
        if (
          connection &&
          connection.readyState() === WebSocket.OPEN &&
          speechSessionIdRef.current === session.id
        ) {
          return;
        }
        closeSpeechSocket({ silent: true });
      }

      if (speechAvailabilityRef.current === "unavailable") {
        fallbackToRest();
        return;
      }

      if (speechAvailabilityRef.current === "unknown") {
        try {
          await apiClient.speechHealth();
          speechAvailabilityRef.current = "available";
        } catch (err) {
          speechAvailabilityRef.current = "unavailable";
          if (process.env.NODE_ENV !== "production") {
            console.warn(
              "[useChatOrchestrator] speech health check failed",
              err,
              {
                personaId: persona.id,
                sessionId: session.id,
              }
            );
          }
          fallbackToRest();
          return;
        }
      }

      if (speechAvailabilityRef.current !== "available") {
        fallbackToRest();
        return;
      }

      setVoiceState("connecting");
      setSpeechStatusMessage("正在连接语音服务…");
      setSpeechStatusTone("info");
      if (process.env.NODE_ENV !== "production") {
        console.debug("[useChatOrchestrator] connecting speech socket", {
          personaId: persona.id,
          sessionId: session.id,
        });
      }

      const { connection, socket } = createSpeechSocket(session.id, {
        onMessage: (message) => handleSpeechSocketMessage(persona, message),
        onError: (error) => {
          console.error(
            "[useChatOrchestrator] speech socket error event",
            error
          );
          setSpeechStatusMessage("语音服务连接失败，已回退至 REST 模式");
          setSpeechStatusTone("error");
          speechModeRef.current = "rest";
          speechAvailabilityRef.current = "unknown";
        },
        onClose: (event) => {
          console.warn("[useChatOrchestrator] speech socket closed", event);
          closeSpeechSocket({
            silent: true,
            reason: "语音服务连接中断，若需继续语音对话请重新开启录音。",
          });
        },
      });

      speechSocketRawRef.current = socket;
      try {
        const readyConnection = await connection;
        if (process.env.NODE_ENV !== "production") {
          console.debug("[useChatOrchestrator] speech socket connected", {
            personaId: persona.id,
            sessionId: session.id,
          });
        }
        speechSocketRef.current = readyConnection;
        speechSessionIdRef.current = session.id;
        speechModeRef.current = "ws";
        setSpeechStatusMessage("语音服务已连接");
        setSpeechStatusTone("info");
        sendSpeechMessage({
          type: "config",
          sessionId: session.id,
          data: {
            personaId: persona.id,
            language: "zh-CN",
            asrEnabled: true,
            ttsEnabled: true,
            streamMode: true,
          },
        });
      } catch (err) {
        console.warn(
          "[useChatOrchestrator] speech socket connect failed",
          err,
          {
            personaId: persona.id,
            sessionId: session.id,
          }
        );
        setSpeechStatusMessage("语音服务暂不可用，已自动降级");
        setSpeechStatusTone("error");
        speechModeRef.current = "rest";
      }
    },
    [
      closeSpeechSocket,
      handleSpeechSocketMessage,
      sendSpeechMessage,
      setSpeechStatusMessage,
      setSpeechStatusTone,
      setVoiceState,
    ]
  );

  const isAbortError = (error: unknown): boolean => {
    if (error instanceof DOMException) {
      return error.name === "AbortError";
    }
    if (typeof error === "object" && error !== null && "name" in error) {
      return (error as { name?: string }).name === "AbortError";
    }
    return false;
  };

  const fetchPersonas = useCallback(async () => {
    setLoading(true);
    setError(null);
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;
    try {
      const data = await apiClient.getPersonas(controller.signal);
      setPersonas(data);
      setConversations((prev) => ({
        ...createInitialConversations(data),
        ...prev,
      }));
      setActivePersonaId((prev) => {
        if (prev && data.some((persona) => persona.id === prev)) return prev;
        return data[0]?.id;
      });
    } catch (err) {
      if (isAbortError(err)) {
        return;
      }
      console.error("[useChatOrchestrator] fetch personas failed", err);
      setError(
        "无法从后端加载角色，请确认服务已在 http://localhost:8080/api 运行。"
      );
      setPersonas([]);
      setConversations({});
      setActivePersonaId(undefined);
    } finally {
      if (abortRef.current === controller) {
        abortRef.current = null;
      }
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchPersonas().catch((err) => console.error(err));
    return () => abortRef.current?.abort();
  }, [fetchPersonas]);

  const deliverFallbackResponse = useCallback(
    (persona: Persona) => {
      const fallbackText = `${persona.name} 正在思考你的问题……`;
      const timestamp = Date.now();
      const localId = `fallback-${timestamp}`;
      pushMessage(persona.id, {
        id: localId,
        localId,
        sessionId: persona.id,
        sender: "assistant",
        content: fallbackText,
        createdAt: new Date().toISOString(),
        emotion: "Thinking",
      });
      updateConversation(persona.id, { isStreaming: false });
    },
    [pushMessage, updateConversation]
  );

  const handleStreamEvent = useCallback(
    (persona: Persona, event: StreamEvent) => {
      const active = streamsRef.current[persona.id];
      if (!active) return;

      const ensureAssistantMessage = (initialContent = "") => {
        if (!active.messageLocalId) {
          const localId = `assistant-${Date.now()}`;
          active.messageLocalId = localId;
          pushMessage(persona.id, {
            id: localId,
            localId,
            sessionId: active.sessionId,
            sender: "assistant",
            content: initialContent,
            createdAt: new Date().toISOString(),
          });
        } else if (initialContent) {
          mutateMessage(persona.id, active.messageLocalId, (prev) => ({
            ...prev,
            content: initialContent,
          }));
        }
        clearStreamFallback(persona.id);
        return active.messageLocalId;
      };

      switch (event.event) {
        case "start": {
          ensureAssistantMessage(event.content ?? event.message ?? "");
          break;
        }
        case "delta": {
          const messageId = ensureAssistantMessage();
          if (messageId && event.content) {
            mutateMessage(persona.id, messageId, (prev) => ({
              ...prev,
              content: `${prev.content ?? ""}${event.content}`,
            }));
          }
          break;
        }
        case "message": {
          const messageId = ensureAssistantMessage(
            event.content ?? event.message ?? ""
          );
          if (messageId) {
            updateConversation(persona.id, { isStreaming: false });
            closeStream(persona.id);
          }
          break;
        }
        case "emotion": {
          const parsePayload = (value?: string) => {
            if (!value) return null;
            try {
              const parsed = JSON.parse(value) as {
                emotion?: unknown;
                scale?: unknown;
                emotionScale?: unknown;
                confidence?: unknown;
                emotionConfidence?: unknown;
              } | null;
              return parsed && typeof parsed === "object" ? parsed : null;
            } catch (error) {
              if (process.env.NODE_ENV !== "production") {
                console.warn(
                  "[useChatOrchestrator] failed to parse emotion payload",
                  error,
                  value
                );
              }
              return null;
            }
          };

          const payload =
            parsePayload(event.content) ?? parsePayload(event.message) ?? null;

          const rawEmotion = (payload?.emotion ?? event.emotion) as
            | unknown
            | undefined;
          const rawScale = (payload?.emotionScale ??
            payload?.scale ??
            event.emotionScale) as unknown | undefined;
          const rawConfidence = (payload?.confidence ??
            payload?.emotionConfidence ??
            event.emotionConfidence) as unknown | undefined;

          const emotion =
            typeof rawEmotion === "string" && rawEmotion.trim().length > 0
              ? rawEmotion.trim()
              : undefined;
          const scale =
            typeof rawScale === "number" && Number.isFinite(rawScale)
              ? rawScale
              : undefined;
          const confidence =
            typeof rawConfidence === "number" && Number.isFinite(rawConfidence)
              ? Math.max(0, Math.min(1, rawConfidence))
              : undefined;

          if (emotion || scale !== undefined || confidence !== undefined) {
            const conversationSnapshot = conversationsRef.current[persona.id];
            const nextEmotion = emotion ?? conversationSnapshot?.lastEmotion;
            const nextScale = scale ?? conversationSnapshot?.lastEmotionScale;
            const nextConfidence =
              confidence ?? conversationSnapshot?.lastEmotionConfidence;
            updateConversation(persona.id, {
              lastEmotion: nextEmotion,
              lastEmotionScale: nextScale,
              lastEmotionConfidence: nextConfidence,
            });
            const messageId = active.messageLocalId;
            if (messageId) {
              mutateMessage(persona.id, messageId, (prev) => ({
                ...prev,
                emotion: nextEmotion ?? prev.emotion,
                emotionScale:
                  nextScale !== undefined ? nextScale : prev.emotionScale,
                emotionConfidence:
                  nextConfidence !== undefined
                    ? nextConfidence
                    : prev.emotionConfidence,
              }));
            }
          }
          break;
        }
        case "end": {
          updateConversation(persona.id, { isStreaming: false });
          closeStream(persona.id);
          break;
        }
        case "error": {
          console.warn("[useChatOrchestrator] stream error event", event.error);
          updateConversation(persona.id, { isStreaming: false });
          closeStream(persona.id);
          setError("生成回复失败，已退回示例回答。");
          deliverFallbackResponse(persona);
          break;
        }
        case "heartbeat":
        case "status":
        default:
          break;
      }
    },
    [
      clearStreamFallback,
      closeStream,
      deliverFallbackResponse,
      mutateMessage,
      pushMessage,
      setError,
      updateConversation,
    ]
  );

  const startStream = useCallback(
    (persona: Persona, session: Session, message: string) => {
      closeStream(persona.id);
      const source = createStreamController(session.id, {
        message,
        onEvent: (event) => handleStreamEvent(persona, event),
        onError: (err) => {
          console.warn("[useChatOrchestrator] stream controller error", err);
          updateConversation(persona.id, { isStreaming: false });
          closeStream(persona.id);
          setError("实时流断开，已提供示例回答。");
          deliverFallbackResponse(persona);
        },
      });
      streamsRef.current[persona.id] = { source, sessionId: session.id };
      if (typeof window !== "undefined") {
        clearStreamFallback(persona.id);
        streamFallbackTimersRef.current[persona.id] = window.setTimeout(() => {
          console.warn(
            "[useChatOrchestrator] stream timeout fallback",
            persona.id
          );
          updateConversation(persona.id, { isStreaming: false });
          closeStream(persona.id);
          setError("AI 服务响应超时，已展示示例回答。");
          deliverFallbackResponse(persona);
        }, STREAM_FALLBACK_TIMEOUT);
      }
    },
    [
      clearStreamFallback,
      closeStream,
      deliverFallbackResponse,
      handleStreamEvent,
      setError,
      updateConversation,
    ]
  );

  const ensureSession = useCallback(
    async (persona: Persona) => {
      const current = conversations[persona.id];
      if (current?.session) return current.session;
      try {
        const session = await apiClient.createSession({
          personaId: persona.id,
        });
        updateConversation(persona.id, { session });
        return session;
      } catch (err) {
        console.warn("[useChatOrchestrator] create session failed", err);
        return undefined;
      }
    },
    [conversations, updateConversation]
  );

  const sendMessage = useCallback(
    async (content: string) => {
      const persona = activePersona;
      if (!persona) return;
      const trimmed = content.trim();
      if (!trimmed) return;

      const timestamp = Date.now();
      const localId = `user-${timestamp}`;
      const existingSessionId = activeConversation?.session?.id ?? persona.id;

      const userMessage: ConversationMessage = {
        id: localId,
        localId,
        sessionId: existingSessionId,
        sender: "user",
        content: trimmed,
        createdAt: new Date(timestamp).toISOString(),
      };

      pushMessage(persona.id, userMessage);
      setComposerText("");
      updateConversation(persona.id, { isStreaming: true });

      const session = await ensureSession(persona);
      if (!session) {
        setError("无法创建会话，请确认后端服务可用。");
        deliverFallbackResponse(persona);
        return;
      }

      if (session.id !== existingSessionId) {
        mutateMessage(persona.id, localId, (prev) => ({
          ...prev,
          sessionId: session.id,
        }));
      }

      try {
        await apiClient.sendMessage({
          sessionId: session.id,
          sender: "user",
          content: trimmed,
        });
      } catch (err) {
        console.warn("[useChatOrchestrator] send message failed", err);
        setError("发送消息失败，请确认后端服务可用。");
        deliverFallbackResponse(persona);
        return;
      }

      startStream(persona, session, trimmed);
    },
    [
      activeConversation?.session?.id,
      activePersona,
      deliverFallbackResponse,
      ensureSession,
      mutateMessage,
      pushMessage,
      setComposerText,
      setError,
      startStream,
      updateConversation,
    ]
  );

  const appendPrompt = useCallback((prompt: string) => {
    setComposerText((prev) => (prev ? `${prev}\n${prompt}` : prompt));
  }, []);

  const toggleVoiceInput = useCallback(async () => {
    if (!activePersona) {
      setSpeechError("请先选择一个角色。");
      return;
    }

    if (process.env.NODE_ENV !== "production") {
      console.debug("[useChatOrchestrator] toggle voice input", {
        personaId: activePersona.id,
        voiceState,
        speechMode: speechModeRef.current,
      });
    }

    if (voiceState === "recording" || voiceState === "streaming") {
      setSpeechStatusMessage("停止录音，正在识别…");
      markFinalChunkRef.current = true;
      mediaRecorderRef.current?.stop();
      setVoiceState("transcribing");
      return;
    }

    if (voiceState === "connecting" || voiceState === "transcribing") return;

    if (
      typeof navigator === "undefined" ||
      !navigator.mediaDevices?.getUserMedia
    ) {
      setSpeechError("当前浏览器不支持语音输入。");
      setVoiceState("error");
      return;
    }

    if (
      typeof window !== "undefined" &&
      typeof window.MediaRecorder === "undefined"
    ) {
      setSpeechError("浏览器未实现 MediaRecorder，无法录音。");
      setVoiceState("error");
      return;
    }

    let stream: MediaStream | null = null;
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    } catch (err) {
      console.warn("[useChatOrchestrator] getUserMedia failed", err);
      setSpeechError("无法访问麦克风，请检查浏览器权限设置。");
      setVoiceState("error");
      return;
    }

    try {
      const session = await ensureSession(activePersona);
      if (!session) {
        stream.getTracks().forEach((track) => track.stop());
        setSpeechError("无法创建会话，请确认后端服务可用。");
        setVoiceState("error");
        return;
      }

      await connectSpeechSocket(activePersona, session);

      pcmChunksRef.current = [];
      speechChunkIndexRef.current = 0;
      partialTranscriptRef.current = "";
      latestFinalTranscriptRef.current = "";
      markFinalChunkRef.current = false;

      try {
        await setupAudioProcessing(stream);
      } catch (err) {
        console.warn(
          "[useChatOrchestrator] setup audio processing failed",
          err
        );
        stream.getTracks().forEach((track) => track.stop());
        teardownAudioProcessing();
        setSpeechError("无法初始化音频处理，请检查浏览器兼容性。");
        setVoiceState("error");
        return;
      }

      const mimeType =
        typeof MediaRecorder !== "undefined" &&
        MediaRecorder.isTypeSupported("audio/webm;codecs=opus")
          ? "audio/webm;codecs=opus"
          : undefined;
      const recorder = new MediaRecorder(
        stream,
        mimeType ? { mimeType } : undefined
      );
      mediaRecorderRef.current = recorder;

      recorder.ondataavailable = () => {
        // Web Audio API 负责采样并转换为 PCM16，此处无需处理数据块
      };

      recorder.onerror = (event) => {
        console.warn("[useChatOrchestrator] recorder error", event);
        setSpeechError("录音失败，请重试。");
        setVoiceState("error");
        pcmChunksRef.current = [];
        stream?.getTracks().forEach((track) => track.stop());
        teardownAudioProcessing();
      };

      recorder.onstop = async () => {
        if (process.env.NODE_ENV !== "production") {
          console.debug("[useChatOrchestrator] recorder stopped", {
            speechMode: speechModeRef.current,
            bufferedChunks: pcmChunksRef.current.length,
            sentChunks: speechChunkIndexRef.current,
            markFinalChunk: markFinalChunkRef.current,
          });
        }
        stream?.getTracks().forEach((track) => track.stop());
        teardownAudioProcessing();

        if (speechModeRef.current === "ws") {
          if (markFinalChunkRef.current || speechChunkIndexRef.current > 0) {
            sendAudioChunk(null, { isFinal: true });
          }
          markFinalChunkRef.current = false;
          setSpeechStatusMessage("语音数据已发送，等待回复…");
          return;
        }

        const combinedBuffer =
          pcmChunksRef.current.length > 0
            ? concatenateArrayBuffers(pcmChunksRef.current)
            : null;
        pcmChunksRef.current = [];
        if (!combinedBuffer || combinedBuffer.byteLength === 0) {
          setVoiceState("idle");
          return;
        }
        setVoiceState("transcribing");
        try {
          const wavBuffer = wrapPCM16ToWav(combinedBuffer, TARGET_SAMPLE_RATE);
          const blob = new Blob([wavBuffer], {
            type: "audio/wav",
          });
          const formData = new FormData();
          formData.append("audio", blob, `voice-${Date.now()}.wav`);
          formData.append("language", "zh-CN");
          const result = await apiClient.transcribeForSession(
            session.id,
            formData
          );
          if (process.env.NODE_ENV !== "production") {
            console.debug("[useChatOrchestrator] REST transcription result", {
              sessionId: session.id,
              textLength: result.text?.length ?? 0,
              confidence: result.confidence,
            });
          }
          if (result.text) {
            setComposerText((prev) =>
              prev ? `${prev}\n${result.text}` : result.text
            );
          }
          setVoiceState("idle");
          setSpeechStatusMessage("语音识别完成");
        } catch (err) {
          console.warn(
            "[useChatOrchestrator] speech transcription failed",
            err
          );
          setSpeechError("语音转文本失败，请稍后重试。");
          setVoiceState("error");
        }
      };

      const useSocket = speechModeRef.current === "ws";
      setSpeechError(null);
      setSpeechStatusMessage("正在录音…再次点击结束");
      setSpeechStatusTone("info");
      setVoiceState("recording");
      if (useSocket) {
        recorder.start(400);
      } else {
        recorder.start();
      }
    } catch (err) {
      console.warn("[useChatOrchestrator] start voice input failed", err);
      stream?.getTracks().forEach((track) => track.stop());
      teardownAudioProcessing();
      setSpeechError("语音通道初始化失败，请稍后重试。");
      setVoiceState("error");
    }
  }, [
    activePersona,
    connectSpeechSocket,
    ensureSession,
    sendAudioChunk,
    setComposerText,
    setupAudioProcessing,
    teardownAudioProcessing,
    voiceState,
  ]);

  const playAssistantAudio = useCallback(
    async (personaId: string, messageLocalId: string) => {
      const conversation = conversations[personaId];
      if (!conversation?.session) {
        setSpeechError("当前会话尚未建立，无法播放语音。");
        setVoiceState("error");
        return;
      }

      const message = conversation.messages.find(
        (item) => (item.localId ?? item.id) === messageLocalId
      );
      if (!message || message.sender !== "assistant") return;

      const cacheKey = `${conversation.session.id}:${messageLocalId}`;
      let audioUrl = ttsAudioCacheRef.current.get(cacheKey);

      if (!audioUrl) {
        setTtsLoadingMessageId(messageLocalId);
        try {
          const emotion = message.emotion ?? conversation.lastEmotion;
          const rawEmotionScale =
            message.emotionScale ?? conversation.lastEmotionScale;
          const normalizedEmotion =
            typeof emotion === "string" && emotion.trim().length > 0
              ? emotion.trim()
              : undefined;
          const normalizedEmotionScale =
            typeof rawEmotionScale === "number" &&
            Number.isFinite(rawEmotionScale)
              ? Math.max(1, Math.min(5, rawEmotionScale))
              : undefined;

          const payload: TTSRequest = {
            text: message.content,
          };
          if (normalizedEmotion) {
            payload.emotion = normalizedEmotion;
            if (normalizedEmotionScale !== undefined) {
              payload.emotionScale = normalizedEmotionScale;
            }
            payload.enableEmotion = true;
          }

          const response = await apiClient.synthesizeForSession(
            conversation.session.id,
            payload
          );
          if (response instanceof Blob) {
            audioUrl = URL.createObjectURL(response);
          } else if (response.audioUrl) {
            audioUrl = response.audioUrl;
          } else {
            throw new Error("No audio data in response");
          }
          ttsAudioCacheRef.current.set(cacheKey, audioUrl);
        } catch (err) {
          console.warn("[useChatOrchestrator] synthesize failed", err, {
            sessionId: conversation.session.id,
            messageLocalId,
          });
          setSpeechError(
            `语音播报失败：${err instanceof Error ? err.message : "未知错误"}`
          );
          setVoiceState("error");
          setTtsLoadingMessageId(null);
          return;
        }
        setTtsLoadingMessageId(null);
      }

      if (!audioUrl) return;

      if (audioElementRef.current) {
        audioElementRef.current.pause();
        audioElementRef.current = null;
      }

      const audio = new Audio(audioUrl);
      audioElementRef.current = audio;
      setTtsPlayingMessageId(messageLocalId);
      audio.play().catch((err) => {
        console.warn("[useChatOrchestrator] audio playback failed", err);
        setSpeechError("浏览器无法播放合成语音。");
        setVoiceState("error");
        setTtsPlayingMessageId(null);
      });

      const cleanup = () => {
        setTtsPlayingMessageId((current) =>
          current === messageLocalId ? null : current
        );
        audio.removeEventListener("ended", cleanup);
        audio.removeEventListener("error", onError);
      };
      const onError = () => {
        setSpeechError("合成语音播放中断。");
        setVoiceState("error");
        setTtsPlayingMessageId((current) =>
          current === messageLocalId ? null : current
        );
      };

      audio.addEventListener("ended", cleanup);
      audio.addEventListener("error", onError);
    },
    [conversations]
  );

  const shufflePersona = useCallback(() => {
    if (personas.length === 0) return;
    const randomPersona = personas[Math.floor(Math.random() * personas.length)];
    setActivePersonaId(randomPersona.id);
    ensureConversation(randomPersona);
  }, [ensureConversation, personas]);

  const selectPersona = useCallback(
    (persona: Persona) => {
      setActivePersonaId(persona.id);
      ensureConversation(persona);
    },
    [ensureConversation]
  );

  useEffect(() => {
    const cache = ttsAudioCacheRef.current;
    return () => {
      abortRef.current?.abort();
      Object.values(streamsRef.current).forEach(({ source }) => source.close());
      if (typeof window !== "undefined") {
        Object.values(streamFallbackTimersRef.current).forEach((timer) =>
          window.clearTimeout(timer)
        );
      }
      streamsRef.current = {};
      streamFallbackTimersRef.current = {};
      if (audioElementRef.current) {
        audioElementRef.current.pause();
        audioElementRef.current = null;
      }
      cache.forEach((url) => {
        if (url.startsWith("blob:")) URL.revokeObjectURL(url);
      });
      cache.clear();
      mediaRecorderRef.current?.stop();
      teardownAudioProcessing();
      const context = audioContextRef.current;
      if (context) {
        context.close().catch(() => undefined);
        audioContextRef.current = null;
      }
      pcmChunksRef.current = [];
      closeSpeechSocket({ silent: true });
    };
  }, [closeSpeechSocket, teardownAudioProcessing]);

  return {
    personas,
    loading,
    error,
    searchTerm,
    setSearchTerm,
    personaTags,
    activePersona,
    activeConversation,
    composerText,
    setComposerText,
    appendPrompt,
    sendMessage,
    selectPersona,
    shufflePersona,
    refresh: fetchPersonas,
    voiceState,
    voiceError: speechError,
    speechStatusMessage,
    speechStatusTone,
    toggleVoiceInput,
    playAssistantAudio,
    ttsLoadingMessageId,
    ttsPlayingMessageId,
  };
};
