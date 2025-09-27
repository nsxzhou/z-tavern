import type { Persona } from '../../api'
import { createPersonaFilterTags, getPersonaInitials, getPersonaPrompts } from '../../utils/persona'

interface PersonaProfileProps {
  persona: Persona
}

export const PersonaProfile: React.FC<PersonaProfileProps> = ({ persona }) => {
  const prompts = getPersonaPrompts(persona)
  const tags = createPersonaFilterTags([persona])
  const openingLine = persona.openingLine?.trim() || '欢迎来到 Z Tavern，开始对话吧。'

  return (
    <aside className="glass-panel flex h-full min-h-[560px] w-full max-w-sm flex-col gap-6 p-6 animate-glass-morph">
      {/* 角色头像和基本信息 */}
      <div className="flex items-center gap-4">
        <div className="relative group">
          {/* 外层光环效果 */}
          <div className="absolute -inset-2 rounded-2xl bg-gradient-to-br from-ztavern-accent-light/30 to-ztavern-accent-dark/30 blur-lg opacity-0 group-hover:opacity-100 transition-all duration-500"></div>

          {/* 主头像容器 */}
          <div className="relative h-16 w-16 rounded-2xl bg-gradient-to-br from-ztavern-accent-light via-ztavern-accent-dark to-ztavern-accent-light shadow-xl shadow-ztavern-accent-light/40 transition-all duration-300 group-hover:scale-110">
            {/* 内层玻璃效果 */}
            <div className="absolute inset-1 flex items-center justify-center rounded-2xl glass-strong backdrop-blur-strong backdrop-saturate-220 backdrop-brightness-115">
              <span className="text-base font-semibold text-ztavern-text-light dark:text-ztavern-text-dark">
                {getPersonaInitials(persona)}
              </span>
            </div>

            {/* 边缘光效 */}
            <div className="absolute inset-0 rounded-2xl ring-1 ring-white/20 dark:ring-white/10"></div>
          </div>
        </div>

        <div className="flex-1">
          <p className="text-xs uppercase tracking-widest text-ztavern-text-secondary-light dark:text-ztavern-text-secondary-dark mb-1">
            当前角色
          </p>
          <h2 className="text-xl font-semibold text-ztavern-text-light dark:text-ztavern-text-dark mb-1">
            {persona.name}
          </h2>
          <p className="text-sm text-ztavern-muted-light dark:text-ztavern-muted-dark">
            {persona.tone}
          </p>
        </div>
      </div>

      {/* 开场白卡片 */}
      <div className="glass-strong rounded-2xl p-4 text-sm backdrop-blur-strong backdrop-saturate-220 backdrop-brightness-115 shadow-inner relative overflow-hidden">
        {/* 内发光效果 */}
        <div className="absolute inset-0 bg-gradient-to-br from-ztavern-accent-light/5 via-transparent to-ztavern-accent-dark/5 pointer-events-none"></div>

        <div className="relative">
          <p className="font-medium text-ztavern-text-secondary-light dark:text-ztavern-text-secondary-dark mb-2">
            开场白
          </p>
          <p className="leading-relaxed text-ztavern-text-light dark:text-ztavern-text-dark">
            {openingLine}
          </p>
        </div>
      </div>

      {/* 详细信息网格 */}
      <div className="grid gap-4 text-sm flex-1">
        {/* 角色提示 */}
        <div className="glass-panel rounded-2xl p-4 backdrop-blur-glass backdrop-saturate-180 backdrop-brightness-108">
          <p className="text-xs uppercase tracking-widest text-ztavern-text-secondary-light dark:text-ztavern-text-secondary-dark mb-2">
            角色提示
          </p>
          <p className="text-sm leading-relaxed text-ztavern-text-light dark:text-ztavern-text-dark">
            {persona.promptHint || '该角色暂无额外提示信息。'}
          </p>
        </div>

        {/* 风格标签 */}
        <div className="glass-panel rounded-2xl p-4 backdrop-blur-glass backdrop-saturate-180 backdrop-brightness-108">
          <p className="text-xs uppercase tracking-widest text-ztavern-text-secondary-light dark:text-ztavern-text-secondary-dark mb-3">
            风格标签
          </p>
          <div className="flex flex-wrap gap-2">
            {tags.length > 0 ? (
              tags.map((tag) => (
                <span
                  key={tag}
                  className="inline-flex items-center rounded-full px-3 py-1.5 text-xs font-medium glass-strong backdrop-blur-md backdrop-saturate-150 text-ztavern-text-light dark:text-ztavern-text-dark transition-all duration-200 hover:scale-105 cursor-default"
                >
                  {tag}
                </span>
              ))
            ) : (
              <span className="text-xs text-ztavern-muted-light dark:text-ztavern-muted-dark">
                暂未提供标签
              </span>
            )}
          </div>
        </div>

        {/* 快速提示 */}
        <div className="glass-panel rounded-2xl p-4 backdrop-blur-glass backdrop-saturate-180 backdrop-brightness-108 flex-1">
          <p className="text-xs uppercase tracking-widest text-ztavern-text-secondary-light dark:text-ztavern-text-secondary-dark mb-3">
            快速提示
          </p>
          <div className="space-y-2">
            {prompts.length > 0 ? (
              prompts.map((prompt, index) => (
                <div
                  key={prompt}
                  className="flex items-start gap-2 group cursor-default"
                  style={{ animationDelay: `${index * 100}ms` }}
                >
                  <span className="mt-1.5 h-1.5 w-1.5 rounded-full bg-ztavern-accent-light/60 dark:bg-ztavern-accent-dark/60 transition-colors group-hover:bg-ztavern-accent-light dark:group-hover:bg-ztavern-accent-dark"></span>
                  <p className="text-xs leading-relaxed text-ztavern-muted-light dark:text-ztavern-muted-dark group-hover:text-ztavern-text-light dark:group-hover:text-ztavern-text-dark transition-colors">
                    {prompt}
                  </p>
                </div>
              ))
            ) : (
              <p className="text-xs text-ztavern-muted-light dark:text-ztavern-muted-dark">
                暂无提示语，可直接开始对话。
              </p>
            )}
          </div>
        </div>
      </div>
    </aside>
  )
}
