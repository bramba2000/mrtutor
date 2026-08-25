import { AppShell, Group } from '@mantine/core'
import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'
import { LogoutButton } from '#/features/auth/components/LogoutButton'
import { meQueryOptions } from '#/features/auth/queries'
import { ApiError, UNAUTHENTICATED_CODE } from '#/lib/api'
import { TutorProfileButton } from '#/features/tutors/components/TutorProfileButton'

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ context, location }) => {
    try {
      await context.queryClient.ensureQueryData(meQueryOptions())
    } catch (err) {
      if (err instanceof ApiError && err.code === UNAUTHENTICATED_CODE) {
        throw redirect({ to: '/login', search: { redirect: location.href } })
      }
      throw err
    }
  },
  component: RouteComponent,
})

function RouteComponent() {
  return (
    <>
      <AppShell
        header={{
          offset: true,
          height: { base: 40, sm: 48, lg: 56 },
        }}
      >
        <AppShell.Header>
          <Group justify="flex-end" h="100%" px="md">
            <TutorProfileButton />
            <LogoutButton />
          </Group>
        </AppShell.Header>
        <AppShell.Main>
          <Outlet />
        </AppShell.Main>
      </AppShell>
    </>
  )
}
