import { Pencil, Trash2 } from 'lucide-react'
import type { Deployment } from '../lib/api'
import { StatusBadge } from './StatusBadge'

type Props = {
  deployment: Deployment
  onDelete: (id: string) => void
  isDeleting: boolean
  onEdit: (deployment: Deployment) => void
}

export function DeploymentRow({ deployment: d, onDelete, isDeleting, onEdit }: Props) {
  return (
    <tr className="bg-white hover:bg-gray-50">
      <td className="px-4 py-3 font-medium text-gray-900">{d.name}</td>
      <td className="px-4 py-3 text-gray-600 font-mono text-xs">{d.image}</td>
      <td className="px-4 py-3">
        <StatusBadge status={d.status} error={d.error} />
      </td>
      <td className="px-4 py-3 text-right flex items-center justify-end gap-1">
        <button
          onClick={() => onEdit(d)}
          aria-label={`Edit ${d.name}`}
          className="p-1.5 rounded text-gray-400 hover:text-gray-700 hover:bg-gray-100"
        >
          <Pencil size={15} />
        </button>
        <button
          onClick={() => onDelete(d.id)}
          disabled={isDeleting}
          aria-label={`Delete ${d.name}`}
          className="p-1.5 rounded text-gray-400 hover:text-red-600 hover:bg-red-50 disabled:opacity-40"
        >
          <Trash2 size={15} />
        </button>
      </td>
    </tr>
  )
}
