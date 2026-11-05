import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { AppHeader } from './app-header'

describe('AppHeader', () => {
  it('shows both views as tabs, with the current one marked', () => {
    render(
      <AppHeader
        view="inspector"
        title={<h1>Demo Inspector</h1>}
        subtitle="ACME Commerce Lenders"
      />,
    )
    const nav = screen.getByRole('navigation', { name: 'Views' })
    expect(nav).toHaveTextContent('Demo')
    expect(
      screen.getByRole('link', { name: 'Demo Inspector' }),
    ).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: 'Demo' })).not.toHaveAttribute(
      'aria-current',
    )
    expect(
      screen.getByRole('link', { name: 'Demo Inspector' }),
    ).toHaveAttribute('href', '/inspector')
    expect(screen.getByRole('link', { name: 'Demo' })).toHaveAttribute(
      'href',
      '/',
    )
  })

  it('offers the inspector in a new tab from the demo only', () => {
    const { rerender } = render(
      <AppHeader view="demo" title={<h1>Demo</h1>} subtitle="x" />,
    )
    const newTab = screen.getByRole('link', {
      name: 'Open Demo Inspector in a new tab',
    })
    expect(newTab).toHaveAttribute('target', '_blank')
    rerender(
      <AppHeader
        view="inspector"
        title={<h1>Demo Inspector</h1>}
        subtitle="x"
      />,
    )
    expect(
      screen.queryByRole('link', { name: 'Open Demo Inspector in a new tab' }),
    ).not.toBeInTheDocument()
  })
})
