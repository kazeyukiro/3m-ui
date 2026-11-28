import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  // Relative base so the built assets (JS/CSS) are referenced relative to the
  // current document URL. This keeps the SPA working whether it is served from
  // the domain root or behind a random panel base path (server.web_path), which
  // is stripped server-side before the request reaches the SPA.
  base: './',
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})
