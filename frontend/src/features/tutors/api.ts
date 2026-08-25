import type { Student } from '#/features/students/types'
import { apiFetch, post, put } from '#/lib/api'
import type { CreateTutorRequest, Tutor, UpdateTutorRequest } from './types'

export function getTutors(): Promise<Tutor[]> {
  return apiFetch('/tutors/')
}

export function getTutorById(id: number): Promise<Tutor> {
  return apiFetch(`/tutors/${id}`)
}

export function getMyTutorProfile(): Promise<Tutor> {
  return apiFetch('/tutors/me')
}

export function getStudentsByTutor(tutorId: number): Promise<Student[]> {
  return apiFetch(`/tutors/${tutorId}/students`)
}

export function createTutor(tutor: CreateTutorRequest): Promise<Tutor> {
  return post('/tutors/', tutor)
}

export function updateTutor(tutor: UpdateTutorRequest): Promise<Tutor> {
  return put(`/tutors/${tutor.id}`, tutor)
}

export function deleteTutor(id: number): Promise<void> {
  return apiFetch(`/tutors/${id}`, { method: 'DELETE' })
}
