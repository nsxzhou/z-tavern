import type { Message, Persona } from '../../api'
import { formatTime } from '../../utils/datetime'

interface ChatBubbleProps {
  message: Message & { localId?: string }
  persona: Persona
  sessionId?: string
  onPlayVoice?: () => void
  isVoiceLoading?: boolean
  isVoicePlaying?: boolean
}

export const ChatBubble: React.FC<ChatBubbleProps> = ({
  message,
  persona,
  sessionId,
  onPlayVoice,
  isVoiceLoading,
  isVoicePlaying,
}) => {
  const isUser = message.sender === 'user'
  const timestamp = message.createdAt ?? Date.now()
  const canPlayVoice = Boolean(!isUser && onPlayVoice && sessionId && message.sessionId === sessionId)

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'} group`}>
      <div
        className={`
          max-w-xl rounded-2xl px-5 py-4 text-sm leading-relaxed transition-all duration-300
          ${isUser
            ? `
              bg-gradient-to-br from-ztavern-accent-light via-ztavern-accent-dark to-ztavern-accent-light
              text-white shadow-lg shadow-ztavern-accent-light/30
              backdrop-filter backdrop-blur-md backdrop-saturate-150
              border border-white/20
              hover:scale-[1.02] hover:shadow-xl hover:shadow-ztavern-accent-light/40
            `
            : `
              glass-panel bg-ztavern-surface-light/90 text-ztavern-text-light
              dark:bg-ztavern-surface-dark/90 dark:text-ztavern-text-dark
              shadow-glass-light dark:shadow-glass-dark
              hover:transform hover:scale-[1.01]
              backdrop-blur-glass backdrop-saturate-180 backdrop-brightness-108
            `
          }
        `}
      >
        <p className="mb-0">{message.content}</p>

        <div className="mt-3 flex items-center justify-between text-[11px] uppercase tracking-widest opacity-70 transition-opacity group-hover:opacity-100">
          <span className="font-medium">
            {isUser ? '你' : persona.name}
          </span>

          <div className="flex items-center gap-2">
            {canPlayVoice && (
              <button
                type="button"
                onClick={onPlayVoice}
                disabled={isVoiceLoading}
                className={`
                  inline-flex items-center gap-1 rounded-full px-3 py-1.5 text-[10px] font-medium
                  transition-all duration-200 ripple-effect
                  ${isUser
                    ? `
                      bg-white/20 text-white border border-white/30
                      hover:bg-white/30 hover:border-white/50
                      disabled:opacity-60 disabled:cursor-not-allowed
                    `
                    : `
                      bg-ztavern-accent-light/15 text-ztavern-accent-light border border-ztavern-accent-light/30
                      dark:bg-ztavern-accent-dark/15 dark:text-ztavern-accent-dark dark:border-ztavern-accent-dark/30
                      hover:bg-ztavern-accent-light/25 hover:border-ztavern-accent-light/50
                      dark:hover:bg-ztavern-accent-dark/25 dark:hover:border-ztavern-accent-dark/50
                      disabled:opacity-60 disabled:cursor-not-allowed
                    `
                  }
                `}
              >
                {isVoiceLoading ? '合成中…' : isVoicePlaying ? '正在播放' : '🔊 播放回复'}
              </button>
            )}

            <span className="tabular-nums">
              {formatTime(timestamp)}
            </span>
          </div>
        </div>

        {message.emotion && !isUser && (
          <div className="mt-3 flex justify-start">
            <span className={`
              inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[10px] font-medium
              transition-all duration-200
              ${isUser
                ? `
                  bg-white/20 text-white border border-white/30
                `
                : `
                  bg-ztavern-accent-light/15 text-ztavern-accent-light border border-ztavern-accent-light/25
                  dark:bg-ztavern-accent-dark/15 dark:text-ztavern-accent-dark dark:border-ztavern-accent-dark/25
                  backdrop-blur-sm backdrop-saturate-150
                `
              }
            `}>
              <span className="h-1.5 w-1.5 rounded-full bg-current opacity-60"></span>
              情绪 · {message.emotion}
            </span>
          </div>
        )}
      </div>
    </div>
  )
}
