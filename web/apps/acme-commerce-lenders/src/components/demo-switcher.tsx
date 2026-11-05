import { Check, ChevronDown } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@/components/ui/button'

/** The demos in this repo. There is one, so the menu has one entry. */
const demos = [{ id: 'acme-commerce-lenders', name: 'ACME Commerce Lenders' }]

export function DemoSwitcher({ current }: { current: string }) {
  const [open, setOpen] = useState(false)
  return (
    <div className="relative">
      <Button
        variant="outline"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        Demos
        <ChevronDown />
      </Button>
      {open && (
        <div
          role="menu"
          className="absolute right-0 top-full z-10 mt-2 w-64 rounded-lg border bg-popover p-1 text-popover-foreground shadow-lg"
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
              {d.name}
              {d.id === current && <Check className="size-4" />}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
