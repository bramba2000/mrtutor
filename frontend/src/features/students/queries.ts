import {
  queryOptions,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query'
import * as studentsApi from './api'

export const studentsKeys = {
  all: ['students'] as const,
  list: () => [...studentsKeys.all, 'list'] as const,
  byId: (id: number) => [...studentsKeys.all, 'byId', id] as const,
}

export function getStudentsQueryOptions() {
  return queryOptions({
    queryKey: studentsKeys.list(),
    queryFn: studentsApi.getStudents,
    retry: false,
  })
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
      // update the students list query to include the new student
      queryClient.setQueryData(
        getStudentsQueryOptions().queryKey,
        (oldData) => {
          if (oldData) {
            return [...oldData, data]
          }
          return [data]
        },
      )
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
      // remove the deleted student from the students list query
      queryClient.setQueryData(getStudentsQueryOptions().queryKey, (oldData) =>
        oldData?.filter((student) => student.id !== id),
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
      // update the students list query to include the updated student
      queryClient.setQueryData(
        getStudentsQueryOptions().queryKey,
        (oldData) => {
          if (!oldData) return [data]
          return oldData.map((student) =>
            student.id === data.id ? data : student,
          )
        },
      )
      invalidateSuggestionQueries(queryClient)
    },
  })
}
