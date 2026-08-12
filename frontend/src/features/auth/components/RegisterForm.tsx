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
import { useRegisterMutation } from '#/features/auth/queries'
import { ApiError, PRINCIPAL_CONFLICT_CODE } from '#/lib/api'
import { RouterAncor } from '#/components/RouterAncor'

export function RegisterForm() {
  const navigate = useNavigate()
  const mutation = useRegisterMutation()

  const handleSubmit = (e: SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    const formData = new FormData(e.currentTarget)
    mutation.mutate(
      {
        username: formData.get('username') as string,
        email: formData.get('email') as string,
        password: formData.get('password') as string,
      },
      {
        onSuccess: () => navigate({ to: '/', replace: true }),
      },
    )
  }

  const error = mutation.error
  // validation failures come back as HTTP 400 with per-field `fields`, not a
  // meaningful `code` (see backend/httpx/errors.go) — branch on status, not code
  const isFieldError =
    error instanceof ApiError && error.status === 400 && !!error.fields?.length
  const fieldErrors = error instanceof ApiError ? error.fieldErrors() : {}
  const message = isFieldError
    ? ''
    : error instanceof ApiError && error.code === PRINCIPAL_CONFLICT_CODE
      ? error.message
      : error
        ? 'An unknown error occurred'
        : ''

  return (
    <Center mx="10rem" h="100vh">
      <Paper p="xl" radius="0" withBorder shadow="sm" miw="400px">
        <Title size="h3" mb="md" ta="center">
          Register
        </Title>
        <Text ta="center">
          You already have an account?{' '}
          <RouterAncor to="/login" search={{ redirect: '/' }}>
            Log in
          </RouterAncor>
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
              placeholder="Choose a username"
              name="username"
              autoComplete="username webauthn"
              error={fieldErrors.username}
              required
            />
            <TextInput
              label="Email"
              placeholder="Enter your email"
              name="email"
              type="email"
              autoComplete="email"
              error={fieldErrors.email}
              required
            />
            <TextInput
              label="Password"
              placeholder="Choose a password"
              type="password"
              name="password"
              autoComplete="new-password"
              error={fieldErrors.password}
              required
            />
            <Button type="submit" loading={mutation.isPending}>
              Register
            </Button>
          </Stack>
        </form>
      </Paper>
    </Center>
  )
}
