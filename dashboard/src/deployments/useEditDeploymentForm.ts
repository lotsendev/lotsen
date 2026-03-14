import { useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateDeployment, type BasicAuthConfig, type Deployment, type FileMount, type UpdateDeploymentInput, type VolumeMount } from '../lib/api'
import { hashPasswordIfNeeded } from '../lib/password'
import { parsePortSpec } from './portSpec'
import { useDynamicRows } from './useDynamicRows'
import type { BasicAuthUserRow, EnvRow, FileMountRow, FormErrors, PortRow, VolumeMountRow } from './useCreateDeploymentForm'

const EMPTY_ERRORS: FormErrors = { envs: {}, ports: {}, volumes: {}, files: {}, basicAuth: {} }

function toEnvRows(envs: Record<string, string>): EnvRow[] {
  return Object.entries(envs).map(([key, value], i) => ({ id: i, key, value }))
}

function toVolumeMountRows(deployment: Deployment): VolumeMountRow[] {
  const mounts: VolumeMount[] = deployment.volume_mounts ?? deployment.volumes.map(volume => {
    const sep = volume.indexOf(':')
    return {
      mode: 'bind',
      source: sep >= 0 ? volume.slice(0, sep) : volume,
      target: sep >= 0 ? volume.slice(sep + 1) : '',
    }
  })

  return mounts.map((mount, i) => ({
    id: i,
    mode: mount.mode,
    source: mount.source,
    target: mount.target,
  }))
}

function toPortRows(items: string[]): PortRow[] {
  return items.map((item, i) => {
    const sep = item.indexOf(':')
    return { id: i, port: sep >= 0 ? item.slice(sep + 1) : item }
  })
}

function toFileMountRows(deployment: Deployment): FileMountRow[] {
  const mounts: FileMount[] = deployment.file_mounts ?? []
  return mounts.map((mount, i) => ({
    id: i,
    source: mount.source,
    target: mount.target,
    content: mount.content,
    readOnly: mount.read_only ?? false,
  }))
}

function findProxyRowId(rows: PortRow[], proxyPort?: number): number | null {
  if (!proxyPort) {
    return null
  }
  const match = rows.find(row => {
    const parsed = parsePortSpec(row.port)
    return parsed?.protocol === 'tcp' && parsed.containerPort === proxyPort
  })
  return match?.id ?? null
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
  const volumeRows = useDynamicRows<VolumeMountRow>(id => ({ id, mode: 'managed', source: '', target: '' }), toVolumeMountRows(deployment))
  const fileRows = useDynamicRows<FileMountRow>(id => ({ id, source: '', target: '', content: '', readOnly: true }), toFileMountRows(deployment))
  const basicAuthRows = useDynamicRows<BasicAuthUserRow>(id => ({ id, username: '', password: '' }), toBasicAuthRows(deployment))
  const [selectedProxyRowId, setSelectedProxyRowId] = useState<number | null>(() => findProxyRowId(toPortRows(deployment.ports), deployment.proxy_port))

  const normalizedInput = useMemo<UpdateDeploymentInput>(() => {
    const envs: Record<string, string> = {}
    for (const row of envRows.rows) {
      envs[row.key.trim()] = row.value
    }

    let proxyPort: number | undefined
    if (domain.trim() && selectedProxyRowId != null) {
      const selected = portRows.rows.find(row => row.id === selectedProxyRowId)
      const parsed = selected ? parsePortSpec(selected.port) : null
      if (parsed?.protocol === 'tcp') {
        proxyPort = parsed.containerPort
      }
    }

    return {
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
      file_mounts: fileRows.rows.map(r => ({
        source: r.source.trim(),
        target: r.target.trim(),
        content: r.content,
        read_only: r.readOnly,
      })),
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
  }, [name, image, domain, isPublic, envRows.rows, portRows.rows, selectedProxyRowId, volumeRows.rows, fileRows.rows, basicAuthEnabled, basicAuthRows.rows, deployment.security])

  const isDirty = useMemo(() => {
    const initial: UpdateDeploymentInput = {
      name: deployment.name,
      image: deployment.image,
      envs: deployment.envs,
      ports: deployment.ports.map(port => {
        const separator = port.indexOf(':')
        return separator >= 0 ? port.slice(separator + 1) : port
      }),
      proxy_port: deployment.proxy_port,
      volume_mounts: deployment.volume_mounts ?? deployment.volumes.map(volume => {
        const separator = volume.indexOf(':')
        return {
          mode: 'bind',
          source: separator >= 0 ? volume.slice(0, separator) : volume,
          target: separator >= 0 ? volume.slice(separator + 1) : '',
        }
      }),
      file_mounts: deployment.file_mounts,
      domain: deployment.domain,
      public: deployment.public,
      basic_auth: deployment.basic_auth,
      security: deployment.security,
    }

    if (initial.name !== normalizedInput.name ||
      initial.image !== normalizedInput.image ||
      initial.domain !== normalizedInput.domain ||
      initial.public !== normalizedInput.public ||
      initial.proxy_port !== normalizedInput.proxy_port) {
      return true
    }

    if (initial.ports.length !== normalizedInput.ports.length || initial.volume_mounts?.length !== normalizedInput.volume_mounts?.length || initial.file_mounts?.length !== normalizedInput.file_mounts?.length) {
      return true
    }

    for (let i = 0; i < initial.ports.length; i += 1) {
      if (initial.ports[i] !== normalizedInput.ports[i]) {
        return true
      }
    }

    for (let i = 0; i < (initial.volume_mounts?.length ?? 0); i += 1) {
      const current = normalizedInput.volume_mounts?.[i]
      const existing = initial.volume_mounts?.[i]
      if (!current || !existing || current.mode !== existing.mode || current.source !== existing.source || current.target !== existing.target) {
        return true
      }
    }

    for (let i = 0; i < (initial.file_mounts?.length ?? 0); i += 1) {
      const current = normalizedInput.file_mounts?.[i]
      const existing = initial.file_mounts?.[i]
      if (!current || !existing ||
        current.source !== existing.source ||
        current.target !== existing.target ||
        current.content !== existing.content ||
        (current.read_only ?? false) !== (existing.read_only ?? false)) {
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
    const errs: FormErrors = { envs: {}, ports: {}, volumes: {}, files: {}, basicAuth: {} }
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
    for (const row of fileRows.rows) {
      if (!row.source.trim() || !row.target.trim()) {
        errs.files[row.id] = 'File name and container path are required'
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
      Object.keys(errs.files).length === 0 &&
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
    selectedProxyRowId, setSelectedProxyRowId,
    envRows,
    portRows,
    volumeRows,
    fileRows,
    basicAuthRows,
    errors,
    handleSubmit,
    isDirty,
    isPending: mutation.isPending,
  }
}
