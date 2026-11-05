import { cva, type VariantProps } from 'class-variance-authority'

/** Small labels: a vendor's source, a state, a count. */
export const badgeVariants = cva(
  'inline-flex w-fit shrink-0 items-center gap-1 rounded-full border border-transparent px-2.5 py-0.5 text-xs font-medium whitespace-nowrap [&_svg]:size-3',
  {
    variants: {
      variant: {
        default: 'bg-secondary text-secondary-foreground',
        outline: 'border-border text-muted-foreground',
        // Good: healthy, done, approved.
        success: 'bg-primary/15 text-primary',
        // Informational: which system served a call.
        info: 'bg-accent/25 text-foreground',
        // Not wrong, but not the usual case: a dry run, production mode.
        warning: 'bg-warning/15 text-warning',
        // Wrong: failed, declined.
        destructive: 'bg-destructive/15 text-destructive',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
)

export type BadgeVariantProps = VariantProps<typeof badgeVariants>
