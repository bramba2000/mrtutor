export interface Tutor {
  id: number
  displayName: string
  email: string
  phone: string
  aboutMe: string
  userId: number
  createdAt: string
  modifiedAt: string
}

export interface CreateTutorRequest {
  displayName: string
  email: string
  phone: string
  aboutMe: string
}

export type UpdateTutorRequest = CreateTutorRequest & {
  id: number
}
