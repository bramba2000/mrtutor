// Subscriber number: a leading group of 3 digits, then every remaining
// digit (up to the backend's 14-digit max) in one trailing group.
export const MAX_SUBSCRIBER_DIGITS = 14

export function phoneMask(raw: string): string {
  const ccLen = raw[0] === '7' ? 1 : 2
  const cc = '9'.repeat(ccLen)
  const rest = '9'.repeat(MAX_SUBSCRIBER_DIGITS - 3)
  return `+${cc} 999 ${rest}`
}
