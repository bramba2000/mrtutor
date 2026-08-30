import { useMatches } from '@mantine/core'

// Keeps the appbar burger and action icon glyphs the same visual size,
// scaled to the AppShell header height configured in `_authenticated.tsx`
// (base/sm/lg).
//
// `control` is the ActionIcon hit-box size (it already adds hover padding
// around its glyph), `icon` is the visible glyph size — used both for the
// icon inside ActionIcon and directly for Burger, which has no padding of
// its own around its rendered X.
export function useAppBarControlSize() {
  const icon = useMatches({ base: 16, sm: 18, lg: 20 })
  const control = useMatches({ base: 24, sm: 26, lg: 28 })
  return { control, icon }
}
