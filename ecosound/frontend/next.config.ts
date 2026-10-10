import type { NextConfig } from 'next';
const config: NextConfig = { experimental: { workerThreads: true, cpus: 2 } };
export default config;
