import type { QueryClient } from '@tanstack/react-query'
import type { ErrorComponentProps } from '@tanstack/react-router'
import {
  createRootRouteWithContext,
  Link,
  Outlet,
} from '@tanstack/react-router'
import { AlertTriangle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { FlowProvider } from '@/demo/flow-context'

interface RouterContext {
  queryClient: QueryClient
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
  notFoundComponent: NotFound,
  errorComponent: ErrorBoundary,
})

function RootLayout() {
  return (
    <FlowProvider>
      <div className="min-h-screen">
        <Outlet />
      </div>
    </FlowProvider>
  )
}

function NotFound() {
  return (
    <div className="mx-auto flex max-w-md flex-col items-center gap-4 px-6 py-24 text-center">
      <h1 className="font-heading text-2xl font-bold">Page not found</h1>
      <Link to="/" className="text-primary underline underline-offset-4">
        Back to the demo
      </Link>
    </div>
  )
}

function ErrorBoundary({ error }: ErrorComponentProps) {
  return (
    <div
      role="alert"
      className="mx-auto flex max-w-md flex-col items-center gap-4 px-6 py-24 text-center"
    >
      <AlertTriangle className="size-8 text-destructive" />
      <h1 className="font-heading text-2xl font-bold">Something went wrong</h1>
      <p className="text-sm text-muted-foreground">{error.message}</p>
      <Button variant="outline" onClick={() => window.location.reload()}>
        Reload
      </Button>
    </div>
  )
}
