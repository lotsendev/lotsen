import type { Deployment, SecurityConfig } from '../lib/api'

function isValidIPv4(input: string) {
  if (!/^(\d{1,3}\.){3}\d{1,3}$/.test(input)) {
    return false
  }

  return input.split('.').every(part => Number(part) >= 0 && Number(part) <= 255)
}

function isValidIPv6(input: string) {
  if (!input.includes(':')) {
    return false
  }

  return /^[0-9a-fA-F:]+$/.test(input)
}

export function isValidCIDROrIP(input: string) {
  const value = input.trim()
  if (!value) {
    return false
  }

  if (!value.includes('/')) {
    return isValidIPv4(value) || isValidIPv6(value)
  }

  const [ip, prefix, ...rest] = value.split('/')
  if (rest.length > 0 || !prefix) {
    return false
  }

  const prefixNum = Number(prefix)
  if (!Number.isInteger(prefixNum) || prefixNum < 0) {
    return false
  }

  if (isValidIPv4(ip)) {
    return prefixNum <= 32
  }
  if (isValidIPv6(ip)) {
    return prefixNum <= 128
  }
  return false
}

export function toSecurityConfig(security: Deployment['security']): SecurityConfig {
  return {
    waf_enabled: security?.waf_enabled ?? true,
    waf_mode: security?.waf_mode ?? 'detection',
    ip_denylist: security?.ip_denylist ?? [],
    ip_allowlist: security?.ip_allowlist ?? [],
    custom_rules: security?.custom_rules ?? [],
  }
}

export function joinRules(rules: string[]) {
  return rules.join('\n')
}

export function splitRules(value: string) {
  return value
    .split('\n')
    .map(rule => rule.trim())
    .filter(Boolean)
}

export function hasSecurityChanges(config: SecurityConfig, existing: Deployment['security']) {
  if (!existing) {
    return (
      config.waf_enabled !== true ||
      config.waf_mode !== 'detection' ||
      config.ip_denylist.length > 0 ||
      config.ip_allowlist.length > 0 ||
      config.custom_rules.length > 0
    )
  }

  return JSON.stringify(existing) !== JSON.stringify(config)
}
