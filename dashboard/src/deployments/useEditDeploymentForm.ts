import { useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateDeployment, type BasicAuthConfig, type Deployment, type UpdateDeploymentInput } from '../lib/api'
import { hashPasswordIfNeeded } from '../lib/password'
import { useDynamicRows } from './useDynamicRows'
import type { BasicAuthUserRow, EnvRow, FormErrors, PairRow, PortRow } from './useCreateDeploymentForm'

const EMPTY_ERRORS: FormErrors = { envs: {}, ports: {}, volumes: {}, basicAuth: {} }

function toEnvRows(envs: Record<string, string>): EnvRow[] {
  return Object.entries(envs).map(([key, value], i) => ({ id: i, key, value }))
}

function toPairRows(items: string[]): PairRow[] {
  return items.map((item, i) => {
    const sep = item.indexOf(':')
    return { id: i, left: sep >= 0 ? item.slice(0, sep) : item, right: sep >= 0 ? item.slice(sep + 1) : '' }
  })
}

function toPortRows(items: string[]): PortRow[] {
  return items.map((item, i) => {
    const sep = item.indexOf(':')
    return { id: i, port: sep >= 0 ? item.slice(sep + 1) : item }
  })
}

function toBasicAuthRows(deployment: Deployment): BasicAuthUserRow[] {
  return (deployment.basic_auth?.users ?? []).map((user, i) => ({ id: i, username: user.username, password: user.password }))
}

function equalBasicAuth(a?: BasicAuthConfig, b?: BasicAuthConfig): boolean {
  if (!a || !b) {
    return a === b
  }
  if (a.users.length !== b.users.length) {
    return false
  }

  return a.users.every((user, index) => {
    const other = b.users[index]
    return Boolean(other) && user.username === other.username && user.password === other.password
  })
}

export function useEditDeploymentForm(deployment: Deployment, onClose: () => void) {
  const queryClient = useQueryClient()

  const [name, setName] = useState(deployment.name)
  const [image, setImage] = useState(deployment.image)
  const [domain, setDomain] = useState(deployment.domain)
  const [isPublic, setIsPublic] = useState(deployment.public)
  const [basicAuthEnabled, setBasicAuthEnabled] = useState(Boolean(deployment.basic_auth && deployment.basic_auth.users.length > 0))
  const [errors, setErrors] = useState<FormErrors>(EMPTY_ERRORS)

  const envRows = useDynamicRows<EnvRow>(id => ({ id, key: '', value: '' }), toEnvRows(deployment.envs))
  const portRows = useDynamicRows<PortRow>(id => ({ id, port: '' }), toPortRows(deployment.ports))
  const volumeRows = useDynamicRows<PairRow>(id => ({ id, left: '', right: '' }), toPairRows(deployment.volumes))
  const basicAuthRows = useDynamicRows<BasicAuthUserRow>(id => ({ id, username: '', password: '' }), toBasicAuthRows(deployment))

  const normalizedInput = useMemo<UpdateDeploymentInput>(() => {
    const envs: Record<string, string> = {}
    for (const row of envRows.rows) {
      envs[row.key.trim()] = row.value
    }

    return {
      name: name.trim(),
      image: image.trim(),
      envs,
      ports: portRows.rows.map(r => r.port.trim()),
      volumes: volumeRows.rows.map(r => `${r.left.trim()}:${r.right.trim()}`),
      domain: domain.trim(),
      public: isPublic,
      basic_auth: basicAuthEnabled
        ? {
            users: basicAuthRows.rows.map(row => ({
              username: row.username.trim(),
              password: row.password.trim(),
            })),
          }
        : undefined,
      security: deployment.security,
    }
  }, [name, image, domain, isPublic, envRows.rows, portRows.rows, volumeRows.rows, basicAuthEnabled, basicAuthRows.rows, deployment.security])

  const isDirty = useMemo(() => {
    const initial: UpdateDeploymentInput = {
      name: deployment.name,
      image: deployment.image,
      envs: deployment.envs,
      ports: deployment.ports.map(port => {
        const separator = port.indexOf(':')
        return separator >= 0 ? port.slice(separator + 1) : port
      }),
      volumes: deployment.volumes,
      domain: deployment.domain,
      public: deployment.public,
      basic_auth: deployment.basic_auth,
      security: deployment.security,
    }

    if (initial.name !== normalizedInput.name ||
      initial.image !== normalizedInput.image ||
      initial.domain !== normalizedInput.domain ||
      initial.public !== normalizedInput.public) {
      return true
    }

    if (initial.ports.length !== normalizedInput.ports.length || initial.volumes.length !== normalizedInput.volumes.length) {
      return true
    }

    for (let i = 0; i < initial.ports.length; i += 1) {
      if (initial.ports[i] !== normalizedInput.ports[i]) {
        return true
      }
    }

    for (let i = 0; i < initial.volumes.length; i += 1) {
      if (initial.volumes[i] !== normalizedInput.volumes[i]) {
        return true
      }
    }

    const initialEnvEntries = Object.entries(initial.envs)
    const currentEnvEntries = Object.entries(normalizedInput.envs)
    if (initialEnvEntries.length !== currentEnvEntries.length) {
      return true
    }

    for (const [key, value] of initialEnvEntries) {
      if (normalizedInput.envs[key] !== value) {
        return true
      }
    }

    return !equalBasicAuth(initial.basic_auth, normalizedInput.basic_auth)
  }, [deployment, normalizedInput])

  const mutation = useMutation({
    mutationFn: (data: Parameters<typeof updateDeployment>[1]) => updateDeployment(deployment.id, data),
    onSuccess: (updated) => {
      queryClient.setQueryData<Deployment[]>(['deployments'], prev =>
        prev?.map(d => (d.id === deployment.id ? updated : d))
      )
      onClose()
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
    for (const row of volumeRows.rows) {
      if (!row.left.trim() || !row.right.trim()) errs.volumes[row.id] = 'Both host and container paths are required'
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

    mutation.mutate({
      ...normalizedInput,
      basic_auth: basicAuth,
    })
  }

  return {
    name, setName,
    image, setImage,
    domain, setDomain,
    isPublic, setIsPublic,
    basicAuthEnabled, setBasicAuthEnabled,
    envRows,
    portRows,
    volumeRows,
    basicAuthRows,
    errors,
    handleSubmit,
    isDirty,
    isPending: mutation.isPending,
  }
}
