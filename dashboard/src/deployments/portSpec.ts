export type ParsedPortSpec = {
  containerPort: number
  protocol: 'tcp' | 'udp'
}

export function parsePortSpec(raw: string): ParsedPortSpec | null {
  const trimmed = raw.trim()
  if (!trimmed) {
    return null
  }

  const [mainPart, protocolPart] = trimmed.split('/')
  const protocol = (protocolPart?.trim().toLowerCase() || 'tcp') as 'tcp' | 'udp'
  if (protocol !== 'tcp' && protocol !== 'udp') {
    return null
  }

  const parts = mainPart.split(':').map(part => part.trim()).filter(Boolean)
  if (parts.length < 1 || parts.length > 2) {
    return null
  }

  const containerRaw = parts[parts.length - 1]
  const containerPort = Number.parseInt(containerRaw, 10)
  if (!Number.isInteger(containerPort) || containerPort < 1 || containerPort > 65535) {
    return null
  }

  return { containerPort, protocol }
}
