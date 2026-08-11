import { Box, Title, Text, Code } from '@mantine/core'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/')({ component: Home })

function Home() {
    return (
    <Box style={{padding: '1.6rem'}}>
      <Title size="h1">Welcome to TanStack Start</Title>
      <Text style={{marginTop: '1rem'}}>
        Edit <Code>src/routes/index.tsx</Code> to get started.
      </Text>
    </Box>
  )
}
