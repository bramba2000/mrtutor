import { Group } from '@mantine/core'
import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'
import { LogoutButton } from '#/features/auth/components/LogoutButton'
import { meQueryOptions } from '#/features/auth/queries'
import { ApiError, UNAUTHENTICATED_CODE } from '#/lib/api'

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
      <Group justify="flex-end" p="sm">
        <LogoutButton />
      </Group>
      <Outlet />
    </>
  )
}
