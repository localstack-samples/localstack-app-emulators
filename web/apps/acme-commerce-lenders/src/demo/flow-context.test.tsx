import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { FlowProvider, useFlow } from './flow-context'

function Probe() {
  return <p>busy: {String(useFlow().busy)}</p>
}

describe('flow context', () => {
  it('gives the flow to what is inside the provider', () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <FlowProvider>
          <Probe />
        </FlowProvider>
      </QueryClientProvider>,
    )
    expect(screen.getByText('busy: false')).toBeInTheDocument()
  })

  it('fails loudly without a provider', () => {
    const quiet = console.error
    console.error = () => {}
    try {
      expect(() => render(<Probe />)).toThrow('useFlow needs a FlowProvider')
    } finally {
      console.error = quiet
    }
  })
})
