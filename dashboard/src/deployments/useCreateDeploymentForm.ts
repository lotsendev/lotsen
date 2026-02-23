import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createDeployment } from '../lib/api'
import { useDynamicRows } from './useDynamicRows'

export type EnvRow = { id: number; key: string; value: string }
export type PairRow = { id: number; left: string; right: string }
export type PortRow = { id: number; port: string }

export type FormErrors = {
  name?: string
  image?: string
  envs: Record<number, string>
  ports: Record<number, string>
  volumes: Record<number, string>
  form?: string
}

const EMPTY_ERRORS: FormErrors = { envs: {}, ports: {}, volumes: {} }

type UseCreateDeploymentFormOptions = {
  onSuccess?: () => void
}

export function useCreateDeploymentForm(options: UseCreateDeploymentFormOptions = {}) {
  const queryClient = useQueryClient()

  const [name, setName] = useState('')
  const [image, setImage] = useState('')
  const [domain, setDomain] = useState('')
  const [errors, setErrors] = useState<FormErrors>(EMPTY_ERRORS)

  const envRows = useDynamicRows<EnvRow>(id => ({ id, key: '', value: '' }))
  const portRows = useDynamicRows<PortRow>(id => ({ id, port: '' }))
  const volumeRows = useDynamicRows<PairRow>(id => ({ id, left: '', right: '' }))

  const mutation = useMutation({
    mutationFn: createDeployment,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['deployments'] })
      setName('')
      setImage('')
      setDomain('')
      envRows.reset()
      portRows.reset()
      volumeRows.reset()
      setErrors(EMPTY_ERRORS)
      options.onSuccess?.()
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
      if (!row.port.trim())
        errs.ports[row.id] = 'Container port is required'
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
      ports: portRows.rows.map(r => r.port.trim()),
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
