import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { DemoSwitcher } from './demo-switcher'

describe('DemoSwitcher', () => {
  it('is the page title, and opens a menu with the current demo selected', async () => {
    render(
      <DemoSwitcher
        current="acme-commerce-lenders"
        title="Demo: ACME Commerce Lenders"
      />,
    )
    const title = screen.getByRole('heading', { level: 1 })
    expect(title).toHaveTextContent('Demo: ACME Commerce Lenders')
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()

    await userEvent.click(
      screen.getByRole('button', { name: 'Demo: ACME Commerce Lenders' }),
    )
    expect(screen.getByRole('menu')).toBeInTheDocument()
    expect(
      screen.getByRole('menuitemradio', {
        name: 'Demo: ACME Commerce Lenders',
      }),
    ).toBeChecked()
  })
})
