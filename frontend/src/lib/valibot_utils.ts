export function fieldError(errors: unknown[]): string | undefined {
  if (errors.length === 0) return undefined
  return errors
    .map((error) =>
      typeof error === 'string'
        ? error
        : (error as { message: string }).message,
    )
    .join(', ')
}
