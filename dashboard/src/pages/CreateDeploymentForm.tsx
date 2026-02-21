import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Trash2 } from 'lucide-react'
import { createDeployment } from '../lib/api'

type EnvRow = { id: number; key: string; value: string }
type PairRow = { id: number; left: string; right: string }

let nextId = 0
function newId() { return nextId++ }
function newEnvRow(): EnvRow { return { id: newId(), key: '', value: '' } }
function newPairRow(): PairRow { return { id: newId(), left: '', right: '' } }

type FormErrors = {
  name?: string
  image?: string
  envs: Record<number, string>
  ports: Record<number, string>
  volumes: Record<number, string>
  form?: string
}

const EMPTY_ERRORS: FormErrors = { envs: {}, ports: {}, volumes: {} }

const inputCls = 'h-9 rounded-md border border-gray-300 px-3 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900 w-full'
const inputErrCls = 'h-9 rounded-md border border-red-400 px-3 text-sm focus:outline-none focus:ring-2 focus:ring-red-500 w-full'
const removeBtnCls = 'p-1.5 rounded text-gray-400 hover:text-red-600 hover:bg-red-50 shrink-0'
const addBtnCls = 'flex items-center gap-1 text-xs text-gray-500 hover:text-gray-800'

export default function CreateDeploymentForm() {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [image, setImage] = useState('')
  const [domain, setDomain] = useState('')
  const [envRows, setEnvRows] = useState<EnvRow[]>([])
  const [portRows, setPortRows] = useState<PairRow[]>([])
  const [volumeRows, setVolumeRows] = useState<PairRow[]>([])
  const [errors, setErrors] = useState<FormErrors>(EMPTY_ERRORS)

  const mutation = useMutation({
    mutationFn: createDeployment,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['deployments'] })
      setName('')
      setImage('')
      setDomain('')
      setEnvRows([])
      setPortRows([])
      setVolumeRows([])
      setErrors(EMPTY_ERRORS)
    },
    onError: (err: Error) => setErrors(prev => ({ ...prev, form: err.message })),
  })

  function validate(): boolean {
    const errs: FormErrors = { envs: {}, ports: {}, volumes: {} }
    if (!name.trim()) errs.name = 'Name is required'
    if (!image.trim()) errs.image = 'Image is required'
    for (const row of envRows) {
      if (!row.key.trim()) errs.envs[row.id] = 'Key is required'
    }
    for (const row of portRows) {
      if (!row.left.trim() || !row.right.trim())
        errs.ports[row.id] = 'Both host and container ports are required'
    }
    for (const row of volumeRows) {
      if (!row.left.trim() || !row.right.trim())
        errs.volumes[row.id] = 'Both host and container paths are required'
    }
    setErrors(errs)
    return (
      !errs.name &&
      !errs.image &&
      Object.keys(errs.envs).length === 0 &&
      Object.keys(errs.ports).length === 0 &&
      Object.keys(errs.volumes).length === 0
    )
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!validate()) return
    const envs: Record<string, string> = {}
    for (const row of envRows) envs[row.key.trim()] = row.value
    mutation.mutate({
      name: name.trim(),
      image: image.trim(),
      envs,
      ports: portRows.map(r => `${r.left.trim()}:${r.right.trim()}`),
      volumes: volumeRows.map(r => `${r.left.trim()}:${r.right.trim()}`),
      domain: domain.trim(),
    })
  }

  return (
    <section className="mb-8 p-6 border border-gray-200 rounded-lg bg-white shadow-sm">
      <h2 className="text-sm font-medium text-gray-700 mb-4">New deployment</h2>
      <form onSubmit={handleSubmit} noValidate className="space-y-5">

        {/* Basic info */}
        <div className="grid grid-cols-2 gap-3">
          <div className="flex flex-col gap-1">
            <label htmlFor="dep-name" className="text-xs text-gray-500">Name *</label>
            <input
              id="dep-name"
              type="text"
              placeholder="my-app"
              value={name}
              onChange={e => setName(e.target.value)}
              className={errors.name ? inputErrCls : inputCls}
            />
            {errors.name && <p className="text-xs text-red-600">{errors.name}</p>}
          </div>
          <div className="flex flex-col gap-1">
            <label htmlFor="dep-image" className="text-xs text-gray-500">Image *</label>
            <input
              id="dep-image"
              type="text"
              placeholder="nginx:latest"
              value={image}
              onChange={e => setImage(e.target.value)}
              className={errors.image ? inputErrCls : inputCls}
            />
            {errors.image && <p className="text-xs text-red-600">{errors.image}</p>}
          </div>
        </div>

        {/* Domain */}
        <div className="flex flex-col gap-1">
          <label htmlFor="dep-domain" className="text-xs text-gray-500">Domain (optional)</label>
          <input
            id="dep-domain"
            type="text"
            placeholder="app.example.com"
            value={domain}
            onChange={e => setDomain(e.target.value)}
            className={inputCls}
          />
        </div>

        {/* Environment Variables */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-medium text-gray-600">Environment variables</span>
            <button
              type="button"
              onClick={() => setEnvRows(r => [...r, newEnvRow()])}
              className={addBtnCls}
              aria-label="Add env var"
            >
              <Plus size={13} /> Add env var
            </button>
          </div>
          {envRows.length > 0 && (
            <div className="space-y-2">
              {envRows.map(row => (
                <div key={row.id}>
                  <div className="flex gap-2 items-center">
                    <input
                      type="text"
                      placeholder="KEY"
                      value={row.key}
                      onChange={e => setEnvRows(rows => rows.map(r => r.id === row.id ? { ...r, key: e.target.value } : r))}
                      className={`${errors.envs[row.id] ? inputErrCls : inputCls} font-mono`}
                    />
                    <input
                      type="text"
                      placeholder="value"
                      value={row.value}
                      onChange={e => setEnvRows(rows => rows.map(r => r.id === row.id ? { ...r, value: e.target.value } : r))}
                      className={inputCls}
                    />
                    <button
                      type="button"
                      onClick={() => setEnvRows(rows => rows.filter(r => r.id !== row.id))}
                      aria-label="Remove env var"
                      className={removeBtnCls}
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                  {errors.envs[row.id] && (
                    <p className="text-xs text-red-600 mt-0.5">{errors.envs[row.id]}</p>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Port Mappings */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-medium text-gray-600">Port mappings</span>
            <button
              type="button"
              onClick={() => setPortRows(r => [...r, newPairRow()])}
              className={addBtnCls}
              aria-label="Add port mapping"
            >
              <Plus size={13} /> Add port
            </button>
          </div>
          {portRows.length > 0 && (
            <div className="space-y-2">
              {portRows.map(row => (
                <div key={row.id}>
                  <div className="flex gap-2 items-center">
                    <input
                      type="text"
                      placeholder="Host port"
                      value={row.left}
                      onChange={e => setPortRows(rows => rows.map(r => r.id === row.id ? { ...r, left: e.target.value } : r))}
                      className={errors.ports[row.id] ? inputErrCls : inputCls}
                    />
                    <span className="text-gray-400 shrink-0 text-sm">:</span>
                    <input
                      type="text"
                      placeholder="Container port"
                      value={row.right}
                      onChange={e => setPortRows(rows => rows.map(r => r.id === row.id ? { ...r, right: e.target.value } : r))}
                      className={errors.ports[row.id] ? inputErrCls : inputCls}
                    />
                    <button
                      type="button"
                      onClick={() => setPortRows(rows => rows.filter(r => r.id !== row.id))}
                      aria-label="Remove port mapping"
                      className={removeBtnCls}
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                  {errors.ports[row.id] && (
                    <p className="text-xs text-red-600 mt-0.5">{errors.ports[row.id]}</p>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Volume Mounts */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-medium text-gray-600">Volume mounts</span>
            <button
              type="button"
              onClick={() => setVolumeRows(r => [...r, newPairRow()])}
              className={addBtnCls}
              aria-label="Add volume mount"
            >
              <Plus size={13} /> Add volume
            </button>
          </div>
          {volumeRows.length > 0 && (
            <div className="space-y-2">
              {volumeRows.map(row => (
                <div key={row.id}>
                  <div className="flex gap-2 items-center">
                    <input
                      type="text"
                      placeholder="/host/path"
                      value={row.left}
                      onChange={e => setVolumeRows(rows => rows.map(r => r.id === row.id ? { ...r, left: e.target.value } : r))}
                      className={`${errors.volumes[row.id] ? inputErrCls : inputCls} font-mono`}
                    />
                    <span className="text-gray-400 shrink-0 text-sm">:</span>
                    <input
                      type="text"
                      placeholder="/container/path"
                      value={row.right}
                      onChange={e => setVolumeRows(rows => rows.map(r => r.id === row.id ? { ...r, right: e.target.value } : r))}
                      className={`${errors.volumes[row.id] ? inputErrCls : inputCls} font-mono`}
                    />
                    <button
                      type="button"
                      onClick={() => setVolumeRows(rows => rows.filter(r => r.id !== row.id))}
                      aria-label="Remove volume mount"
                      className={removeBtnCls}
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                  {errors.volumes[row.id] && (
                    <p className="text-xs text-red-600 mt-0.5">{errors.volumes[row.id]}</p>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Submit */}
        {errors.form && <p className="text-xs text-red-600">{errors.form}</p>}
        <button
          type="submit"
          disabled={mutation.isPending}
          className="h-9 px-4 rounded-md bg-gray-900 text-white text-sm font-medium hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {mutation.isPending ? 'Creating…' : 'Create'}
        </button>
      </form>
    </section>
  )
}
