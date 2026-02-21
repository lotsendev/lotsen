import { useCreateDeploymentForm, type EnvRow, type PairRow } from './useCreateDeploymentForm'
import { DynamicSection } from './DynamicSection'

const inputCls = 'h-9 rounded-md border border-gray-300 px-3 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900 w-full'
const inputErrCls = 'h-9 rounded-md border border-red-400 px-3 text-sm focus:outline-none focus:ring-2 focus:ring-red-500 w-full'

export default function CreateDeploymentForm() {
  const {
    name, setName, image, setImage, domain, setDomain,
    envRows, portRows, volumeRows,
    errors, handleSubmit, isPending,
  } = useCreateDeploymentForm()

  return (
    <section className="mb-8 p-6 border border-gray-200 rounded-lg bg-white shadow-sm">
      <h2 className="text-sm font-medium text-gray-700 mb-4">New deployment</h2>
      <form onSubmit={handleSubmit} noValidate className="space-y-5">

        <div className="grid grid-cols-2 gap-3">
          <div className="flex flex-col gap-1">
            <label htmlFor="dep-name" className="text-xs text-gray-500">Name *</label>
            <input id="dep-name" type="text" placeholder="my-app" value={name}
              onChange={e => setName(e.target.value)}
              className={errors.name ? inputErrCls : inputCls} />
            {errors.name && <p className="text-xs text-red-600">{errors.name}</p>}
          </div>
          <div className="flex flex-col gap-1">
            <label htmlFor="dep-image" className="text-xs text-gray-500">Image *</label>
            <input id="dep-image" type="text" placeholder="nginx:latest" value={image}
              onChange={e => setImage(e.target.value)}
              className={errors.image ? inputErrCls : inputCls} />
            {errors.image && <p className="text-xs text-red-600">{errors.image}</p>}
          </div>
        </div>

        <div className="flex flex-col gap-1">
          <label htmlFor="dep-domain" className="text-xs text-gray-500">Domain (optional)</label>
          <input id="dep-domain" type="text" placeholder="app.example.com" value={domain}
            onChange={e => setDomain(e.target.value)} className={inputCls} />
        </div>

        <DynamicSection<EnvRow>
          title="Environment variables" addLabel="Add env var" removeLabel="Remove env var"
          rows={envRows.rows} onAdd={envRows.add} onRemove={envRows.remove}
          errorFor={row => errors.envs[row.id]}
          renderRow={row => (<>
            <input type="text" placeholder="KEY" value={row.key}
              onChange={e => envRows.update(row.id, { key: e.target.value })}
              className={`${errors.envs[row.id] ? inputErrCls : inputCls} font-mono`} />
            <input type="text" placeholder="value" value={row.value}
              onChange={e => envRows.update(row.id, { value: e.target.value })}
              className={inputCls} />
          </>)}
        />

        <DynamicSection<PairRow>
          title="Port mappings" addLabel="Add port mapping" removeLabel="Remove port mapping"
          rows={portRows.rows} onAdd={portRows.add} onRemove={portRows.remove}
          errorFor={row => errors.ports[row.id]}
          renderRow={row => (<>
            <input type="text" placeholder="Host port" value={row.left}
              onChange={e => portRows.update(row.id, { left: e.target.value })}
              className={errors.ports[row.id] ? inputErrCls : inputCls} />
            <span className="text-gray-400 shrink-0 text-sm">:</span>
            <input type="text" placeholder="Container port" value={row.right}
              onChange={e => portRows.update(row.id, { right: e.target.value })}
              className={errors.ports[row.id] ? inputErrCls : inputCls} />
          </>)}
        />

        <DynamicSection<PairRow>
          title="Volume mounts" addLabel="Add volume mount" removeLabel="Remove volume mount"
          rows={volumeRows.rows} onAdd={volumeRows.add} onRemove={volumeRows.remove}
          errorFor={row => errors.volumes[row.id]}
          renderRow={row => (<>
            <input type="text" placeholder="/host/path" value={row.left}
              onChange={e => volumeRows.update(row.id, { left: e.target.value })}
              className={`${errors.volumes[row.id] ? inputErrCls : inputCls} font-mono`} />
            <span className="text-gray-400 shrink-0 text-sm">:</span>
            <input type="text" placeholder="/container/path" value={row.right}
              onChange={e => volumeRows.update(row.id, { right: e.target.value })}
              className={`${errors.volumes[row.id] ? inputErrCls : inputCls} font-mono`} />
          </>)}
        />

        {errors.form && <p className="text-xs text-red-600">{errors.form}</p>}
        <button type="submit" disabled={isPending}
          className="h-9 px-4 rounded-md bg-gray-900 text-white text-sm font-medium hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed">
          {isPending ? 'Creating…' : 'Create'}
        </button>
      </form>
    </section>
  )
}
