import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createDeployment } from '../lib/api'
import { hashPasswordIfNeeded } from '../lib/password'
import { useDynamicRows } from './useDynamicRows'

export type EnvRow = { id: number; key: string; value: string }
export type PairRow = { id: number; left: string; right: string }
export type PortRow = { id: number; port: string }
export type BasicAuthUserRow = { id: number; username: string; password: string }
export type RegistryAuthMode = 'username_password' | 'identity_token'

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
  const [registryAuthEnabled, setRegistryAuthEnabled] = useState(false)
  const [registryServerAddress, setRegistryServerAddress] = useState('')
  const [registryUsername, setRegistryUsername] = useState('')
  const [registryPassword, setRegistryPassword] = useState('')
  const [registryIdentityToken, setRegistryIdentityToken] = useState('')
  const [registryAuthMode, setRegistryAuthMode] = useState<RegistryAuthMode>('username_password')
  const [errors, setErrors] = useState<FormErrors>(EMPTY_ERRORS)

  const envRows = useDynamicRows<EnvRow>(id => ({ id, key: '', value: '' }))
  const portRows = useDynamicRows<PortRow>(id => ({ id, port: '' }))
  const volumeRows = useDynamicRows<PairRow>(id => ({ id, left: '', right: '' }))
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
      setRegistryAuthEnabled(false)
      setRegistryServerAddress('')
      setRegistryUsername('')
      setRegistryPassword('')
      setRegistryIdentityToken('')
      setRegistryAuthMode('username_password')
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
    for (const row of volumeRows.rows) {
      if (!row.left.trim() || !row.right.trim()) errs.volumes[row.id] = 'Both host and container paths are required'
    }
    if (basicAuthEnabled) {
      if (basicAuthRows.rows.length === 0) errs.form = 'Add at least one basic auth user'
      for (const row of basicAuthRows.rows) {
        if (!row.username.trim() || !row.password.trim()) errs.basicAuth[row.id] = 'Username and password are required'
      }
    }
    if (registryAuthEnabled) {
      if (!registryServerAddress.trim()) errs.form = 'Registry server is required when private registry is enabled'
      if (registryAuthMode === 'username_password' && (!registryUsername.trim() || !registryPassword.trim())) {
        errs.form = 'Registry username and password are required'
      }
      if (registryAuthMode === 'identity_token' && !registryIdentityToken.trim()) {
        errs.form = 'Registry identity token is required'
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

    mutation.mutate({
      name: name.trim(),
      image: image.trim(),
      envs,
      ports: portRows.rows.map(r => r.port.trim()),
      volumes: volumeRows.rows.map(r => `${r.left.trim()}:${r.right.trim()}`),
      domain: domain.trim(),
      public: isPublic,
      basic_auth: basicAuth,
      registry_auth: registryAuthEnabled
        ? {
            server_address: registryServerAddress.trim(),
            username: registryAuthMode === 'username_password' ? registryUsername.trim() : undefined,
            password: registryAuthMode === 'username_password' ? registryPassword.trim() : undefined,
            identity_token: registryAuthMode === 'identity_token' ? registryIdentityToken.trim() : undefined,
          }
        : undefined,
    })
  }

  return {
    name, setName,
    image, setImage,
    domain, setDomain,
    isPublic, setIsPublic,
    basicAuthEnabled, setBasicAuthEnabled,
    registryAuthEnabled, setRegistryAuthEnabled,
    registryServerAddress, setRegistryServerAddress,
    registryUsername, setRegistryUsername,
    registryPassword, setRegistryPassword,
    registryIdentityToken, setRegistryIdentityToken,
    registryAuthMode, setRegistryAuthMode,
    envRows,
    portRows,
    volumeRows,
    basicAuthRows,
    errors,
    handleSubmit,
    isPending: mutation.isPending,
  }
}
