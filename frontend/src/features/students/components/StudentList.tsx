import { getStudentsByTutorQueryOptions } from '#/features/tutors/queries'
import { ActionIcon, EmptyState, Group, Table } from '@mantine/core'
import type { Student } from '../types'
import { useQuery } from '@tanstack/react-query'
import { MagnifyingGlassIcon } from '@phosphor-icons/react'
import { NotePencilIcon, TrashIcon } from '@phosphor-icons/react/dist/ssr'

export interface StudentListProps {
  tutorId: number
  onEdit?: (student: Student) => void
  onDelete?: (student: Student) => void
}

export function StudentList({ tutorId, onEdit, onDelete }: StudentListProps) {
  const { data: students, isPending } = useQuery(
    getStudentsByTutorQueryOptions(tutorId),
  )
  const colN = 3

  let tBody = (
    <Table.Tr>
      <Table.Td colSpan={colN}>
        <EmptyState
          icon={<MagnifyingGlassIcon />}
          title="No students found"
          description="Try to reset filters or create your first student"
          mb="sm"
        />
      </Table.Td>
    </Table.Tr>
  )
  if (students && students.length > 0) {
    tBody = (
      <>
        {students.map((student) => (
          <Table.Tr key={student.id}>
            <Table.Td style={{ whiteSpace: 'nowrap', width: '1%' }}>
              {student.id}
            </Table.Td>
            <Table.Td>{student.displayName}</Table.Td>
            <Table.Td style={{ whiteSpace: 'nowrap', width: '1%' }}>
              <Group gap="xs" wrap="nowrap" justify="flex-end">
                <ActionIcon
                  variant="transparent"
                  aria-label={`edit student ${student.id}`}
                  c="dimmed"
                  onClick={() => onEdit?.(student)}
                >
                  <NotePencilIcon size="20" />
                </ActionIcon>
                <ActionIcon
                  variant="transparent"
                  aria-label={`delete student ${student.id}`}
                  c="red"
                  onClick={() => onDelete?.(student)}
                >
                  <TrashIcon size="20" />
                </ActionIcon>
              </Group>
            </Table.Td>
          </Table.Tr>
        ))}
      </>
    )
  } else if (isPending) {
    tBody = (
      <Table.Tr>
        <Table.Td colSpan={colN}>Loading...</Table.Td>
      </Table.Tr>
    )
  }

  return (
    <Table withTableBorder w="100%">
      <Table.Thead>
        <Table.Tr>
          <Table.Th style={{ whiteSpace: 'nowrap', width: '1%' }}>ID</Table.Th>
          <Table.Th>Name</Table.Th>
          <Table.Th align="right" style={{ whiteSpace: 'nowrap', width: '1%' }}>
            Actions
          </Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>{tBody}</Table.Tbody>
    </Table>
  )
}
