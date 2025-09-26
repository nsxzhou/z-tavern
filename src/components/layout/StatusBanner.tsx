interface StatusBannerProps {
  tone?: 'info' | 'error'
  message: string
  visible?: boolean
}

export const StatusBanner: React.FC<StatusBannerProps> = ({ tone = 'info', message, visible = true }) => {
  if (!visible) return null

  const toneClasses =
    tone === 'error'
      ? 'border-red-300/70 bg-red-100/60 text-red-700 dark:border-red-500/50 dark:bg-red-500/10 dark:text-red-200'
      : 'border-ztavern-border-light bg-ztavern-layer-light text-ztavern-muted-light dark:border-ztavern-border-dark dark:bg-ztavern-layer-dark dark:text-ztavern-muted-dark'

  return (
    <div
      className={`glass-panel mb-6 flex items-center gap-3 border px-5 py-3 text-sm shadow-none ${toneClasses}`}
    >
      <span
        className={`h-2.5 w-2.5 rounded-full ${
          tone === 'error'
            ? 'bg-red-500 shadow-[0_0_0_4px_rgba(248,113,113,0.2)]'
            : 'bg-sky-400 shadow-[0_0_0_4px_rgba(125,211,252,0.26)]'
        }`}
      />
      <span>{message}</span>
    </div>
  )
}
