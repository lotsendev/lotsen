import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateDeployment, type Deployment } from '../lib/api'
import { useDynamicRows } from './useDynamicRows'
import type { EnvRow, FormErrors, PairRow } from './useCreateDeploymentForm'

const EMPTY_ERRORS: FormErrors = { envs: {}, ports: {}, volumes: {} }

function toEnvRows(envs: Record<string, string>): EnvRow[] {
  return Object.entries(envs).map(([key, value], i) => ({ id: i, key, value }))
}

function toPairRows(items: string[]): PairRow[] {
  return items.map((item, i) => {
    const sep = item.indexOf(':')
    return { id: i, left: sep >= 0 ? item.slice(0, sep) : item, right: sep >= 0 ? item.slice(sep + 1) : '' }
  })
}

export function useEditDeploymentForm(deployment: Deployment, onClose: () => void) {
  const queryClient = useQueryClient()

  const [name, setName] = useState(deployment.name)
  const [image, setImage] = useState(deployment.image)
  const [domain, setDomain] = useState(deployment.domain)
  const [errors, setErrors] = useState<FormErrors>(EMPTY_ERRORS)

  const envRows = useDynamicRows<EnvRow>(id => ({ id, key: '', value: '' }), toEnvRows(deployment.envs))
  const portRows = useDynamicRows<PairRow>(id => ({ id, left: '', right: '' }), toPairRows(deployment.ports))
  const volumeRows = useDynamicRows<PairRow>(id => ({ id, left: '', right: '' }), toPairRows(deployment.volumes))

  const mutation = useMutation({
    mutationFn: (data: Parameters<typeof updateDeployment>[1]) => updateDeployment(deployment.id, data),
    onSuccess: () => {
      queryClient.setQueryData<Deployment[]>(['deployments'], prev =>
        prev?.map(d => d.id === deployment.id ? { ...d, status: 'deploying' } : d)
      )
      onClose()
    },
    onError: (err: Error) => setErrors(prev => ({ ...prev, form: err.message })),
  })

  function validate(): boolean {
    const errs: FormErrors = { envs: {}, ports: {}, volumes: {} }
    if (!name.trim()) errs.name = 'Name is required'
    if (!image.trim()) errs.image = 'Image is required'
    for (const row of envRows.rows) {
      if (!row.key.trim()) errs.envs[row.id] = 'Key is required'
    }
    for (const row of portRows.rows) {
      if (!row.left.trim() || !row.right.trim())
        errs.ports[row.id] = 'Both host and container ports are required'
    }
    for (const row of volumeRows.rows) {
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
    for (const row of envRows.rows) envs[row.key.trim()] = row.value
    mutation.mutate({
      name: name.trim(),
      image: image.trim(),
      envs,
      ports: portRows.rows.map(r => `${r.left.trim()}:${r.right.trim()}`),
      volumes: volumeRows.rows.map(r => `${r.left.trim()}:${r.right.trim()}`),
      domain: domain.trim(),
    })
  }

  return {
    name, setName,
    image, setImage,
    domain, setDomain,
    envRows,
    portRows,
    volumeRows,
    errors,
    handleSubmit,
    isPending: mutation.isPending,
  }
}
