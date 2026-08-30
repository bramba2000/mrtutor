import { ActionIcon } from '@mantine/core'
import { cloneElement } from 'react'
import type { ReactElement } from 'react'
import { useAppBarControlSize } from '#/hooks/useAppBarControlSize'

export interface AppBarActionIconProps {
  icon: ReactElement<{ size?: number }>
  'aria-label': string
  onClick?: () => void
  loading?: boolean
  disabled?: boolean
  renderRoot?: (props: Record<string, unknown>) => React.ReactNode
}

// Uniform action button for the appbar: keeps every icon button the same
// size as the navigation burger, scaled to the current header height.
export function AppBarActionIcon({ icon, ...props }: AppBarActionIconProps) {
  const { control, icon: iconSize } = useAppBarControlSize()

  return (
    <ActionIcon variant="subtle" size={control} {...props}>
      {cloneElement(icon, { size: iconSize })}
    </ActionIcon>
  )
}
