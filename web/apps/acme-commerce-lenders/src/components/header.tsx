import logo from '@localstack-app-emulators/brand/logo/localstack-logo-horizontal-light.svg'
import { RotateCcw } from 'lucide-react'
import { Button } from '@/components/ui/button'

export function Header({
  onReset,
  busy,
}: {
  onReset: () => void
  busy: boolean
}) {
  return (
    <header className="flex items-center justify-between gap-4">
      <div className="flex items-center gap-4">
        <img src={logo} alt="LocalStack" className="h-8" />
        <div className="h-8 w-px bg-border" />
        <div>
          <h1 className="font-heading text-xl font-bold leading-tight">
            ACME Commerce Lenders
          </h1>
          <p className="text-sm text-muted-foreground">
            LocalStack for SaaS Apps
          </p>
        </div>
      </div>
      <Button variant="outline" onClick={onReset} disabled={busy}>
        <RotateCcw />
        Reset
      </Button>
    </header>
  )
}
