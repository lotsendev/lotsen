import { type FormEvent } from 'react'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Label } from '../components/ui/label'
import { type Deployment } from '../lib/api'
import { SecurityCIDRListField } from './SecurityCIDRListField'
import { splitRules } from './securityConfig'
import { useDeploymentSecurityForm } from './useDeploymentSecurityForm'

type Props = {
  deployment: Deployment
}

export function DeploymentSecurityPanel({ deployment }: Props) {
  const form = useDeploymentSecurityForm(deployment)
  const customRuleCount = splitRules(form.customRulesText).length

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
        <Badge variant={form.config.waf_mode === 'enforcement' ? 'destructive' : 'warning'} className="capitalize">
          Mode: {form.config.waf_mode}
        </Badge>
      </div>

      <form onSubmit={handleSubmit} className="mt-4 space-y-4">
        <label className="flex items-center gap-2 text-sm text-foreground">
          <input
            type="checkbox"
            checked={form.config.waf_enabled}
            onChange={event => form.setWAFEnabled(event.target.checked)}
          />
          Enable WAF for this deployment
        </label>

        <div className="space-y-2">
          <Label htmlFor={`security-waf-mode-${deployment.id}`}>WAF mode</Label>
          <select
            id={`security-waf-mode-${deployment.id}`}
            value={form.config.waf_mode}
            onChange={event => form.setWAFMode(event.target.value as 'detection' | 'enforcement')}
            className="h-9 w-full rounded-md border border-input bg-transparent px-3 py-1.5 text-sm text-foreground shadow-xs outline-none transition-[color,box-shadow] focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]"
          >
            <option value="detection">Detection (log only)</option>
            <option value="enforcement">Enforcement (block requests)</option>
          </select>
        </div>

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

        <div className="space-y-2 rounded-md border border-border/60 bg-background/60 p-3">
          <p className="text-xs font-medium text-foreground">Effective rules for this deployment</p>
          <ul className="space-y-1 text-xs text-muted-foreground">
            <li>Custom deployment rules: {customRuleCount}</li>
            <li>
              Rule syntax guide:{' '}
              <a
                href="https://coraza.io/docs/seclang/directives/"
                target="_blank"
                rel="noreferrer"
                className="text-sky-700 underline decoration-sky-300 underline-offset-2 transition-colors hover:text-sky-800"
              >
                Coraza SecLang directives
              </a>
            </li>
          </ul>
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
