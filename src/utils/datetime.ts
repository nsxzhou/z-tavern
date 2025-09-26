export const formatTime = (timestamp: number | string) => {
  const date = typeof timestamp === 'number' ? new Date(timestamp) : new Date(timestamp)
  return new Intl.DateTimeFormat('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}
