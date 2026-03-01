import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import type { ComponentProps } from 'react'
import { cn } from '../../lib/utils'

const badgeVariants = cva(
  'inline-flex items-center justify-center rounded-md border px-2 py-0.5 text-xs font-medium w-fit whitespace-nowrap shrink-0 gap-1 [&>svg]:size-3 pointer-events-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive transition-[color,box-shadow]',
  {
    variants: {
      variant: {
        default: 'border-transparent bg-primary text-primary-foreground [a&]:hover:bg-primary/90',
        secondary: 'border-transparent bg-secondary text-secondary-foreground [a&]:hover:bg-secondary/90',
        success: 'border-transparent bg-[#2a7a64]/15 text-[#1f5f4f] dark:bg-[#2a7a64]/25 dark:text-[#93d0bc]',
        warning: 'border-transparent bg-primary/12 text-primary',
        info: 'border-transparent bg-[#1a96e0]/15 text-[#126b9e] dark:bg-[#1a96e0]/25 dark:text-[#89c9ed]',
        destructive: 'border-transparent bg-destructive/15 text-destructive',
        outline: 'text-foreground [a&]:hover:bg-accent [a&]:hover:text-accent-foreground',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
)

type BadgeProps = ComponentProps<'span'> & VariantProps<typeof badgeVariants>

type BadgeComponentProps = BadgeProps & {
  asChild?: boolean
}

export function Badge({ className, variant, asChild = false, ...props }: BadgeComponentProps) {
  const Comp = asChild ? Slot : 'span'
  return <Comp data-slot="badge" className={cn(badgeVariants({ variant }), className)} {...props} />
}
