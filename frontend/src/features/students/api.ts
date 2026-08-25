import { apiFetch, post, put } from '#/lib/api'
import type {
  CreateStudentRequest,
  Student,
  UpdateStudentRequest,
} from './types'

export function getStudentById(id: number): Promise<Student> {
  return apiFetch(`/students/${id}`)
}

export function createStudent(student: CreateStudentRequest): Promise<Student> {
  return post('/students/', student)
}

export function updateStudent(student: UpdateStudentRequest): Promise<Student> {
  return put(`/students/${student.id}`, student)
}

export function deleteStudent(id: number): Promise<void> {
  return apiFetch(`/students/${id}`, { method: 'DELETE' })
}

export function getStudentSchools(): Promise<string[]> {
  return apiFetch('/students/schools')
}

export function getStudentStudyPrograms(): Promise<string[]> {
  return apiFetch('/students/study-programs')
}

export function getStudentClasses(): Promise<string[]> {
  return apiFetch('/students/classes')
}
