import { defineConfig } from 'vitest/config'
import { svelte } from '@sveltejs/vite-plugin-svelte'

export default defineConfig({
  resolve: {
    conditions: ['browser'],
  },
  plugins: [
    svelte({
      hot: false,
      compilerOptions: {
        // Force DOM-mode compilation so onMount callbacks fire in jsdom
        generate: 'dom',
        hydratable: false,
      },
    }),
  ],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
    include: ['src/**/*.test.ts'],
    coverage: {
      provider: 'v8',
      include: ['src/components/**/*.svelte'],
      reporter: ['text', 'lcov'],
    },
  },
})
