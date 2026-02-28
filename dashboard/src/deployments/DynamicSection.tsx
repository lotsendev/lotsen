import { Plus, Trash2 } from 'lucide-react'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import type { Row } from './useDynamicRows'

interface Props<T extends Row> {
  title: string
  description?: string
  addLabel: string
  removeLabel: string
  rows: T[]
  onAdd: () => void
  onRemove: (id: number) => void
  renderRow: (row: T) => React.ReactNode
  errorFor?: (row: T) => string | undefined
}

export function DynamicSection<T extends Row>({
  title, description, addLabel, removeLabel, rows, onAdd, onRemove, renderRow, errorFor,
}: Props<T>) {
  return (
    <section className="rounded-lg border border-border/60 bg-background/60 p-3 sm:p-4">
      <div className="mb-3 flex flex-wrap items-start justify-between gap-2">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <span className="text-sm font-semibold text-foreground">{title}</span>
            <Badge variant="outline" className="h-5 px-1.5 text-[10px]">
              {rows.length}
            </Badge>
          </div>
          {description && <p className="text-xs text-muted-foreground">{description}</p>}
        </div>
        <Button
          type="button"
          onClick={onAdd}
          variant="outline"
          size="sm"
          className="h-7 gap-1.5 px-2.5 text-xs"
          aria-label={addLabel}
        >
          <Plus size={13} /> {addLabel}
        </Button>
      </div>
      {rows.length > 0 && (
        <div className="space-y-2">
          {rows.map(row => (
            <div key={row.id}>
              <div className="flex items-center gap-2 rounded-md border border-border/40 bg-card/80 p-2">
                {renderRow(row)}
                <Button
                  type="button"
                  onClick={() => onRemove(row.id)}
                  aria-label={removeLabel}
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7 shrink-0 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                >
                  <Trash2 size={14} />
                </Button>
              </div>
              {errorFor?.(row) && (
                <p className="mt-0.5 text-xs text-destructive">{errorFor(row)}</p>
              )}
            </div>
          ))}
        </div>
      )}

      {rows.length === 0 && (
        <p className="text-xs text-muted-foreground">No entries yet.</p>
      )}
    </section>
  )
}
