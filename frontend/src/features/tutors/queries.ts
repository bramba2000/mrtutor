import {
  queryOptions,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query'
import * as tutorsApi from './api'

export const tutorsKeys = {
  all: ['tutors'] as const,
  list: () => [...tutorsKeys.all, 'list'] as const,
  byId: (id: number) => [...tutorsKeys.all, 'byId', id] as const,
  me: () => [...tutorsKeys.all, 'me'] as const,
}

export function getTutorsQueryOptions() {
  return queryOptions({
    queryKey: tutorsKeys.list(),
    queryFn: tutorsApi.getTutors,
    retry: false,
  })
}

export function getTutorByIdQueryOptions(id: number) {
  return queryOptions({
    queryKey: tutorsKeys.byId(id),
    queryFn: () => tutorsApi.getTutorById(id),
    retry: false,
  })
}

export function getMyTutorProfileQueryOptions() {
  return queryOptions({
    queryKey: tutorsKeys.me(),
    queryFn: tutorsApi.getMyTutorProfile,
    retry: false,
  })
}

export function useCreateTutorMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: tutorsApi.createTutor,
    onSuccess: (data) => {
      // set the createTutor data in the cache for the specific tutor by id
      queryClient.setQueryData(getTutorByIdQueryOptions(data.id).queryKey, data)
      // the caller is always the owner of the tutor they just created
      queryClient.setQueryData(getMyTutorProfileQueryOptions().queryKey, data)
      // update the tutors list query to include the new tutor
      queryClient.setQueryData(getTutorsQueryOptions().queryKey, (oldData) => {
        if (oldData) {
          return [...oldData, data]
        }
        return [data]
      })
    },
  })
}

export function useDeleteTutorMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: tutorsApi.deleteTutor,
    onSuccess: (_data, id) => {
      // drop the cached single-tutor entry
      queryClient.removeQueries({
        queryKey: getTutorByIdQueryOptions(id).queryKey,
      })
      // remove the deleted tutor from the tutors list query
      queryClient.setQueryData(getTutorsQueryOptions().queryKey, (oldData) =>
        oldData?.filter((tutor) => tutor.id !== id),
      )
    },
  })
}

export function useUpdateTutorMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: tutorsApi.updateTutor,
    onSuccess: (data) => {
      // set the updateTutor data in the cache for the specific tutor by id
      queryClient.setQueryData(getTutorByIdQueryOptions(data.id).queryKey, data)
      // update the tutors list query to include the updated tutor
      queryClient.setQueryData(getTutorsQueryOptions().queryKey, (oldData) => {
        if (!oldData) return [data]
        return oldData.map((tutor) => (tutor.id === data.id ? data : tutor))
      })
    },
  })
}
