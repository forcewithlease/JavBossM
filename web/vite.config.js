import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const backendTarget = process.env.JAVBOSS_API_TARGET || 'http://localhost:17654'
const backendProxy = () => ({ target: backendTarget, changeOrigin: false })

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/healthz': backendProxy(),
      '/auth': backendProxy(),
      '/videos': backendProxy(),
      '/tags': backendProxy(),
      '/sync': backendProxy(),
      '/system': backendProxy(),
      '/directories': backendProxy(),
      '/downloader': backendProxy(),
      '/downloads': backendProxy(),
      '/extension': backendProxy(),
      '/jav': backendProxy(),
      '/config': backendProxy(),
      '/tools': backendProxy(),
    },
  },
})
