import { type FormEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Label } from '../components/ui/label'
import { getSecurityConfig, type Deployment } from '../lib/api'
import { SecurityCIDRListField } from './SecurityCIDRListField'
import { useDeploymentSecurityForm } from './useDeploymentSecurityForm'

type Props = {
  deployment: Deployment
}

export function DeploymentSecurityPanel({ deployment }: Props) {
  const securityQuery = useQuery({
    queryKey: ['security-config'],
    queryFn: getSecurityConfig,
  })
  const globalWAFEnabled = securityQuery.data?.wafEnabled ?? true

  const form = useDeploymentSecurityForm(deployment, globalWAFEnabled)

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    form.submit()
  }

  return (
    <section className="rounded-xl border border-border/60 bg-card p-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground/60">Security</p>
          <p className="mt-1 text-xs text-muted-foreground">Configure per-deployment WAF, IP filtering, and custom rules.</p>
        </div>
        {securityQuery.data?.wafMode ? (
          <Badge variant="info" className="capitalize">
            Global mode: {securityQuery.data.wafMode}
          </Badge>
        ) : null}
      </div>

      {!securityQuery.isLoading && securityQuery.data?.wafEnabled === false ? (
        <p className="mt-3 rounded-md border border-amber-300/50 bg-amber-50 px-3 py-2 text-xs text-amber-900">
          Global WAF is disabled. Enabling WAF here will not take effect until global WAF is enabled.
        </p>
      ) : null}

      <form onSubmit={handleSubmit} className="mt-4 space-y-4">
        <label className="flex items-center gap-2 text-sm text-foreground">
          <input
            type="checkbox"
            checked={form.config.waf_enabled}
            onChange={event => form.setWAFEnabled(event.target.checked)}
          />
          Enable WAF for this deployment
        </label>

        <SecurityCIDRListField
          id={`security-denylist-${deployment.id}`}
          label="IP denylist"
          value={form.denyInput}
          entries={form.config.ip_denylist}
          emptyLabel="No denylist entries configured."
          badgeVariant="warning"
          onChange={form.setDenyInput}
          onAdd={form.addDenyEntry}
          onRemove={form.removeDenyEntry}
        />

        <SecurityCIDRListField
          id={`security-allowlist-${deployment.id}`}
          label="IP allowlist"
          value={form.allowInput}
          entries={form.config.ip_allowlist}
          emptyLabel="No allowlist entries configured."
          badgeVariant="info"
          onChange={form.setAllowInput}
          onAdd={form.addAllowEntry}
          onRemove={form.removeAllowEntry}
        />

        <div className="space-y-2">
          <Label htmlFor={`security-custom-rules-${deployment.id}`}>Custom rules</Label>
          <textarea
            id={`security-custom-rules-${deployment.id}`}
            value={form.customRulesText}
            onChange={event => form.setCustomRules(event.target.value)}
            placeholder={'SecRule REQUEST_URI "@contains blocked" "id:10001,phase:1,deny,status:403"'}
            className="min-h-28 w-full rounded-md border border-input bg-transparent px-3 py-2 font-mono text-xs text-foreground shadow-xs outline-none transition-[color,box-shadow] placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]"
          />
          <p className="text-xs text-muted-foreground">Use one ModSecurity SecRule per line.</p>
        </div>

        {form.inputError ? <p className="text-xs text-destructive">{form.inputError}</p> : null}
        {form.formError ? <p className="text-xs text-destructive">{form.formError}</p> : null}

        <div className="flex items-center gap-3">
          <Button type="submit" disabled={form.isSaving || !form.hasChanges}>
            {form.isSaving ? 'Saving security...' : 'Save security'}
          </Button>
          {!form.hasChanges && !form.isDirty ? <p className="text-xs text-muted-foreground">No pending security changes.</p> : null}
        </div>
      </form>
    </section>
  )
}
