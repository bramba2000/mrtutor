import { AppShell, Burger, Group } from '@mantine/core'
import type { NavLinkProps } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'
import type { LinkOptions } from '@tanstack/react-router'
import { LogoutButton } from '#/features/auth/components/LogoutButton'
import { meQueryOptions } from '#/features/auth/queries'
import { ApiError, UNAUTHENTICATED_CODE } from '#/lib/api'
import { useAppBarControlSize } from '#/hooks/useAppBarControlSize'
import type { Icon } from '@phosphor-icons/react'
import {
  ChalkboardTeacherIcon,
  HouseIcon,
} from '@phosphor-icons/react/dist/ssr'
import { RouterNavLink } from '#/components/RouterNavLink'

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
  const [mobileNavOpened, { toggle: toggleMobileNav }] = useDisclosure(false)
  const [desktopNavOpened, { toggle: toggleDesktopNav }] = useDisclosure(true)
  const { icon: burgerSize } = useAppBarControlSize()

  return (
    <>
      <AppShell
        header={{
          offset: true,
          height: { base: 40, sm: 48, lg: 56 },
        }}
        navbar={{
          width: 250,
          breakpoint: 'sm',
          collapsed: { mobile: !mobileNavOpened, desktop: !desktopNavOpened },
        }}
      >
        <AppShell.Header>
          <Group justify="space-between" h="100%" px="md">
            <Group h="100%" gap="sm">
              <Burger
                opened={mobileNavOpened}
                onClick={toggleMobileNav}
                hiddenFrom="sm"
                size={burgerSize}
                aria-label={
                  mobileNavOpened ? 'Close navigation' : 'Open navigation'
                }
              />
              <Burger
                opened={desktopNavOpened}
                onClick={toggleDesktopNav}
                visibleFrom="sm"
                size={burgerSize}
                aria-label={
                  desktopNavOpened ? 'Close navigation' : 'Open navigation'
                }
              />
            </Group>
            <LogoutButton />
          </Group>
        </AppShell.Header>
        <AppShell.Navbar>
          {navItems.map(({ icon: Icon, label, children, ...props }) => (
            <RouterNavLink
              key={label}
              {...props}
              leftSection={<Icon size={16} />}
              label={label}
            />
          ))}
        </AppShell.Navbar>
        <AppShell.Main>
          <Outlet />
        </AppShell.Main>
      </AppShell>
    </>
  )
}

type NavItemsProps = Omit<LinkOptions, 'children'> &
  Pick<NavLinkProps, 'children'> & {
    label: string
    icon: Icon
  }

const navItems: NavItemsProps[] = [
  {
    label: 'Dashboard',
    to: '/',
    icon: HouseIcon,
  },
  {
    label: 'Profile',
    to: '/tutors/profile',
    icon: ChalkboardTeacherIcon,
  },
]
