import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
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
