import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createDeployment } from '../lib/api'
import { hashPasswordIfNeeded } from '../lib/password'
import { parsePortSpec } from './portSpec'
import { useDynamicRows } from './useDynamicRows'

export type EnvRow = { id: number; key: string; value: string }
export type VolumeMountRow = { id: number; mode: 'managed' | 'bind'; source: string; target: string }
export type PortRow = { id: number; port: string }
export type BasicAuthUserRow = { id: number; username: string; password: string }

export type FormErrors = {
  name?: string
  image?: string
  envs: Record<number, string>
  ports: Record<number, string>
  volumes: Record<number, string>
  basicAuth: Record<number, string>
  form?: string
}

const EMPTY_ERRORS: FormErrors = { envs: {}, ports: {}, volumes: {}, basicAuth: {} }

type UseCreateDeploymentFormOptions = {
  onSuccess?: () => void
}

export function useCreateDeploymentForm(options: UseCreateDeploymentFormOptions = {}) {
  const queryClient = useQueryClient()

  const [name, setName] = useState('')
  const [image, setImage] = useState('')
  const [domain, setDomain] = useState('')
  const [isPublic, setIsPublic] = useState(false)
  const [basicAuthEnabled, setBasicAuthEnabled] = useState(false)
  const [selectedProxyRowId, setSelectedProxyRowId] = useState<number | null>(null)
  const [errors, setErrors] = useState<FormErrors>(EMPTY_ERRORS)

  const envRows = useDynamicRows<EnvRow>(id => ({ id, key: '', value: '' }))
  const portRows = useDynamicRows<PortRow>(id => ({ id, port: '' }))
  const volumeRows = useDynamicRows<VolumeMountRow>(id => ({ id, mode: 'managed', source: '', target: '' }))
  const basicAuthRows = useDynamicRows<BasicAuthUserRow>(id => ({ id, username: '', password: '' }))

  const mutation = useMutation({
    mutationFn: createDeployment,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['deployments'] })
      setName('')
      setImage('')
      setDomain('')
      setIsPublic(false)
      setBasicAuthEnabled(false)
      setSelectedProxyRowId(null)
      envRows.reset()
      portRows.reset()
      volumeRows.reset()
      basicAuthRows.reset()
      setErrors(EMPTY_ERRORS)
      options.onSuccess?.()
    },
    onError: (err: Error) => setErrors(prev => ({ ...prev, form: err.message })),
  })

  function validate(): boolean {
    const errs: FormErrors = { envs: {}, ports: {}, volumes: {}, basicAuth: {} }
    if (!name.trim()) errs.name = 'Name is required'
    if (!image.trim()) errs.image = 'Image is required'
    for (const row of envRows.rows) {
      if (!row.key.trim()) errs.envs[row.id] = 'Key is required'
    }
    for (const row of portRows.rows) {
      if (!row.port.trim()) errs.ports[row.id] = 'Container port is required'
    }
    if (domain.trim()) {
      if (selectedProxyRowId == null) {
        errs.form = 'Select a proxy target port when domain is set'
      } else {
        const selected = portRows.rows.find(row => row.id === selectedProxyRowId)
        if (!selected) {
          errs.form = 'Select a valid proxy target port when domain is set'
        } else {
          const parsed = parsePortSpec(selected.port)
          if (!parsed) {
            errs.ports[selected.id] = 'Proxy target must be a valid port mapping'
          } else if (parsed.protocol !== 'tcp') {
            errs.ports[selected.id] = 'Proxy target must use TCP'
          }
        }
      }
    }
    for (const row of volumeRows.rows) {
      if (!row.source.trim() || !row.target.trim()) {
        errs.volumes[row.id] = row.mode === 'managed'
          ? 'Volume name and container path are required'
          : 'Host and container paths are required'
      }
    }
    if (basicAuthEnabled) {
      if (basicAuthRows.rows.length === 0) errs.form = 'Add at least one basic auth user'
      for (const row of basicAuthRows.rows) {
        if (!row.username.trim() || !row.password.trim()) errs.basicAuth[row.id] = 'Username and password are required'
      }
    }
    setErrors(errs)
    return (
      !errs.name &&
      !errs.image &&
      Object.keys(errs.envs).length === 0 &&
      Object.keys(errs.ports).length === 0 &&
      Object.keys(errs.volumes).length === 0 &&
      Object.keys(errs.basicAuth).length === 0 &&
      !errs.form
    )
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!validate()) return
    const envs: Record<string, string> = {}
    for (const row of envRows.rows) envs[row.key.trim()] = row.value

    const basicAuth = basicAuthEnabled
      ? {
          users: await Promise.all(
            basicAuthRows.rows.map(async row => ({
              username: row.username.trim(),
              password: await hashPasswordIfNeeded(row.password.trim()),
            }))
          ),
        }
      : undefined

    let proxyPort: number | undefined
    if (domain.trim() && selectedProxyRowId != null) {
      const selected = portRows.rows.find(row => row.id === selectedProxyRowId)
      const parsed = selected ? parsePortSpec(selected.port) : null
      if (parsed?.protocol === 'tcp') {
        proxyPort = parsed.containerPort
      }
    }

    mutation.mutate({
      name: name.trim(),
      image: image.trim(),
      envs,
      ports: portRows.rows.map(r => r.port.trim()),
      proxy_port: proxyPort,
      volume_mounts: volumeRows.rows.map(r => ({
        mode: r.mode,
        source: r.source.trim(),
        target: r.target.trim(),
      })),
      domain: domain.trim(),
      public: isPublic,
      basic_auth: basicAuth,
    })
  }

  return {
    name, setName,
    image, setImage,
    domain, setDomain,
    isPublic, setIsPublic,
    basicAuthEnabled, setBasicAuthEnabled,
    selectedProxyRowId, setSelectedProxyRowId,
    envRows,
    portRows,
    volumeRows,
    basicAuthRows,
    errors,
    handleSubmit,
    isPending: mutation.isPending,
  }
}
