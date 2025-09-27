import { useMemo } from 'react'
import type { Persona } from '../../api'
import { getPersonaInitials } from '../../utils/persona'

interface PersonaSidebarProps {
  personas: Persona[]
  activePersonaId?: string
  isLoading?: boolean
  searchTerm: string
  onSearchChange: (keyword: string) => void
  onSelectPersona: (persona: Persona) => void
  onShuffle?: () => void
  tags?: string[]
  onTagClick?: (tag: string) => void
}

export const PersonaSidebar: React.FC<PersonaSidebarProps> = ({
  personas,
  activePersonaId,
  isLoading = false,
  searchTerm,
  onSearchChange,
  onSelectPersona,
  onShuffle,
  tags,
  onTagClick,
}) => {
  const filteredPersonas = useMemo(() => {
    if (!searchTerm) return personas
    const keyword = searchTerm.toLowerCase()
    return personas.filter((persona) => {
      const fields = [
        persona.name,
        persona.title,
        persona.tone,
        ...(persona.traits ?? []),
        ...(persona.expertise ?? []),
      ]
      return fields.some((field) => field?.toLowerCase().includes(keyword))
    })
  }, [personas, searchTerm])

  const shuffleDisabled = !onShuffle || personas.length < 2 || isLoading

  return (
    <aside className="glass-panel flex h-full min-h-[560px] w-full max-w-xs flex-col gap-5 border border-white/40 p-5">
      <div className="flex items-center gap-3">
        <div>
          <h2 className="text-base font-semibold text-slate-800 dark:text-slate-100">角色探索</h2>
          <p className="text-xs text-ztavern-muted-light dark:text-ztavern-muted-dark">
            挑选最合适的酒馆同伴
          </p>
        </div>
        <button
          type="button"
          onClick={shuffleDisabled ? undefined : onShuffle}
          disabled={shuffleDisabled}
          className={`ml-auto inline-flex items-center rounded-full border px-3 py-1.5 text-xs transition ${
            shuffleDisabled
              ? 'cursor-not-allowed border-slate-200/40 bg-white/30 text-slate-400 dark:border-slate-700/40 dark:bg-slate-900/30 dark:text-slate-600'
              : 'border-slate-300/40 bg-white/40 text-slate-600 hover:border-slate-400/70 hover:bg-white/60 dark:border-slate-600/50 dark:text-slate-200 dark:hover:border-slate-500'
          }`}
        >
          随机推荐
        </button>
      </div>

      <div className="persona-search relative">
        <input
          id="persona-search"
          type="search"
          value={searchTerm}
          onChange={(event) => onSearchChange(event.target.value)}
          placeholder="搜索角色或关键词"
          className="w-full rounded-full border border-slate-300/50 bg-white/60 px-4 py-2.5 text-sm text-slate-600 shadow-inner outline-none transition focus:border-slate-400 focus:shadow-lg focus:shadow-slate-200/40 disabled:opacity-60 dark:border-slate-600/50 dark:bg-slate-900/50 dark:text-slate-100 dark:focus:border-slate-400"
          disabled={isLoading}
        />
      </div>

      {tags && tags.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {tags.map((tag) => (
            <button
              key={tag}
              type="button"
              onClick={() => onTagClick?.(tag)}
              className="rounded-full border border-slate-300/40 bg-white/50 px-3 py-1 text-xs text-slate-600 transition hover:border-slate-400/70 hover:bg-white/80 dark:border-slate-600/50 dark:bg-slate-900/40 dark:text-slate-300 dark:hover:border-slate-500"
            >
              #{tag}
            </button>
          ))}
        </div>
      )}

      <div className="relative flex-1 overflow-hidden rounded-2xl border border-slate-200/30 bg-white/40 shadow-inner dark:border-slate-700/40 dark:bg-slate-900/30">
        <div className="absolute inset-0 overflow-y-auto p-2 pr-1">
          {isLoading && (
            <p className="px-3 py-4 text-xs text-slate-500 dark:text-slate-400">加载角色中…</p>
          )}
          {!isLoading && filteredPersonas.length === 0 && (
            <p className="px-3 py-4 text-xs text-slate-500 dark:text-slate-400">
              未找到匹配角色，试试其他关键词。
            </p>
          )}
          {filteredPersonas.map((persona) => {
            const isActive = persona.id === activePersonaId
            const initials = getPersonaInitials(persona)
            return (
              <button
                key={persona.id}
                type="button"
                aria-pressed={isActive}
                onClick={() => onSelectPersona(persona)}
                className={`group relative flex w-full items-center gap-3 rounded-2xl border px-3 py-3 text-left transition ${
                  isActive
                    ? 'border-sky-300/70 bg-white/80 shadow-lg shadow-sky-200/40 dark:border-sky-400/60 dark:bg-slate-900/70'
                    : 'border-transparent bg-white/30 hover:border-slate-200/70 hover:bg-white/60 dark:bg-slate-900/30 dark:hover:border-slate-600/60 dark:hover:bg-slate-900/50'
                }`}
              >
                <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-slate-800 text-sm font-semibold text-white shadow-lg shadow-slate-900/20 dark:bg-slate-100 dark:text-slate-900">
                  {initials}
                </span>
                <span className="flex-1">
                  <span className="flex items-center gap-2">
                    <strong className="text-sm font-semibold text-slate-800 dark:text-slate-100">
                      {persona.name}
                    </strong>
                  </span>
                  <span className="mt-1 block text-[12px] text-slate-500 dark:text-slate-400">
                    {persona.tone || persona.title}
                  </span>
                </span>
              </button>
            )
          })}
        </div>
      </div>
    </aside>
  )
}
