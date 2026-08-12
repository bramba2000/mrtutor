import { apiFetch, post } from '#/lib/api'
import type {
  LoginRequest,
  Principal,
  RegisterRequest,
} from '#/features/auth/types'

export function login(request: LoginRequest): Promise<void> {
  return post('/auth/login', request)
}

export function register(request: RegisterRequest): Promise<Principal> {
  return post('/auth/register', request)
}

export function logout(): Promise<void> {
  return post('/auth/logout', undefined)
}

export function me(): Promise<Principal> {
  return apiFetch('/auth/me')
}
