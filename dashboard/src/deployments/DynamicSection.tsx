import { Plus, Trash2 } from 'lucide-react'
import { Button } from '../components/ui/button'
import type { Row } from './useDynamicRows'

interface Props<T extends Row> {
  title: string
  addLabel: string
  removeLabel: string
  rows: T[]
  onAdd: () => void
  onRemove: (id: number) => void
  renderRow: (row: T) => React.ReactNode
  errorFor?: (row: T) => string | undefined
}

export function DynamicSection<T extends Row>({
  title, addLabel, removeLabel, rows, onAdd, onRemove, renderRow, errorFor,
}: Props<T>) {
  return (
    <div>
      <div className="mb-2 flex items-center justify-between">
        <span className="text-sm font-medium text-foreground">{title}</span>
        <Button
          type="button"
          onClick={onAdd}
          variant="ghost"
          size="sm"
          className="h-7 px-2 text-xs text-muted-foreground hover:text-foreground"
          aria-label={addLabel}
        >
          <Plus size={13} /> {addLabel}
        </Button>
      </div>
      {rows.length > 0 && (
        <div className="space-y-2">
          {rows.map(row => (
            <div key={row.id}>
              <div className="flex items-center gap-2">
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
    </div>
  )
}
