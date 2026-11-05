import logo from '@localstack-app-emulators/brand/logo/localstack-logo-horizontal-light.svg'
import { useRouter } from '@tanstack/react-router'
import { ExternalLink } from 'lucide-react'
import type { MouseEvent, ReactNode } from 'react'
import { DemoSwitcher } from '@/components/demo-switcher'
import { ScenarioInfo } from '@/components/scenario-info'
import { cn } from '@/lib/utils'

type View = 'demo' | 'inspector'

const views: { id: View; to: '/' | '/inspector'; label: string }[] = [
  { id: 'demo', to: '/', label: 'Demo' },
  { id: 'inspector', to: '/inspector', label: 'Demo Inspector' },
]

/**
 * The header both views share. The title is the demo's, because the tabs, not
 * the title, say which view you are on. The logo and title are on the left, the
 * two views are tabs, and the view's own controls are on the right.
 */
export function AppHeader({
  view,
  children,
}: {
  view: View
  children?: ReactNode
}) {
  // Without a router, such as in a test, a tab is an ordinary link.
  const router = useRouter({ warn: false })
  const go = (to: string) => (e: MouseEvent) => {
    if (!router || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0)
      return
    e.preventDefault()
    void router.navigate({ to })
  }

  return (
    <header className="flex items-center justify-between gap-4">
      <div className="flex items-center gap-4">
        <img src={logo} alt="LocalStack" className="h-8" />
        <div className="h-8 w-px bg-border" />
        <div>
          <div className="flex items-center gap-2">
            <DemoSwitcher
              current="acme-commerce-lenders"
              title="Demo: ACME Commerce Lenders"
            />
            <ScenarioInfo />
          </div>
          <p className="text-sm text-muted-foreground">
            LocalStack for SaaS Applications
          </p>
        </div>
      </div>
      <nav aria-label="Views" className="flex items-center gap-1">
        {views.map((v) => (
          <a
            key={v.id}
            href={v.to}
            aria-current={v.id === view ? 'page' : undefined}
            onClick={go(v.to)}
            className={cn(
              'border-b-2 px-3 py-2 text-sm font-medium transition-colors',
              v.id === view
                ? 'border-primary text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            )}
          >
            {v.label}
          </a>
        ))}
        {view === 'demo' && (
          <a
            href="/inspector"
            target="_blank"
            rel="noreferrer"
            aria-label="Open Demo Inspector in a new tab"
            title="Open Demo Inspector in a new tab, to put beside the demo"
            className="rounded-md p-2 text-muted-foreground hover:text-foreground"
          >
            <ExternalLink className="size-4" />
          </a>
        )}
      </nav>
      <div className="flex min-w-24 items-center justify-end gap-2">
        {children}
      </div>
    </header>
  )
}
