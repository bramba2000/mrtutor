// mirrors auth.Principal in backend/auth/models.go
export type Principal = {
  id: number
  username: string
  email: string
  createdAt?: string
  updatedAt?: string
}

// mirrors auth.LoginIn in backend/auth/service.go
export type LoginRequest = {
  token: string
  password: string
}

// mirrors auth.RegisterIn in backend/auth/service.go
export type RegisterRequest = {
  username: string
  email: string
  password: string
}
