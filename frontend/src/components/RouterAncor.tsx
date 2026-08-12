import { Anchor } from '@mantine/core'
import type { AnchorProps } from '@mantine/core'
import { createLink } from '@tanstack/react-router'
import type { LinkComponent } from '@tanstack/react-router'
import { forwardRef } from 'react'

export interface RouterAncorProps extends Omit<AnchorProps, 'href'> {}

const MantineLinkComponet = forwardRef<HTMLAnchorElement, RouterAncorProps>(
  ({ ...props }, ref) => {
    return <Anchor ref={ref} {...props} />
  },
)

const CreatedLink = createLink(MantineLinkComponet)

export const RouterAncor: LinkComponent<typeof MantineLinkComponet> = (
  props,
) => {
  return <CreatedLink preload="intent" {...props} />
}
