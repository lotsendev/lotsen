import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Trash2 } from 'lucide-react'
import { getDeployments, createDeployment, deleteDeployment, type DeploymentStatus } from '../lib/api'

const STATUS_STYLES: Record<DeploymentStatus, string> = {
  idle: 'bg-gray-100 text-gray-600',
  deploying: 'bg-blue-100 text-blue-700',
  healthy: 'bg-green-100 text-green-700',
  failed: 'bg-red-100 text-red-700',
}

export default function DeploymentList() {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [image, setImage] = useState('')
  const [formError, setFormError] = useState('')

  const { data: deployments, isLoading, isError } = useQuery({
    queryKey: ['deployments'],
    queryFn: getDeployments,
  })

  const createMutation = useMutation({
    mutationFn: createDeployment,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['deployments'] })
      setName('')
      setImage('')
      setFormError('')
    },
    onError: (err: Error) => setFormError(err.message),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteDeployment,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['deployments'] }),
  })

  function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim() || !image.trim()) {
      setFormError('Name and image are required.')
      return
    }
    createMutation.mutate({ name: name.trim(), image: image.trim() })
  }

  return (
    <main className="max-w-4xl mx-auto px-6 py-10">
      <h1 className="text-2xl font-semibold text-gray-900 mb-8">Deployments</h1>

      {/* Create form */}
      <section className="mb-8 p-6 border border-gray-200 rounded-lg bg-white shadow-sm">
        <h2 className="text-sm font-medium text-gray-700 mb-4">New deployment</h2>
        <form onSubmit={handleCreate} className="flex gap-3 items-end">
          <div className="flex flex-col gap-1 flex-1">
            <label htmlFor="name" className="text-xs text-gray-500">Name</label>
            <input
              id="name"
              type="text"
              placeholder="my-app"
              value={name}
              onChange={e => setName(e.target.value)}
              className="h-9 rounded-md border border-gray-300 px-3 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
            />
          </div>
          <div className="flex flex-col gap-1 flex-1">
            <label htmlFor="image" className="text-xs text-gray-500">Image</label>
            <input
              id="image"
              type="text"
              placeholder="nginx:latest"
              value={image}
              onChange={e => setImage(e.target.value)}
              className="h-9 rounded-md border border-gray-300 px-3 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900"
            />
          </div>
          <button
            type="submit"
            disabled={createMutation.isPending}
            className="h-9 px-4 rounded-md bg-gray-900 text-white text-sm font-medium hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {createMutation.isPending ? 'Creating…' : 'Create'}
          </button>
        </form>
        {formError && <p className="mt-2 text-xs text-red-600">{formError}</p>}
      </section>

      {/* Deployment table */}
      {isLoading && <p className="text-sm text-gray-500">Loading deployments…</p>}
      {isError && <p className="text-sm text-red-600">Failed to load deployments.</p>}
      {deployments && deployments.length === 0 && (
        <p className="text-sm text-gray-500">No deployments yet.</p>
      )}
      {deployments && deployments.length > 0 && (
        <div className="border border-gray-200 rounded-lg overflow-hidden shadow-sm">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b border-gray-200">
              <tr>
                <th className="text-left px-4 py-3 font-medium text-gray-600">Name</th>
                <th className="text-left px-4 py-3 font-medium text-gray-600">Image</th>
                <th className="text-left px-4 py-3 font-medium text-gray-600">Status</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {deployments.map(d => (
                <tr key={d.id} className="bg-white hover:bg-gray-50">
                  <td className="px-4 py-3 font-medium text-gray-900">{d.name}</td>
                  <td className="px-4 py-3 text-gray-600 font-mono text-xs">{d.image}</td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_STYLES[d.status]}`}>
                      {d.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => deleteMutation.mutate(d.id)}
                      disabled={deleteMutation.isPending}
                      aria-label={`Delete ${d.name}`}
                      className="p-1.5 rounded text-gray-400 hover:text-red-600 hover:bg-red-50 disabled:opacity-40"
                    >
                      <Trash2 size={15} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </main>
  )
}
