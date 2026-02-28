import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import Landing from '@/pages/Landing'
import DocsLayout from '@/pages/docs/DocsLayout'
import GettingStarted from '@/pages/docs/GettingStarted'
import DeploymentConfiguration from '@/pages/docs/DeploymentConfiguration'
import StrictModeSetup from '@/pages/docs/StrictModeSetup'
import ProductionReadiness from '@/pages/docs/ProductionReadiness'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Landing />} />
        <Route path="/docs" element={<DocsLayout />}>
          <Route index element={<Navigate to="getting-started" replace />} />
          <Route path="getting-started" element={<GettingStarted />} />
          <Route path="deployment-configuration" element={<DeploymentConfiguration />} />
          <Route path="strict-mode-setup" element={<StrictModeSetup />} />
          <Route path="production-readiness" element={<ProductionReadiness />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
