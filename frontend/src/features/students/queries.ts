import {
  getMyTutorProfileQueryOptions,
  getStudentsByTutorQueryOptions,
} from '#/features/tutors/queries'
import {
  queryOptions,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query'
import * as studentsApi from './api'
import type { Student } from './types'

export const studentsKeys = {
  all: ['students'] as const,
  byId: (id: number) => [...studentsKeys.all, 'byId', id] as const,
}

export function getStudentByIdQueryOptions(id: number) {
  return queryOptions({
    queryKey: studentsKeys.byId(id),
    queryFn: () => studentsApi.getStudentById(id),
    retry: false,
  })
}

export function getStudentSchoolsQueryOptions() {
  return queryOptions({
    queryKey: [...studentsKeys.all, 'schools'] as const,
    queryFn: studentsApi.getStudentSchools,
    retry: false,
  })
}

export function getStudentStudyProgramsQueryOptions() {
  return queryOptions({
    queryKey: [...studentsKeys.all, 'studyPrograms'] as const,
    queryFn: studentsApi.getStudentStudyPrograms,
    retry: false,
  })
}

export function getStudentClassesQueryOptions() {
  return queryOptions({
    queryKey: [...studentsKeys.all, 'classes'] as const,
    queryFn: studentsApi.getStudentClasses,
    retry: false,
  })
}

// patchCurrentTutorStudentsList updates the signed-in tutor's cached student
// list, if it's cached, since the backend always enrolls a created/updated
// student with the signed-in tutor.
function patchCurrentTutorStudentsList(
  queryClient: ReturnType<typeof useQueryClient>,
  updater: (students: Student[]) => Student[],
) {
  const tutor = queryClient.getQueryData(
    getMyTutorProfileQueryOptions().queryKey,
  )
  if (!tutor) return
  queryClient.setQueryData(
    getStudentsByTutorQueryOptions(tutor.id).queryKey,
    (oldData) => (oldData ? updater(oldData) : oldData),
  )
}

export function useCreateStudentMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: studentsApi.createStudent,
    onSuccess: (data) => {
      // set the createStudent data in the cache for the specific student by id
      queryClient.setQueryData(
        getStudentByIdQueryOptions(data.id).queryKey,
        data,
      )
      // update the signed-in tutor's student list to include the new student
      patchCurrentTutorStudentsList(queryClient, (students) => [
        ...students,
        data,
      ])
      invalidateSuggestionQueries(queryClient)
    },
  })
}

function invalidateSuggestionQueries(
  queryClient: ReturnType<typeof useQueryClient>,
) {
  queryClient.invalidateQueries({
    queryKey: getStudentSchoolsQueryOptions().queryKey,
  })
  queryClient.invalidateQueries({
    queryKey: getStudentStudyProgramsQueryOptions().queryKey,
  })
  queryClient.invalidateQueries({
    queryKey: getStudentClassesQueryOptions().queryKey,
  })
}

export function useDeleteStudentMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: studentsApi.deleteStudent,
    onSuccess: (_data, id) => {
      // drop the cached single-student entry
      queryClient.removeQueries({
        queryKey: getStudentByIdQueryOptions(id).queryKey,
      })
      // remove the deleted student from the signed-in tutor's student list
      patchCurrentTutorStudentsList(queryClient, (students) =>
        students.filter((student) => student.id !== id),
      )
    },
  })
}

export function useUpdateStudentMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: studentsApi.updateStudent,
    onSuccess: (data) => {
      // set the updateStudent data in the cache for the specific student by id
      queryClient.setQueryData(
        getStudentByIdQueryOptions(data.id).queryKey,
        data,
      )
      // update the signed-in tutor's student list with the updated student
      patchCurrentTutorStudentsList(queryClient, (students) =>
        students.map((student) => (student.id === data.id ? data : student)),
      )
      invalidateSuggestionQueries(queryClient)
    },
  })
}
