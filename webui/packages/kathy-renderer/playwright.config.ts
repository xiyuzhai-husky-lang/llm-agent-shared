import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests',
  testMatch: 'browser.spec.ts',
  use: {
    baseURL: 'http://127.0.0.1:5193',
    browserName: 'chromium',
  },
  webServer: {
    command: 'bunx vite tests --host 127.0.0.1 --port 5193',
    url: 'http://127.0.0.1:5193',
    reuseExistingServer: false,
  },
})
