export interface Student {
  id: number
  displayName: string
  email: string
  phone: string
  school: string
  studyProgram: string
  class: string
  birthDate: string
  createdAt: string
  modifiedAt: string
}

export interface CreateStudentRequest {
  displayName: string
  email: string
  phone: string
  school: string
  studyProgram: string
  class: string
  birthDate: string
}

export type UpdateStudentRequest = CreateStudentRequest & {
  id: number
}
