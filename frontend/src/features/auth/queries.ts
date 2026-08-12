import {
  queryOptions,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query'
import * as authApi from '#/features/auth/api'
import type { LoginRequest, RegisterRequest } from '#/features/auth/types'

export const authKeys = {
  all: ['auth'] as const,
  me: () => [...authKeys.all, 'me'] as const,
}

export function meQueryOptions() {
  return queryOptions({
    queryKey: authKeys.me(),
    queryFn: authApi.me,
    retry: false,
    refetchOnWindowFocus: true,
    refetchOnReconnect: true,
  })
}

export function useLoginMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (request: LoginRequest) => authApi.login(request),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: authKeys.all }),
  })
}

export function useRegisterMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (request: RegisterRequest) => authApi.register(request),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: authKeys.all }),
  })
}

export function useLogoutMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => authApi.logout(),
    // a stale cache after logout would leak the previous user's data into the next session
    onSuccess: () => queryClient.clear(),
  })
}
