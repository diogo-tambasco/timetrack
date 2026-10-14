/// <reference types="vitest/config" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { VitePWA } from 'vite-plugin-pwa';

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    // CA-08/CA-14: manifest instalável + precache do app shell (D-06).
    // Fase 16 (CA-17/RNF-06): estratégia injectManifest — SW customizado em
    // src/sw.ts (push + notificationclick); o precache/navigateFallback/
    // denylist que o generateSW dava são RECRIADOS manualmente no sw.ts.
    VitePWA({
      registerType: 'autoUpdate',
      strategies: 'injectManifest',
      srcDir: 'src',
      filename: 'sw.ts',
      manifest: {
        name: 'Timetrack',
        short_name: 'Timetrack',
        display: 'standalone',
        background_color: '#0e0e10',
        theme_color: '#0e0e10',
        icons: [
          { src: '/icons/icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: '/icons/icon-512.png', sizes: '512x512', type: 'image/png' },
        ],
      },
      injectManifest: {
        globPatterns: ['**/*.{js,css,html,woff2,webmanifest,png}'],
      },
    }),
  ],
  server: {
    port: 5173,
    proxy: {
      // ws:true encaminha o upgrade do /api/v1/ws (fase 04). SEM
      // changeOrigin: com ele o Host vira :8080 mas o Origin do browser
      // segue :5173/:4173 e o origin check do backend reprova com 403 SÓ
      // no dev/preview (plan §6 — armadilha vetada).
      '/api': { target: 'http://localhost:8080', ws: true },
      '/healthz': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
    },
  },
  // o E2E roda contra `vite preview` (4173) — proxy explícito, igual ao dev
  preview: {
    proxy: {
      '/api': { target: 'http://localhost:8080', ws: true },
      '/healthz': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
  },
});
