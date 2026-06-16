import { defineConfig, type Plugin } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react(), utf8ContentType()],
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

function utf8ContentType(): Plugin {
  return {
    name: 'paas-utf8-content-type',
    configureServer(server) {
      server.middlewares.use((_, res, next) => {
        const setHeader = res.setHeader.bind(res);
        res.setHeader = (name, value) => {
          if (typeof name === 'string' && name.toLowerCase() === 'content-type' && typeof value === 'string') {
            return setHeader(name, withUTF8Charset(value));
          }
          return setHeader(name, value);
        };
        next();
      });
    }
  };
}

function withUTF8Charset(contentType: string): string {
  if (/(^|;) charset=/i.test(contentType)) {
    return contentType;
  }
  if (/^(text\/html|text\/javascript|application\/javascript|text\/css|application\/json|text\/plain)(;|$)/i.test(contentType)) {
    return `${contentType}; charset=utf-8`;
  }
  return contentType;
}
