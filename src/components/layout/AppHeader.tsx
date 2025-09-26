import { ThemeToggle } from './ThemeToggle'

interface AppHeaderProps {
  statusMessage?: string
}

export const AppHeader: React.FC<AppHeaderProps> = ({ statusMessage }) => {
  return (
    <header className="glass-panel flex flex-wrap items-center justify-between gap-6 rounded-2xl px-6 py-5 backdrop-blur-glass backdrop-saturate-180 backdrop-brightness-108 relative overflow-hidden animate-glass-morph">
      {/* 背景光效 */}
      <div className="absolute inset-0 bg-gradient-to-r from-ztavern-accent-light/5 via-transparent to-ztavern-accent-dark/5 pointer-events-none opacity-60"></div>

      <div className="flex items-center gap-4 relative z-10">
        {/* Logo 容器 */}
        <div className="relative group">
          {/* 外层光环 */}
          <div className="absolute -inset-1 rounded-full bg-gradient-to-br from-ztavern-accent-light/40 to-ztavern-accent-dark/40 blur-md opacity-0 group-hover:opacity-100 transition-all duration-500"></div>

          {/* 主 Logo */}
          <div className="relative flex h-12 w-12 items-center justify-center rounded-full bg-ztavern-brand shadow-lg shadow-ztavern-accent-light/40 transition-all duration-300 group-hover:scale-110 group-hover:shadow-xl">
            {/* Logo 内部玻璃效果 */}
            <div className="absolute inset-0.5 rounded-full glass-strong backdrop-blur-strong backdrop-saturate-220 backdrop-brightness-115 opacity-20"></div>

            <span className="relative text-xl font-semibold tracking-tight text-white drop-shadow-sm">
              ZT
            </span>

            {/* 边缘光效 */}
            <div className="absolute inset-0 rounded-full ring-1 ring-white/30"></div>
          </div>
        </div>

        {/* 标题和描述 */}
        <div className="flex-1 min-w-0">
          <h1 className="text-lg font-semibold tracking-tight text-ztavern-text-light dark:text-ztavern-text-dark mb-1">
            Z Tavern · Liquid Glass
          </h1>
          <p className="text-sm text-ztavern-muted-light dark:text-ztavern-muted-dark leading-tight">
            角色酒馆体验原型 · 沉浸式对话与语音交互
          </p>
        </div>
      </div>

      {/* 右侧控制区域 */}
      <div className="flex flex-1 items-center justify-end gap-4 relative z-10">
        {/* 状态指示器 */}
        {statusMessage && (
          <div className="glass-strong hidden items-center gap-3 rounded-full px-4 py-2.5 text-sm backdrop-blur-strong backdrop-saturate-220 backdrop-brightness-115 shadow-inner sm:flex relative overflow-hidden">
            {/* 内发光 */}
            <div className="absolute inset-0 bg-gradient-to-r from-emerald-500/5 via-transparent to-emerald-500/5 pointer-events-none"></div>

            {/* 状态指示点 */}
            <div className="relative">
              <span className="flex h-2.5 w-2.5">
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75"></span>
                <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-emerald-500 shadow-sm"></span>
              </span>
            </div>

            <span className="relative text-ztavern-text-light dark:text-ztavern-text-dark font-medium">
              {statusMessage}
            </span>
          </div>
        )}

        {/* 主题切换器 */}
        <div className="relative">
          <ThemeToggle />
        </div>
      </div>
    </header>
  )
}
