import { Button, Group, Modal, Stack, Text } from '@mantine/core'
import type { ReactNode } from 'react'

export interface DeleteConfirmDialogProps {
  opened: boolean
  onClose: () => void
  onConfirm: () => void
  title: string
  description: ReactNode
  loading?: boolean
  error?: string
}

export function DeleteConfirmDialog({
  opened,
  onClose,
  onConfirm,
  title,
  description,
  loading,
  error,
}: DeleteConfirmDialogProps) {
  return (
    <Modal opened={opened} onClose={onClose} title={title}>
      <Stack gap="sm">
        <Text size="sm">{description}</Text>
        {error && (
          <Text size="sm" c="red">
            {error}
          </Text>
        )}
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose} disabled={loading}>
            Cancel
          </Button>
          <Button color="red" onClick={onConfirm} loading={loading}>
            Delete
          </Button>
        </Group>
      </Stack>
    </Modal>
  )
}
