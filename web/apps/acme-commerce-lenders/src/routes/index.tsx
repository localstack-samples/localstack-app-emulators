import { createFileRoute } from '@tanstack/react-router'
import { DemoPage } from '@/demo/demo-page'

export const Route = createFileRoute('/')({
  component: DemoPage,
})
