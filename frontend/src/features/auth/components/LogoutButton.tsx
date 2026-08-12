import { Button } from '@mantine/core'
import { useNavigate } from '@tanstack/react-router'
import { useLogoutMutation } from '#/features/auth/queries'

export function LogoutButton() {
  const navigate = useNavigate()
  const mutation = useLogoutMutation()

  return (
    <Button
      variant="subtle"
      loading={mutation.isPending}
      onClick={() =>
        mutation.mutate(undefined, {
          onSuccess: () =>
            navigate({
              to: '/login',
              search: { redirect: '/' },
              replace: true,
            }),
        })
      }
    >
      Log out
    </Button>
  )
}
