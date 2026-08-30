import { RouterNavLink } from '#/components/RouterNavLink'
import { ChalkboardTeacherIcon } from '@phosphor-icons/react'

// Navbar menu entry that links to the tutor profile page
export function TutorProfileButton() {
  return (
    <RouterNavLink
      to="/tutors/profile"
      leftSection={<ChalkboardTeacherIcon size={16} />}
      label="Tutor Profile"
    />
  )
}
