import { RouterAncor } from '#/components/RouterAncor'
import { ActionIcon } from '@mantine/core'
import { ChalkboardTeacherIcon } from '@phosphor-icons/react'

// Action button that redirects to the tutor profile page
export function TutorProfileButton() {
  return (
    <ActionIcon
      variant="subtle"
      renderRoot={(props) => (
        <RouterAncor {...props} to="/tutors/profile"></RouterAncor>
      )}
    >
      <ChalkboardTeacherIcon size={20} />
    </ActionIcon>
  )
}
