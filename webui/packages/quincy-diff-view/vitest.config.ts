import { defineConfig } from 'vitest/config'

export default defineConfig({
  // The existing model tests use Bun's equivalent test API.
  resolve: { alias: { 'bun:test': 'vitest' } },
  test: {
    environment: 'jsdom',
    // Keep DOM-only seam tests out of the package's original `bun test` suite.
    include: ['tests/**/*.test.ts', 'tests/**/*.vitest.tsx'],
  },
})
