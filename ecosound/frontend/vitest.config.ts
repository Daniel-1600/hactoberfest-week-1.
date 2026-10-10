import { defineConfig } from 'vitest/config';
export default defineConfig({ test: { environment: 'jsdom', globals: true, pool: 'threads', maxWorkers: 1, setupFiles: ['./tests/setup.ts'] } });
