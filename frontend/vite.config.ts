import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { VitePWA } from 'vite-plugin-pwa';
import { writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

const outDir = resolve(import.meta.dirname, '../web/internal/ui/dist');

export default defineConfig({
  plugins: [
    svelte(),
    {
      // go:embed needs dist/ to contain a file even before the first build,
      // so the committed placeholder is recreated after emptyOutDir wipes it.
      name: 'keep-go-embed-placeholder',
      apply: 'build',
      closeBundle() {
        writeFileSync(resolve(outDir, '.gitkeep'), '');
      },
    },
    VitePWA({
      registerType: 'autoUpdate',
      injectRegister: 'auto',
      includeAssets: ['favicon.ico', 'favicon.svg', 'apple-touch-icon-180x180.png'],
      manifest: {
        name: 'SwissWaterTemps',
        short_name: 'WaterTemps',
        description: 'Aktuelle Wassertemperaturen der Schweizer Seen und Flüsse',
        lang: 'de-CH',
        start_url: '/',
        scope: '/',
        display: 'standalone',
        background_color: '#f4f8fb',
        theme_color: '#0b6fa4',
        icons: [
          { src: 'pwa-64x64.png', sizes: '64x64', type: 'image/png' },
          { src: 'pwa-192x192.png', sizes: '192x192', type: 'image/png' },
          { src: 'pwa-512x512.png', sizes: '512x512', type: 'image/png' },
          { src: 'maskable-icon-512x512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,svg,png,ico,webmanifest}'],
        navigateFallback: '/index.html',
        navigateFallbackDenylist: [/^\/api\//, /^\/healthz/],
        runtimeCaching: [
          {
            // Fresh data when online, the last response when offline.
            urlPattern: ({ url }) => url.pathname === '/api/v1/stations',
            handler: 'NetworkFirst',
            options: {
              cacheName: 'stations',
              networkTimeoutSeconds: 6,
              expiration: { maxEntries: 1 },
            },
          },
          {
            urlPattern: ({ url }) => /^\/api\/v1\/stations\/[^/]+\/history$/.test(url.pathname),
            handler: 'NetworkFirst',
            options: {
              cacheName: 'station-history',
              networkTimeoutSeconds: 6,
              expiration: { maxEntries: 30, maxAgeSeconds: 7 * 24 * 60 * 60 },
            },
          },
          {
            urlPattern: ({ url }) => url.hostname === 'wmts.geo.admin.ch',
            handler: 'CacheFirst',
            options: {
              cacheName: 'map-tiles',
              expiration: { maxEntries: 500, maxAgeSeconds: 30 * 24 * 60 * 60 },
              cacheableResponse: { statuses: [0, 200] },
            },
          },
        ],
      },
    }),
  ],
  build: {
    // Embedded into the Go binary, see web/internal/ui.
    outDir,
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:3000',
    },
  },
});
