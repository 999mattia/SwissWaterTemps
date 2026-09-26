import { defineConfig, minimal2023Preset } from '@vite-pwa/assets-generator/config';

export default defineConfig({
  headLinkOptions: { preset: '2023' },
  preset: {
    ...minimal2023Preset,
    // iOS ignores transparency in home screen icons, so fill the background.
    apple: { ...minimal2023Preset.apple, resizeOptions: { background: '#0b6fa4' } },
    maskable: { ...minimal2023Preset.maskable, resizeOptions: { background: '#0b6fa4' } },
  },
  images: ['public/favicon.svg'],
});
