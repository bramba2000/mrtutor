import { useNavigate } from '@tanstack/react-router'
import { useLogoutMutation } from '#/features/auth/queries'
import { SignOutIcon } from '@phosphor-icons/react'
import { AppBarActionIcon } from '#/components/AppBarActionIcon'

export function LogoutButton() {
  const navigate = useNavigate()
  const mutation = useLogoutMutation()

  return (
    <AppBarActionIcon
      aria-label="Log out"
      icon={<SignOutIcon />}
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
    />
  )
}
