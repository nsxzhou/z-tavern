/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: [
    './index.html',
    './src/**/*.{js,ts,jsx,tsx}',
  ],
  theme: {
    extend: {
      fontFamily: {
        'system': ['-apple-system', 'BlinkMacSystemFont', '"Segoe UI"', 'Roboto', '"Helvetica Neue"', 'Arial', '"Noto Sans"', '"Liberation Sans"', 'sans-serif'],
      },
      colors: {
        // Liquid Glass 核心颜色系统 - 优化版
        'ztavern-surface-light': 'rgba(248, 250, 252, 0.85)',  // 更中性的浅色
        'ztavern-surface-dark': 'rgba(22, 25, 31, 0.85)',     // 统一的深色
        'ztavern-layer-light': 'rgba(255, 255, 255, 0.75)',   // 纯白玻璃层
        'ztavern-layer-dark': 'rgba(28, 32, 38, 0.80)',       // 协调的暗层
        'ztavern-border-light': 'rgba(226, 232, 240, 0.60)',  // 柔和边框
        'ztavern-border-dark': 'rgba(255, 255, 255, 0.08)',   // 统一暗边框
        'ztavern-muted-light': '#64748b',                      // 中性灰色
        'ztavern-muted-dark': '#94a3b8',                       // 柔和暗色
        'ztavern-accent-light': '#0ea5e9',                     // 清新蓝色
        'ztavern-accent-dark': '#38bdf8',                      // 明亮蓝色
        'ztavern-text-light': '#0f172a',                       // 深色文字
        'ztavern-text-dark': '#f8fafc',                        // 亮色文字
        'ztavern-text-secondary-light': '#475569',             // 次要文字
        'ztavern-text-secondary-dark': '#cbd5e1',              // 暗色次要文字
        // 扩展的半透明层次
        'glass-light': 'rgba(255, 255, 255, 0.80)',           // 亮色玻璃
        'glass-dark': 'rgba(0, 0, 0, 0.25)',                  // 暗色玻璃
        'glass-border-light': 'rgba(226, 232, 240, 0.40)',    // 亮色玻璃边框
        'glass-border-dark': 'rgba(255, 255, 255, 0.12)',     // 暗色玻璃边框
      },
      boxShadow: {
        // Liquid Glass 阴影系统 - 优化版
        'glass-light': '0 8px 32px -8px rgba(0, 0, 0, 0.08), 0 4px 16px -4px rgba(0, 0, 0, 0.04)',
        'glass-dark': '0 8px 32px -8px rgba(0, 0, 0, 0.50), 0 4px 16px -4px rgba(0, 0, 0, 0.25)',
        'glass-hover-light': '0 12px 40px -12px rgba(0, 0, 0, 0.12), 0 8px 24px -8px rgba(0, 0, 0, 0.08)',
        'glass-hover-dark': '0 12px 40px -12px rgba(0, 0, 0, 0.60), 0 8px 24px -8px rgba(0, 0, 0, 0.35)',
        'glass-glow-light': '0 0 0 1px rgba(14, 165, 233, 0.15), 0 0 20px -5px rgba(14, 165, 233, 0.25)',
        'glass-glow-dark': '0 0 0 1px rgba(56, 189, 248, 0.20), 0 0 20px -5px rgba(56, 189, 248, 0.35)',
        // 原有阴影保持兼容
        'ztavern-light': '0 22px 60px -32px rgba(15, 23, 42, 0.15), 0 20px 40px -24px rgba(56, 189, 248, 0.20)',
        'ztavern-dark': '0 32px 68px -30px rgba(0, 0, 0, 0.60), 0 20px 48px -22px rgba(56, 189, 248, 0.30)',
        'ztavern-hover': '0 30px 72px -30px rgba(14, 165, 233, 0.25)',
        'ztavern-hover-dark': '0 28px 70px -28px rgba(56, 189, 248, 0.40)',
      },
      borderRadius: {
        // Liquid Glass 圆角规范
        '2xl': '1.5rem',
        'full': '9999px',
      },
      backdropBlur: {
        'glass': '25px',
        'glass-strong': '36px',
      },
      backdropSaturate: {
        '180': '1.8',
        '220': '2.2',
      },
      backdropBrightness: {
        '108': '1.08',
        '115': '1.15',
      },
      animation: {
        // Liquid Glass 流体动画
        'glass-morph': 'glass-morph 0.4s cubic-bezier(0.2, 0.8, 0.2, 1)',
        'glass-hover': 'glass-hover 0.3s cubic-bezier(0.2, 0.8, 0.2, 1)',
        'glass-focus': 'glass-focus 0.2s cubic-bezier(0.2, 0.8, 0.2, 1)',
        'ripple': 'ripple 0.6s cubic-bezier(0, 0, 0.2, 1)',
      },
      transitionTimingFunction: {
        'glass': 'cubic-bezier(0.2, 0.8, 0.2, 1)',
        'glass-spring': 'cubic-bezier(0.34, 1.56, 0.64, 1)',
      },
      transitionDuration: {
        'glass': '400ms',
        'glass-fast': '200ms',
        'glass-slow': '600ms',
      },
      backgroundImage: {
        // 优化后的背景渐变 - 更中性专业
        'ztavern-light': 'linear-gradient(145deg, rgba(248, 250, 252, 0.96) 0%, rgba(241, 245, 249, 0.94) 42%, rgba(236, 240, 246, 0.92) 100%)',
        'ztavern-dark': 'linear-gradient(145deg, #0f1114 0%, #1a1d23 45%, #1e2329 100%)',
        'ztavern-brand': 'linear-gradient(135deg, rgba(14, 165, 233, 0.98), rgba(56, 189, 248, 0.95))',
        'glass-gradient-light': 'linear-gradient(120deg, rgba(255, 255, 255, 0.85) 0%, rgba(248, 250, 252, 0.60) 100%)',
        'glass-gradient-dark': 'linear-gradient(120deg, rgba(28, 32, 38, 0.85) 0%, rgba(22, 25, 31, 0.70) 100%)',
      },
    },
  },
  plugins: [],
}
