import {
  Autocomplete,
  Button,
  MaskInput,
  Modal,
  Stack,
  TextInput,
} from '@mantine/core'
import type { CreateStudentRequest, UpdateStudentRequest } from '../types'
import { useForm } from '@tanstack/react-form'
import { useQuery } from '@tanstack/react-query'
import * as v from 'valibot'
import { phoneMask } from '#/lib/phone'
import {
  getStudentClassesQueryOptions,
  getStudentSchoolsQueryOptions,
  getStudentStudyProgramsQueryOptions,
  useCreateStudentMutation,
  useUpdateStudentMutation,
} from '../queries'

type StudentUpsertData = CreateStudentRequest | UpdateStudentRequest

const emptyStudentData: StudentUpsertData = {
  displayName: '',
  email: '',
  phone: '',
  birthDate: '',
  class: '',
  school: '',
  studyProgram: '',
}

const studentSchema = v.object({
  displayName: v.pipe(v.string(), v.maxLength(256)),
  email: v.pipe(v.string(), v.maxLength(256), v.email()),
  phone: v.pipe(v.string(), v.maxLength(16), v.minLength(11)),
  birthDate: v.string(),
  class: v.pipe(v.string(), v.maxLength(256)),
  school: v.pipe(v.string(), v.maxLength(256)),
  studyProgram: v.pipe(v.string(), v.maxLength(256)),
})

export interface StudentDialogProps {
  opened: boolean
  close: () => void
  data?: StudentUpsertData
}

export function StudentDialog({ opened, close, data }: StudentDialogProps) {
  if (!data) {
    data = emptyStudentData
  }

  const create = useCreateStudentMutation()
  const update = useUpdateStudentMutation()

  const { data: schools = [] } = useQuery(getStudentSchoolsQueryOptions())
  const { data: studyPrograms = [] } = useQuery(
    getStudentStudyProgramsQueryOptions(),
  )
  const { data: classes = [] } = useQuery(getStudentClassesQueryOptions())

  const onSubmit = (values: StudentUpsertData) => {
    if ('id' in data && data.id) {
      update.mutate({ id: data.id, ...values }, { onSuccess: close })
    } else {
      create.mutate(values, { onSuccess: close })
    }
  }

  const { Field, handleSubmit, Subscribe } = useForm({
    defaultValues: data,
    validators: {
      onDynamic: studentSchema,
    },
    onSubmit: ({ value }) => onSubmit(value),
  })

  return (
    <Modal opened={opened} onClose={close} title="Student Dialog">
      <form
        onSubmit={(e) => {
          e.preventDefault()
          e.stopPropagation()
          handleSubmit()
        }}
      >
        <Stack gap="sm" mb="md">
          <Field
            name="displayName"
            children={(field) => (
              <TextInput
                label="Display Name"
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.currentTarget.value)}
                error={field.state.meta.errors.join(', ')}
              />
            )}
          />
          <Field
            name="email"
            children={(field) => (
              <TextInput
                label="Email"
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(e) => field.handleChange(e.currentTarget.value)}
                error={field.state.meta.errors.join(', ')}
              />
            )}
          />
          <Field
            name="phone"
            children={(field) => (
              <MaskInput
                label="Phone"
                mask={phoneMask('')}
                modify={(raw) => ({ mask: phoneMask(raw) })}
                slotChar={null}
                placeholder="+"
                defaultValue={field.state.value}
                onBlur={field.handleBlur}
                onChangeRaw={(raw) => field.handleChange('+' + raw)}
                error={field.state.meta.errors.join(', ')}
              />
            )}
          />
          <Field
            name="school"
            children={(field) => (
              <Autocomplete
                label="School"
                data={schools}
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(value) => field.handleChange(value)}
                error={field.state.meta.errors.join(', ')}
              />
            )}
          />
          <Field
            name="class"
            children={(field) => (
              <Autocomplete
                label="Class"
                data={classes}
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(value) => field.handleChange(value)}
                error={field.state.meta.errors.join(', ')}
              />
            )}
          />
          <Field
            name="studyProgram"
            children={(field) => (
              <Autocomplete
                label="Study Program"
                data={studyPrograms}
                value={field.state.value}
                onBlur={field.handleBlur}
                onChange={(value) => field.handleChange(value)}
                error={field.state.meta.errors.join(', ')}
              />
            )}
          />
          <Subscribe
            selector={(state) => [state.canSubmit, state.isSubmitting]}
            children={([canSubmit, isSubmitting]) => (
              <Button
                type="submit"
                disabled={!canSubmit}
                loading={isSubmitting}
              >
                Submit
              </Button>
            )}
          />
        </Stack>
      </form>
    </Modal>
  )
}
