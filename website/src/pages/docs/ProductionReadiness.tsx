import DocsPage from '@/pages/docs/DocsPage'
import productionReadinessMarkdown from '@/content/docs/production-readiness.md?raw'

export default function ProductionReadiness() {
  return <DocsPage markdown={productionReadinessMarkdown} />
}
