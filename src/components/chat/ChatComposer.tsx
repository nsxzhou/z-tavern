import { useCallback, useEffect, useRef } from 'react'

type ComposerVoiceState =
  | 'idle'
  | 'connecting'
  | 'recording'
  | 'streaming'
  | 'transcribing'
  | 'error'

interface ChatComposerProps {
  value: string
  onChange: (value: string) => void
  onSend: () => void
  disabled?: boolean
  placeholder?: string
  onPromptSelect?: (prompt: string) => void
  onVoiceToggle?: () => void
  voiceState?: ComposerVoiceState
  voiceError?: string | null
  voiceStatusMessage?: string
  voiceStatusTone?: 'info' | 'error'
}

export const ChatComposer: React.FC<ChatComposerProps> = ({
  value,
  onChange,
  onSend,
  disabled = false,
  placeholder = '输入你的想法或问题，按 Enter 发送',
  onVoiceToggle,
  voiceState = 'idle',
  voiceError,
  voiceStatusMessage,
  voiceStatusTone = 'info',
}) => {
  const textareaRef = useRef<HTMLTextAreaElement | null>(null)

  const handleKeyDown = useCallback(
    (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (event.key === 'Enter' && !event.shiftKey) {
        event.preventDefault()
        onSend()
      }
    },
    [onSend],
  )

  useEffect(() => {
    const textarea = textareaRef.current
    if (!textarea) return
    textarea.style.height = 'auto'
    textarea.style.height = `${textarea.scrollHeight}px`
  }, [value])

  const voiceStatusClass =
    voiceStatusTone === 'error'
      ? 'text-red-500 dark:text-red-400'
      : 'text-ztavern-muted-light dark:text-ztavern-muted-dark'

  return (
    <div className="glass-panel rounded-2xl p-5 space-y-4 backdrop-blur-glass backdrop-saturate-180 backdrop-brightness-108 animate-glass-morph">

      <div className="relative group">
        <textarea
          ref={textareaRef}
          rows={3}
          value={value}
          onKeyDown={handleKeyDown}
          onChange={(event) => onChange(event.target.value)}
          placeholder={placeholder}
          className={`
            input-glass w-full min-h-[80px] max-h-[200px] resize-none rounded-2xl px-4 py-3 text-sm
            text-ztavern-text-light dark:text-ztavern-text-dark
            placeholder-ztavern-muted-light dark:placeholder-ztavern-muted-dark
            transition-all duration-300
            ${disabled ? 'opacity-60 cursor-not-allowed' : 'hover:scale-[1.01]'}
            focus:scale-[1.01] focus:shadow-glass-glow-light dark:focus:shadow-glass-glow-dark
          `}
          disabled={disabled}
        />

        {/* 输入框边缘光效 */}
        <div className="absolute inset-0 rounded-2xl opacity-0 group-focus-within:opacity-100 pointer-events-none transition-opacity duration-300">
          <div className="absolute inset-0 rounded-2xl bg-gradient-to-r from-ztavern-accent-light/20 via-transparent to-ztavern-accent-dark/20 blur-sm"></div>
        </div>
      </div>

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={onVoiceToggle}
              disabled={voiceState === 'transcribing'}
              className={`
                btn-glass text-xs px-4 py-2 rounded-full font-medium
                flex items-center gap-2 transition-all duration-200
                ${voiceState === 'recording'
                  ? 'bg-red-500/20 text-red-600 border-red-400/30 dark:bg-red-500/10 dark:text-red-400'
                  : ''
                }
                disabled:opacity-50 disabled:cursor-not-allowed
                hover:scale-105 ripple-effect
              `}
            >
              {voiceState === 'recording' ? (
                <>
                  <span className="relative flex h-2 w-2">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-red-400 opacity-75"></span>
                    <span className="relative inline-flex rounded-full h-2 w-2 bg-red-500"></span>
                  </span>
                  停止录音
                </>
              ) : (
                <>🎙️ 语音输入</>
              )}
            </button>

            <div className="hidden sm:flex items-center gap-1 text-xs text-ztavern-muted-light dark:text-ztavern-muted-dark">
              <kbd className="px-1.5 py-0.5 bg-ztavern-surface-light dark:bg-ztavern-surface-dark rounded border border-ztavern-border-light dark:border-ztavern-border-dark text-[10px]">
                Shift
              </kbd>
              <span>+</span>
              <kbd className="px-1.5 py-0.5 bg-ztavern-surface-light dark:bg-ztavern-surface-dark rounded border border-ztavern-border-light dark:border-ztavern-border-dark text-[10px]">
                Enter
              </kbd>
              <span>换行</span>
            </div>
          </div>

          <div className="flex flex-col gap-1 text-xs">
            {voiceState === 'connecting' && (
              <div className="flex items-center gap-2 text-ztavern-accent-light dark:text-ztavern-accent-dark">
                <div className="flex gap-1">
                  <div className="h-1 w-1 rounded-full bg-current animate-pulse"></div>
                  <div className="h-1 w-1 rounded-full bg-current animate-pulse" style={{ animationDelay: '0.2s' }}></div>
                  <div className="h-1 w-1 rounded-full bg-current animate-pulse" style={{ animationDelay: '0.4s' }}></div>
                </div>
                正在连接语音服务…
              </div>
            )}

            {voiceState === 'recording' && (
              <div className="flex items-center gap-2 text-amber-500 dark:text-amber-400">
                <div className="relative flex h-2 w-2">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-2 w-2 bg-amber-500"></span>
                </div>
                录音中…再次点击停止
              </div>
            )}

            {voiceState === 'transcribing' && (
              <div className="flex items-center gap-2 text-ztavern-accent-light dark:text-ztavern-accent-dark">
                <div className="flex gap-1">
                  <div className="h-1 w-1 rounded-full bg-current animate-bounce"></div>
                  <div className="h-1 w-1 rounded-full bg-current animate-bounce" style={{ animationDelay: '0.1s' }}></div>
                  <div className="h-1 w-1 rounded-full bg-current animate-bounce" style={{ animationDelay: '0.2s' }}></div>
                </div>
                正在转写语音…
              </div>
            )}

            {voiceState === 'streaming' && (
              <span className="text-ztavern-accent-light dark:text-ztavern-accent-dark">
                等待 AI 流式回复…
              </span>
            )}
          {voiceStatusMessage && (
            <span
              className={voiceStatusClass}
              role={voiceStatusTone === 'error' ? 'alert' : undefined}
            >
              {voiceStatusMessage}
            </span>
          )}
            {voiceError && (
              <span className="text-red-500 dark:text-red-400" role="alert">
                {voiceError}
              </span>
            )}
          </div>
        </div>
        <button
          type="button"
          onClick={onSend}
          disabled={disabled || value.trim().length === 0}
          className={`
            btn-primary relative overflow-hidden group
            ${disabled || value.trim().length === 0
              ? 'opacity-50 cursor-not-allowed'
              : 'hover:scale-105 active:scale-95'
            }
            transition-all duration-200
          `}
        >
          {/* 按钮光效 */}
          <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/20 to-transparent -translate-x-full group-hover:translate-x-full transition-transform duration-500"></div>

          <span className="relative flex items-center gap-2">
            发送消息
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <line x1="22" y1="2" x2="11" y2="13"></line>
              <polygon points="22,2 15,22 11,13 2,9 22,2"></polygon>
            </svg>
          </span>
        </button>
      </div>
    </div>
  )
}
