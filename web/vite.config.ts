import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  // The Go package internal/spa embeds the build (go:embed only sees files at
  // or below its package). Vite empties a folder outside web/ only when told.
  build: { outDir: '../internal/spa/dist', emptyOutDir: true },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test-setup.ts',
    // Run the tests west of UTC, where "YYYY-MM-DD" parsed as a Date is the
    // day before: date bugs fail here instead of only for some users.
    env: { TZ: 'America/Los_Angeles' },
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
    },
  },
})
