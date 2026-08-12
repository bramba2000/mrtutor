import {
  Alert,
  Button,
  Center,
  Paper,
  Stack,
  TextInput,
  Title,
  Text,
} from '@mantine/core'
import { useNavigate } from '@tanstack/react-router'
import type { SubmitEvent } from 'react'
import { useLoginMutation } from '#/features/auth/queries'
import { ApiError, INVALID_CREDENTIALS_CODE } from '#/lib/api'
import { RouterAncor } from '#/components/RouterAncor'

export function LoginForm({ redirectTo }: { redirectTo: string }) {
  const navigate = useNavigate()
  const mutation = useLoginMutation()

  const handleSubmit = (e: SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    const formData = new FormData(e.currentTarget)
    mutation.mutate(
      {
        token: formData.get('username') as string,
        password: formData.get('password') as string,
      },
      {
        onSuccess: () => navigate({ to: redirectTo, replace: true }),
      },
    )
  }

  const error = mutation.error
  const message =
    error instanceof ApiError && error.code === INVALID_CREDENTIALS_CODE
      ? error.message
      : error
        ? 'An unknown error occurred'
        : ''

  return (
    <Center mx="10rem" h="100vh">
      <Paper p="xl" radius="0" withBorder shadow="sm" miw="400px">
        <Title size="h3" mb="md" ta="center">
          Login
        </Title>
        <Text ta="center">
          Do not have an account yet?{' '}
          <RouterAncor to="/register">Register</RouterAncor>
        </Text>
        <form onSubmit={handleSubmit}>
          <Stack>
            <Alert
              color="red"
              variant="light"
              hidden={!message}
              style={{ textTransform: 'capitalize' }}
            >
              {message}
            </Alert>
            <TextInput
              label="Username"
              placeholder="Enter your username"
              name="username"
              autoComplete="username webauthn"
              required
            />
            <TextInput
              label="Password"
              placeholder="Enter your password"
              type="password"
              name="password"
              autoComplete="current-password webauthn"
              required
            />
            <Button type="submit" loading={mutation.isPending}>
              Login
            </Button>
          </Stack>
        </form>
      </Paper>
    </Center>
  )
}
