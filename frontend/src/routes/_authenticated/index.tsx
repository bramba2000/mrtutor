import { Box, Text, Title } from '@mantine/core'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/')({ component: Home })

function Home() {
  return (
    <Box p="xl">
      <Title size="h1">Welcome</Title>
      <Text mt="md">You are logged in.</Text>
    </Box>
  )
}
