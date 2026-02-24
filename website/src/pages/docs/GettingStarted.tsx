import DocsPage from '@/pages/docs/DocsPage'
import gettingStartedMarkdown from '@/content/docs/getting-started.md?raw'

export default function GettingStarted() {
  return <DocsPage markdown={gettingStartedMarkdown} />
}
