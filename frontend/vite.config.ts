import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const base = process.env.VITE_BASE ?? '/legacy/'

export default defineConfig({
  base,
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api/v1/docs': {
        target: 'http://127.0.0.1:8080',
        rewrite: (path) => path.replace(/^\/api\/v1\/docs/, '/swagger'),
      },
      '/api': {
        target: 'http://127.0.0.1:8080',
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
    },
  },
})
