import { RotateCcw } from 'lucide-react'
import { AppHeader } from '@/components/app-header'
import { Button } from '@/components/ui/button'

export function Header({
  onReset,
  busy,
}: {
  onReset: () => void
  busy: boolean
}) {
  return (
    <AppHeader view="demo">
      <Button variant="outline" onClick={onReset} disabled={busy}>
        <RotateCcw />
        Reset
      </Button>
    </AppHeader>
  )
}
