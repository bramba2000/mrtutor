import { Outlet, createRootRouteWithContext } from '@tanstack/react-router'
import { TanStackRouterDevtoolsPanel } from '@tanstack/react-router-devtools'
import { ReactQueryDevtoolsPanel } from '@tanstack/react-query-devtools'
import { TanStackDevtools } from '@tanstack/react-devtools'
import { MantineProvider } from '@mantine/core'

import '@mantine/core/styles.css'
import '../global.css'
import type { QueryClient } from '@tanstack/react-query'

type RootContext = {
  queryClient: QueryClient
}

export const Route = createRootRouteWithContext<RootContext>()({
  component: RootComponent,
})

function RootComponent() {
  return (
    <>
      <MantineProvider>
        <Outlet />
      </MantineProvider>
      <TanStackDevtools
        config={{
          position: 'bottom-right',
        }}
        plugins={[
          {
            name: 'TanStack Router',
            render: <TanStackRouterDevtoolsPanel />,
          },
          {
            name: 'TanStack Query',
            render: <ReactQueryDevtoolsPanel />,
          },
        ]}
      />
    </>
  )
}
