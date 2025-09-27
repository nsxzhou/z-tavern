import type { Message, Persona } from '../../api'
import { ChatBubble } from './ChatBubble'

interface ChatTimelineProps {
  persona: Persona
  messages: Array<Message & { localId?: string }>
  isStreaming?: boolean
  sessionId?: string
  onPlayVoice?: (messageLocalId: string) => void
  ttsLoadingMessageId?: string | null
  ttsPlayingMessageId?: string | null
}

export const ChatTimeline: React.FC<ChatTimelineProps> = ({
  persona,
  messages,
  isStreaming,
  sessionId,
  onPlayVoice,
  ttsLoadingMessageId,
  ttsPlayingMessageId,
}) => {
  return (
    <div className="glass-panel flex-1 h-full min-h-0 space-y-4 overflow-y-auto p-5">
      {messages.length === 0 && !isStreaming && (
        <div className="flex h-full min-h-[320px] flex-col items-center justify-center gap-2 text-sm text-slate-400 dark:text-slate-500">
          <span role="img" aria-hidden>
            ✨
          </span>
          <p>还没有消息，向角色打个招呼吧。</p>
        </div>
      )}
      {messages.map((message) => {
        const key = message.localId ?? message.id
        return (
          <ChatBubble
            key={key}
            message={message}
            persona={persona}
            sessionId={sessionId}
            onPlayVoice={onPlayVoice ? () => onPlayVoice(key) : undefined}
            isVoiceLoading={key === ttsLoadingMessageId}
            isVoicePlaying={key === ttsPlayingMessageId}
          />
        )
      })}
      {isStreaming && (
        <div className="flex justify-start">
          <div className="flex items-center gap-2 rounded-full border border-slate-300/40 bg-white/60 px-3 py-1.5 text-xs text-slate-500 shadow-inner dark:border-slate-600/40 dark:bg-slate-900/60 dark:text-slate-300">
            <span className="h-2 w-2 animate-pulse rounded-full bg-sky-400" />
            正在思考回应…
          </div>
        </div>
      )}
    </div>
  )
}
