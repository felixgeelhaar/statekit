import { defineConfig } from 'astro/config';
import vue from '@astrojs/vue';

export default defineConfig({
  site: 'https://felixgeelhaar.github.io',
  base: '/statekit',
  output: 'static',
  outDir: './dist',
  integrations: [vue()],
  build: {
    format: 'file',
  },
});
