import { ThemeProvider } from './context/ThemeProvider'
import { AppHeader } from './components/layout/AppHeader'
import { ChatExperience } from './components/chat/ChatExperience'

function App() {
  return (
    <ThemeProvider>
      <div className="mx-auto flex min-h-screen max-w-7xl flex-col gap-6 px-4 pb-16 pt-10 sm:px-8 lg:px-10">
        <AppHeader statusMessage="本地联调模式 · 目标端点 http://localhost:8080/api" />
        {/* <StatusBanner message="请确保后端服务已启动，未配置时默认使用 VITE_API_BASE_URL 环境变量" /> */}
        <ChatExperience />
      </div>
    </ThemeProvider>
  )
}

export default App
