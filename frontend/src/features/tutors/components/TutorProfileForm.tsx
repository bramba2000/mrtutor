import {
  Button,
  Card,
  Group,
  MaskInput,
  SimpleGrid,
  Stack,
  Text,
  Textarea,
  TextInput,
} from '@mantine/core'
import { useForm } from '@tanstack/react-form'
import { useNavigate } from '@tanstack/react-router'
import * as v from 'valibot'
import { phoneMask } from '#/lib/phone'
import type { CreateTutorRequest, Tutor } from '../types'
import { useCreateTutorMutation, useUpdateTutorMutation } from '../queries'

export interface TutorProfileFormProps {
  data?: Tutor
}

const emptyTutorData: CreateTutorRequest = {
  displayName: '',
  email: '',
  phone: '',
  aboutMe: '',
}

const tutorSchema = v.object({
  displayName: v.pipe(v.string(), v.nonEmpty(), v.maxLength(256)),
  email: v.string(),
  phone: v.pipe(v.string(), v.maxLength(16), v.minLength(11)),
  aboutMe: v.pipe(v.string(), v.maxLength(2000)),
})

export function TutorProfileForm({ data }: TutorProfileFormProps) {
  const navigate = useNavigate()
  const create = useCreateTutorMutation()
  const update = useUpdateTutorMutation()
  const mutation = data ? update : create

  const onSubmit = (values: CreateTutorRequest) => {
    if (data?.id) {
      update.mutate({ id: data.id, ...values })
    } else {
      create.mutate(values, {
        onSuccess: () => navigate({ to: '/', replace: true }),
      })
    }
  }

  const { Field, handleSubmit, Subscribe } = useForm({
    defaultValues: data ?? emptyTutorData,
    validators: {
      onDynamic: tutorSchema,
    },
    onSubmit: ({ value }) => onSubmit(value),
  })

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        e.stopPropagation()
        handleSubmit()
      }}
    >
      <Stack gap="lg">
        <SimpleGrid cols={{ base: 1, md: 2 }} spacing="lg">
          <Card withBorder radius={0} p="lg">
            <Text fw="bold" mb="md">
              Personal information
            </Text>
            <Stack gap="sm">
              <Field
                name="displayName"
                children={(field) => (
                  <TextInput
                    label="Display name"
                    radius={0}
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
                    radius={0}
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
            </Stack>
          </Card>

          <Card withBorder radius={0} p="lg">
            <Text fw="bold" mb="md">
              About me
            </Text>
            <Field
              name="aboutMe"
              children={(field) => (
                <Textarea
                  label="Bio"
                  radius={0}
                  minRows={8}
                  autosize
                  value={field.state.value}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.currentTarget.value)}
                  error={field.state.meta.errors.join(', ')}
                />
              )}
            />
          </Card>
        </SimpleGrid>

        {mutation.error && (
          <Text c="red" size="sm">
            {mutation.error.message}
          </Text>
        )}

        <Group justify="flex-end">
          <Subscribe
            selector={(state) => [state.canSubmit, state.isSubmitting]}
            children={([canSubmit, isSubmitting]) => (
              <Button
                type="submit"
                radius={0}
                disabled={!canSubmit}
                loading={isSubmitting || mutation.isPending}
              >
                {data ? 'Save changes' : 'Create profile'}
              </Button>
            )}
          />
        </Group>
      </Stack>
    </form>
  )
}
