import { ActionIcon } from '@mantine/core'
import { useNavigate } from '@tanstack/react-router'
import { useLogoutMutation } from '#/features/auth/queries'
import { SignOutIcon } from '@phosphor-icons/react'

export function LogoutButton() {
  const navigate = useNavigate()
  const mutation = useLogoutMutation()

  return (
    <ActionIcon
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
      <SignOutIcon size={20} />
    </ActionIcon>
  )
}
