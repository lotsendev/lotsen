import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { patchDeployment, type Deployment, type SecurityConfig } from '../lib/api'
import { hasSecurityChanges, isValidCIDROrIP, joinRules, splitRules, toSecurityConfig } from './securityConfig'

type ListKey = 'ip_denylist' | 'ip_allowlist'

const invalidEntryError = 'IP filters must be valid CIDR ranges or IP addresses.'

export function useDeploymentSecurityForm(deployment: Deployment) {
  const queryClient = useQueryClient()
  const [config, setConfig] = useState<SecurityConfig>(() => toSecurityConfig(deployment.security))
  const [customRulesText, setCustomRulesText] = useState(() => joinRules(deployment.security?.custom_rules ?? []))
  const [denyInput, setDenyInput] = useState('')
  const [allowInput, setAllowInput] = useState('')
  const [inputError, setInputError] = useState<string>()
  const [formError, setFormError] = useState<string>()
  const [isDirty, setIsDirty] = useState(false)

  useEffect(() => {
    setConfig(toSecurityConfig(deployment.security))
    setCustomRulesText(joinRules(deployment.security?.custom_rules ?? []))
    setDenyInput('')
    setAllowInput('')
    setInputError(undefined)
    setFormError(undefined)
    setIsDirty(false)
  }, [deployment.id, deployment.security])

  const mutation = useMutation({
    mutationFn: (security: SecurityConfig) => patchDeployment(deployment.id, { security }),
    onSuccess: updated => {
      queryClient.setQueryData<Deployment[]>(['deployments'], prev =>
        prev?.map(item => (item.id === deployment.id ? updated : item))
      )
      setFormError(undefined)
      setIsDirty(false)
    },
    onError: (error: Error) => {
      setFormError(error.message)
    },
  })

  const hasChanges = useMemo(
    () => hasSecurityChanges(config, deployment.security),
    [config, deployment.security]
  )

  function setWAFEnabled(enabled: boolean) {
    setConfig(prev => ({ ...prev, waf_enabled: enabled }))
    setIsDirty(true)
  }

  function setCustomRules(value: string) {
    setCustomRulesText(value)
    setIsDirty(true)
  }

  function setWAFMode(mode: SecurityConfig['waf_mode']) {
    setConfig(prev => ({ ...prev, waf_mode: mode }))
    setIsDirty(true)
  }

  function addEntry(key: ListKey, value: string) {
    const next = value.trim()
    if (!next) {
      return
    }

    if (!isValidCIDROrIP(next)) {
      setInputError(invalidEntryError)
      return
    }

    setInputError(undefined)
    setIsDirty(true)
    setConfig(prev => {
      if (prev[key].includes(next)) {
        return prev
      }

      return {
        ...prev,
        [key]: [...prev[key], next],
      }
    })

    if (key === 'ip_denylist') {
      setDenyInput('')
      return
    }
    setAllowInput('')
  }

  function removeEntry(key: ListKey, value: string) {
    setIsDirty(true)
    setConfig(prev => ({
      ...prev,
      [key]: prev[key].filter(item => item !== value),
    }))
  }

  function submit() {
    const security = {
      ...config,
      custom_rules: splitRules(customRulesText),
    }
    setConfig(security)
    setFormError(undefined)
    mutation.mutate(security)
  }

  return {
    config,
    customRulesText,
    denyInput,
    allowInput,
    inputError,
    formError,
    hasChanges,
    isDirty,
    isSaving: mutation.isPending,
    setDenyInput,
    setAllowInput,
    setWAFEnabled,
    setWAFMode,
    setCustomRules,
    addDenyEntry: (value: string) => addEntry('ip_denylist', value),
    addAllowEntry: (value: string) => addEntry('ip_allowlist', value),
    removeDenyEntry: (value: string) => removeEntry('ip_denylist', value),
    removeAllowEntry: (value: string) => removeEntry('ip_allowlist', value),
    submit,
  }
}
