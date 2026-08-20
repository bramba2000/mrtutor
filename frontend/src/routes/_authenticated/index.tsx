import { DeleteConfirmDialog } from '#/components/DeleteConfirmDialog'
import { StudentDialog } from '#/features/students/components/StudentDialog'
import { StudentList } from '#/features/students/components/StudentList'
import { useDeleteStudentMutation } from '#/features/students/queries'
import type { Student } from '#/features/students/types'
import { Box, Breadcrumbs, Button, Group, Text } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { PlusIcon } from '@phosphor-icons/react/dist/ssr'
import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'

export const Route = createFileRoute('/_authenticated/')({ component: Home })

function Home() {
  const [dialogOpened, { open: openDialog, close: closeDialog }] =
    useDisclosure(false)
  const [deleteOpened, { open: openDelete, close: closeDelete }] =
    useDisclosure(false)

  const [toEdit, setToEdit] = useState<Student>()
  const [toDelete, setToDelete] = useState<Student>()

  const deleteStudent = useDeleteStudentMutation()

  const onAddStudent = () => {
    setToEdit(undefined)
    openDialog()
  }

  const onEditStudent = (student: Student) => {
    setToEdit(student)
    openDialog()
  }

  const onDeleteStudent = (student: Student) => {
    setToDelete(student)
    openDelete()
  }

  const onConfirmDelete = () => {
    if (!toDelete) return
    deleteStudent.mutate(toDelete.id, { onSuccess: closeDelete })
  }

  return (
    <Box p="xl">
      <Group justify="space-between" mb="md">
        <Breadcrumbs separator=">">
          <Text size="lg" fw="bold">
            Students
          </Text>
        </Breadcrumbs>
        <Button onClick={onAddStudent} leftSection={<PlusIcon />}>
          Add Student
        </Button>
      </Group>
      <StudentList onEdit={onEditStudent} onDelete={onDeleteStudent} />
      <StudentDialog opened={dialogOpened} close={closeDialog} data={toEdit} />
      <DeleteConfirmDialog
        opened={deleteOpened}
        onClose={closeDelete}
        onConfirm={onConfirmDelete}
        title="Delete student"
        description={`Are you sure you want to delete ${toDelete?.displayName}? This action cannot be undone.`}
        loading={deleteStudent.isPending}
        error={deleteStudent.error?.message}
      />
    </Box>
  )
}
