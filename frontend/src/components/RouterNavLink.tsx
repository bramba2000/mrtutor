import { NavLink } from '@mantine/core'
import type { NavLinkProps } from '@mantine/core'
import type { LinkComponent, LinkComponentProps } from '@tanstack/react-router'
import { createLink } from '@tanstack/react-router'
import { forwardRef } from 'react'

type MantineNavLinkProps = Omit<NavLinkProps, 'href'>

const MantineNavLinkComponent = forwardRef<
  HTMLAnchorElement,
  MantineNavLinkProps
>((props, ref) => {
  return <NavLink ref={ref} {...props} />
})

const CreatedNavLink = createLink(MantineNavLinkComponent)

export type RouterNavLinkProps = LinkComponentProps<typeof CreatedNavLink>

export const RouterNavLink: LinkComponent<typeof CreatedNavLink> = (props) => {
  return <CreatedNavLink preload="intent" {...props} />
}
