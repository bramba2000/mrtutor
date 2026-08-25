import { TutorProfileForm } from '#/features/tutors/components/TutorProfileForm'
import { getMyTutorProfileQueryOptions } from '#/features/tutors/queries'
import { ApiError } from '#/lib/api'
import { Box, Breadcrumbs, LoadingOverlay, Text } from '@mantine/core'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/tutors/profile')({
  component: RouteComponent,
})

function RouteComponent() {
  const { data, error, isLoading } = useQuery(getMyTutorProfileQueryOptions())

  const notFound = error instanceof ApiError && error.status === 404

  return (
    <Box p="xl" pos="relative">
      <LoadingOverlay visible={isLoading} />
      <Breadcrumbs separator=">" mb="md">
        <Text size="lg" fw="bold">
          Profile
        </Text>
      </Breadcrumbs>
      <TutorProfileForm data={notFound ? undefined : data} />
    </Box>
  )
}
