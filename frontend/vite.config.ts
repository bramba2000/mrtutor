import { defineConfig } from 'vite'
import { devtools } from '@tanstack/devtools-vite'

import { tanstackRouter } from '@tanstack/router-plugin/vite'

import viteReact from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// The backend is reached through this proxy rather than by absolute URL so
// the browser sees a single origin in dev. That is what lets the session
// cookie (SameSite=Lax, Secure only in production) work over plain HTTP, and
// it is why the backend needs no CORS middleware.
const backend = process.env.BACKEND_URL ?? 'http://localhost:8080'

const config = defineConfig({
  resolve: { tsconfigPaths: true },
  plugins: [
    devtools(),
    tailwindcss(),
    tanstackRouter({ target: 'react', autoCodeSplitting: true }),
    viteReact(),
  ],
  server: { port: 3000, proxy: { '/api': { target: backend } } },
  preview: { port: 3000, proxy: { '/api': { target: backend } } },
  build: {
    // Feeds backend/web's //go:embed. emptyOutDir stays false so the
    // committed dist/.gitkeep placeholder survives the build — see
    // backend/web/embed.go.
    outDir: '../backend/web/dist',
    emptyOutDir: false,
  },
})

export default config
