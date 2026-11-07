import {
  type BadgeVariantProps,
  badgeVariants,
} from '@localstack-app-emulators/brand/variants/badge'
import type * as React from 'react'

import { cn } from '@/lib/utils'

function Badge({
  className,
  variant,
  ...props
}: React.ComponentProps<'span'> & BadgeVariantProps) {
  return (
    <span
      data-slot="badge"
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    />
  )
}

export { Badge, badgeVariants }
