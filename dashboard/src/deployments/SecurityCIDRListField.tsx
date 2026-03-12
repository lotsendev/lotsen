import { X } from 'lucide-react'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Label } from '../components/ui/label'

type Props = {
  id: string
  label: string
  value: string
  entries: string[]
  emptyLabel: string
  badgeVariant: 'warning' | 'info'
  onChange: (value: string) => void
  onAdd: (value: string) => void
  onRemove: (value: string) => void
}

export function SecurityCIDRListField({
  id,
  label,
  value,
  entries,
  emptyLabel,
  badgeVariant,
  onChange,
  onAdd,
  onRemove,
}: Props) {
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      <div className="flex gap-2">
        <Input
          id={id}
          placeholder="Enter CIDR or IP and press Enter"
          value={value}
          onChange={event => onChange(event.target.value)}
          onKeyDown={event => {
            if (event.key !== 'Enter') {
              return
            }
            event.preventDefault()
            onAdd(value)
          }}
        />
        <Button type="button" variant="outline" onClick={() => onAdd(value)}>
          Add
        </Button>
      </div>
      {entries.length > 0 ? (
        <div className="flex flex-wrap gap-2">
          {entries.map(item => (
            <Badge key={`${id}-${item}`} variant={badgeVariant} className="pointer-events-auto gap-1 pr-1">
              <span className="font-mono">{item}</span>
              <button
                type="button"
                className="rounded p-0.5 hover:bg-accent"
                onClick={() => onRemove(item)}
                aria-label={`Remove ${item}`}
              >
                <X className="h-3 w-3" />
              </button>
            </Badge>
          ))}
        </div>
      ) : (
        <p className="text-xs text-muted-foreground">{emptyLabel}</p>
      )}
    </div>
  )
}
