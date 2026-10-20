import { defineConfig } from 'vitest/config'
import { fileURLToPath } from 'node:url'

export default defineConfig({
  // The adjacent classification tests share this package's installed runner.
  resolve: { alias: { vitest: fileURLToPath(import.meta.resolve('vitest')) } },
  test: {
    include: ['tests/**/*.test.{ts,tsx}', '../classification/**/*.test.ts'],
    environment: 'jsdom',
    // Node 24's optimizing WASM compiler exhausts memory on the full grammar
    // fixture matrix; exercise the same WASM with its baseline compiler.
    execArgv: ['--liftoff-only'],
  },
})
