import DocsPage from '@/pages/docs/DocsPage'
import strictModeSetupMarkdown from '@/content/docs/strict-mode-setup.md?raw'

export default function StrictModeSetup() {
  return <DocsPage markdown={strictModeSetupMarkdown} />
}
