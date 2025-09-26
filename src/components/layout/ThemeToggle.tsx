import { useCallback } from 'react'
import { useTheme } from '../../hooks/useTheme'

const Sun = (props: React.SVGProps<SVGSVGElement>) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" {...props}>
    <circle cx="12" cy="12" r="5" />
    <path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" />
  </svg>
)

const Moon = (props: React.SVGProps<SVGSVGElement>) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" {...props}>
    <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79Z" />
  </svg>
)

export const ThemeToggle: React.FC = () => {
  const { theme, toggle } = useTheme()

  const handleToggle = useCallback(() => {
    toggle()
  }, [toggle])

  return (
    <button
      type="button"
      onClick={handleToggle}
      className="btn-glass relative flex h-10 w-20 items-center justify-center rounded-full p-1 transition-all duration-300 hover:scale-105 ripple-effect group"
      aria-label="切换主题"
      aria-pressed={theme === 'dark'}
    >
      {/* 滑动指示器背景 */}
      <div
        className={`
          absolute inset-1 w-8 h-8 rounded-full transition-all duration-300 ease-out
          glass-strong backdrop-blur-strong backdrop-saturate-220 backdrop-brightness-115
          shadow-glass-light dark:shadow-glass-dark
          ${theme === 'dark'
            ? 'translate-x-9 bg-gradient-to-br from-ztavern-accent-dark/20 to-ztavern-accent-dark/10'
            : 'translate-x-0 bg-gradient-to-br from-ztavern-accent-light/20 to-ztavern-accent-light/10'
          }
        `}
      />

      {/* 图标容器 */}
      <div className="relative z-10 flex h-full w-full items-center">
        {/* 太阳图标 */}
        <div className="flex h-8 w-8 items-center justify-center">
          <Sun
            className={`h-4 w-4 transition-all duration-300 ${
              theme === 'light'
                ? 'text-ztavern-accent-light scale-100 opacity-100'
                : 'text-ztavern-muted-light/40 dark:text-ztavern-muted-dark/40 scale-90 opacity-60'
            }`}
          />
        </div>

        {/* 月亮图标 */}
        <div className="flex h-8 w-8 items-center justify-center ml-1">
          <Moon
            className={`h-4 w-4 transition-all duration-300 ${
              theme === 'dark'
                ? 'text-ztavern-accent-dark scale-100 opacity-100'
                : 'text-ztavern-muted-light/40 dark:text-ztavern-muted-dark/40 scale-90 opacity-60'
            }`}
          />
        </div>
      </div>
    </button>
  )
}
