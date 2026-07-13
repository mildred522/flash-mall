import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { viteSingleFile } from 'vite-plugin-singlefile';
import path from 'path';

const sharedSrc = path.resolve(__dirname, '../shared/src');

export default defineConfig({
  plugins: [react(), viteSingleFile()],
  resolve: {
    alias: {
      '@flash-mall/shared': sharedSrc,
    },
  },
  optimizeDeps: {
    include: ['react', 'react-dom', 'react-router-dom'],
  },
  server: {
    port: 3000,
    proxy: {
      '/api': 'http://localhost:8888',
    },
  },
  build: {
    outDir: 'dist',
  },
});
