import {
  type ButtonVariantProps,
  buttonVariants,
} from '@localstack-app-emulators/brand/variants/button'
import type * as React from 'react'

import { cn } from '@/lib/utils'

function Button({
  className,
  variant = 'default',
  size = 'default',
  type = 'button',
  ...props
}: React.ComponentProps<'button'> & ButtonVariantProps) {
  return (
    <button
      data-slot="button"
      type={type}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
