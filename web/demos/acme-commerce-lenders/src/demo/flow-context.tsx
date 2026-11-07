import { createContext, type ReactNode, useContext } from 'react'
import { useDemoFlow } from './use-demo-flow'

const FlowContext = createContext<ReturnType<typeof useDemoFlow> | null>(null)

/**
 * Holds the demo's progress above the router, so switching to the inspector and
 * back does not lose where the presenter was.
 */
export function FlowProvider({ children }: { children: ReactNode }) {
  const flow = useDemoFlow()
  return <FlowContext.Provider value={flow}>{children}</FlowContext.Provider>
}

export function useFlow() {
  const flow = useContext(FlowContext)
  if (!flow) throw new Error('useFlow needs a FlowProvider above it')
  return flow
}
