import { defineConfig } from 'vite'
import { devtools } from '@tanstack/devtools-vite'

import { tanstackRouter } from '@tanstack/router-plugin/vite'

import viteReact from '@vitejs/plugin-react'
import mantinePreset from 'postcss-preset-mantine'
import postcssSimpleVars from 'postcss-simple-vars'

const backend = process.env.BACKEND_URL ?? 'http://localhost:8080'

const config = defineConfig({
  resolve: { tsconfigPaths: true },
  plugins: [
    devtools(),
    tanstackRouter({ target: 'react', autoCodeSplitting: true }),
    viteReact(),
  ],
  server: { port: 3000, proxy: { '/api': { target: backend } } },
  preview: { port: 3000, proxy: { '/api': { target: backend } } },
  build: {
    outDir: './dist',
  },
  css: {
    postcss: {
      plugins: [
        mantinePreset(),
        postcssSimpleVars({
          variables: {
            'mantine-breakpoint-xs': '36em',
            'mantine-breakpoint-sm': '48em',
            'mantine-breakpoint-md': '62em',
            'mantine-breakpoint-lg': '75em',
            'mantine-breakpoint-xl': '88em',
          },
        }),
      ],
    },
  },
})

export default config
