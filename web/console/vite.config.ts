import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    fileParallelism: false,
    setupFiles: './src/tests/setup.ts',
    coverage: {
      include: ['src/**/*.{ts,tsx}'],
      exclude: ['src/main.tsx', 'src/tests/**', 'src/vite-env.d.ts', '**/*.test.ts', '**/*.test.tsx']
    }
  }
});
