import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

import { resolve } from 'node:path'

export default defineConfig({
  plugins: [react()],
  base: './',
  resolve: {
    alias: {
      '@shared': resolve(__dirname, '../shared'),
    },
  },
  server: {
    fs: { allow: [resolve(__dirname, '..')] },
    proxy: {
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true },
    },
  },
  build: { outDir: 'dist', emptyOutDir: true },
})
