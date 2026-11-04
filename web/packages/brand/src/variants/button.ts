import { cva, type VariantProps } from 'class-variance-authority'

/**
 * LocalStack-branded variants for the shadcn `<Button>` in each demo UI.
 *
 * Mint is the action color. The other variants stay quiet so the one action a
 * presenter should take next is the one that stands out.
 */
export const buttonVariants = cva(
  "group/button inline-flex shrink-0 items-center justify-center gap-2 rounded-lg border border-transparent bg-clip-padding font-medium whitespace-nowrap transition-all outline-none select-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 active:translate-y-px disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground hover:bg-primary/90',
        secondary:
          'bg-secondary text-secondary-foreground hover:bg-secondary/80',
        outline:
          'border-border bg-transparent text-foreground hover:border-primary hover:text-primary',
        ghost: 'text-foreground hover:bg-muted',
        destructive:
          'bg-destructive/15 text-destructive hover:bg-destructive/25',
      },
      size: {
        default: 'h-9 px-4 text-sm',
        sm: 'h-8 px-3 text-sm',
        // Big enough to hit and to read from across a booth.
        lg: 'h-12 px-6 text-base',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)

export type ButtonVariantProps = VariantProps<typeof buttonVariants>
