import { Check, ChevronDown } from 'lucide-react'
import { useState } from 'react'

/** The demos in this repo. There is one, so the menu has one entry. */
const demos = [{ id: 'acme-commerce-lenders', name: 'ACME Commerce Lenders' }]

/** The page title, which is also the menu for choosing another demo. */
export function DemoSwitcher({
  current,
  title,
}: {
  current: string
  title: string
}) {
  const [open, setOpen] = useState(false)
  return (
    <div className="relative">
      <h1 className="font-heading text-xl font-bold leading-tight">
        <button
          type="button"
          aria-haspopup="menu"
          aria-expanded={open}
          onClick={() => setOpen(!open)}
          className="flex items-center gap-1.5 rounded-md hover:text-primary focus-visible:text-primary"
        >
          {title}
          <ChevronDown className="size-5" />
        </button>
      </h1>
      {open && (
        <div
          role="menu"
          className="absolute left-0 top-full z-10 mt-2 w-72 rounded-lg border bg-popover p-1 text-popover-foreground shadow-lg"
        >
          {demos.map((d) => (
            <button
              key={d.id}
              type="button"
              role="menuitemradio"
              aria-checked={d.id === current}
              onClick={() => setOpen(false)}
              className="flex w-full items-center justify-between gap-2 rounded-md px-3 py-2 text-left text-sm hover:bg-muted/50"
            >
              Demo: {d.name}
              {d.id === current && <Check className="size-4" />}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
