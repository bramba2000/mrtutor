import { createFileRoute, redirect } from '@tanstack/react-router'
import { LoginForm } from '#/features/auth/components/LoginForm'
import { meQueryOptions } from '#/features/auth/queries'
import { ApiError, UNAUTHENTICATED_CODE } from '#/lib/api'

export const Route = createFileRoute('/(auth)/login')({
  validateSearch: (search) => ({
    redirect: (search.redirect as string) || '/',
  }),
  beforeLoad: async ({ search, context }) => {
    try {
      await context.queryClient.ensureQueryData(meQueryOptions())
    } catch (err) {
      if (err instanceof ApiError && err.code === UNAUTHENTICATED_CODE) {
        return
      }
      throw err
    }
    throw redirect({ to: search.redirect, replace: true })
  },
  component: RouteComponent,
})

function RouteComponent() {
  const { redirect: redirectTo } = Route.useSearch()
  return <LoginForm redirectTo={redirectTo} />
}
