import { PersonaSidebar } from './PersonaSidebar'
import { ChatTimeline } from './ChatTimeline'
import { ChatComposer } from './ChatComposer'
import { PersonaProfile } from './PersonaProfile'
import { useChatOrchestrator } from '../../hooks/useChatOrchestrator'

export const ChatExperience: React.FC = () => {
  const {
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
    refresh,
    voiceState,
    voiceError,
    speechStatusMessage,
    speechStatusTone,
    toggleVoiceInput,
    playAssistantAudio,
    ttsLoadingMessageId,
    ttsPlayingMessageId,
  } = useChatOrchestrator()

  if (loading && personas.length === 0) {
    return (
      <section className="glass-panel flex min-h-[360px] flex-col items-center justify-center gap-3 border border-white/40 p-10 text-sm text-slate-500 dark:text-slate-400">
        <span className="h-3 w-3 animate-pulse rounded-full bg-sky-400" aria-hidden />
        <p>正在从后端加载角色，请稍候…</p>
      </section>
    )
  }

  if (!loading && personas.length === 0) {
    return (
      <section className="glass-panel flex min-h-[360px] flex-col items-center justify-center gap-4 border border-white/40 p-10 text-center text-sm text-slate-500 dark:text-slate-400">
        <p>{error ?? '暂未获取到角色数据，请稍后重试。'}</p>
        <button
          type="button"
          onClick={refresh}
          className="inline-flex items-center justify-center rounded-full border border-sky-300/60 bg-white/70 px-4 py-2 text-xs font-medium text-sky-600 transition hover:border-sky-400/80 hover:bg-white/90 dark:border-sky-500/50 dark:bg-slate-900/60 dark:text-sky-300 dark:hover:border-sky-400"
        >
          重新请求后端
        </button>
      </section>
    )
  }

  if (!activePersona) {
    return <p className="text-center text-sm text-slate-500">尚未加载角色数据。</p>
  }

  return (
    <section className="grid h-full min-h-0 gap-4 lg:grid-cols-[320px,1fr,320px]">
      <PersonaSidebar
        personas={personas}
        activePersonaId={activePersona.id}
        searchTerm={searchTerm}
        onSearchChange={setSearchTerm}
        onSelectPersona={selectPersona}
        onShuffle={shufflePersona}
        tags={personaTags}
        onTagClick={setSearchTerm}
        isLoading={loading}
      />

      <div className="flex flex-1 min-h-0 flex-col gap-5 overflow-hidden h-full">
        {error && (
          <div className="rounded-2xl border border-amber-300/60 bg-amber-100/60 px-4 py-2 text-xs text-amber-600 shadow-inner dark:border-amber-500/50 dark:bg-amber-500/10 dark:text-amber-200">
            {error}
          </div>
        )}
        {/* {activeConversation?.session && (
          <div className="rounded-2xl border border-sky-300/60 bg-sky-100/60 px-4 py-2 text-xs text-sky-700 shadow-inner dark:border-sky-500/50 dark:bg-sky-500/10 dark:text-sky-200">
            提示：切换角色后无需等待声线同步，使用当前会话调用语音合成即可自动匹配角色音色。若返回 404 或 400，请检查会话是否仍然有效且角色绑定是否正确。
          </div>
        )} */}
        <ChatTimeline
          persona={activePersona}
          messages={activeConversation?.messages ?? []}
          isStreaming={activeConversation?.isStreaming}
          sessionId={activeConversation?.session?.id}
          ttsLoadingMessageId={ttsLoadingMessageId}
          ttsPlayingMessageId={ttsPlayingMessageId}
          onPlayVoice={
            activePersona
              ? (messageId) => playAssistantAudio(activePersona.id, messageId)
              : undefined
          }
        />
        <ChatComposer
          value={composerText}
          onChange={setComposerText}
          onSend={() => sendMessage(composerText)}
          onPromptSelect={appendPrompt}
          onVoiceToggle={toggleVoiceInput}
          voiceState={voiceState}
          voiceError={voiceError}
          voiceStatusMessage={speechStatusMessage}
          voiceStatusTone={speechStatusTone}
        />
      </div>

      <PersonaProfile persona={activePersona} />
    </section>
  )
}
