import { createFileRoute } from '@tanstack/react-router'
import { InspectorPage } from '@/inspector/inspector-page'

export const Route = createFileRoute('/inspector')({
  component: InspectorPage,
})
