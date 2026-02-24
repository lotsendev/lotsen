import DocsPage from '@/pages/docs/DocsPage'
import deploymentConfigurationMarkdown from '@/content/docs/deployment-configuration.md?raw'

export default function DeploymentConfiguration() {
  return <DocsPage markdown={deploymentConfigurationMarkdown} />
}
