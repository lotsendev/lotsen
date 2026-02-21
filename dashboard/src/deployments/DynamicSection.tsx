import { Plus, Trash2 } from 'lucide-react'
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
      <div className="flex items-center justify-between mb-2">
        <span className="text-xs font-medium text-gray-600">{title}</span>
        <button
          type="button"
          onClick={onAdd}
          className="flex items-center gap-1 text-xs text-gray-500 hover:text-gray-800"
          aria-label={addLabel}
        >
          <Plus size={13} /> {addLabel}
        </button>
      </div>
      {rows.length > 0 && (
        <div className="space-y-2">
          {rows.map(row => (
            <div key={row.id}>
              <div className="flex gap-2 items-center">
                {renderRow(row)}
                <button
                  type="button"
                  onClick={() => onRemove(row.id)}
                  aria-label={removeLabel}
                  className="p-1.5 rounded text-gray-400 hover:text-red-600 hover:bg-red-50 shrink-0"
                >
                  <Trash2 size={14} />
                </button>
              </div>
              {errorFor?.(row) && (
                <p className="text-xs text-red-600 mt-0.5">{errorFor(row)}</p>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
