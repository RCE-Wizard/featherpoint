import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Layout } from './components/Layout'
import { Login } from './pages/Login'
import { Fleet } from './pages/Fleet'
import { HostDetail } from './pages/HostDetail'
import { AgentDetail } from './pages/AgentDetail'
import { Unsigned } from './pages/Unsigned'
import { Dormant } from './pages/Dormant'
import { VersionSprawl } from './pages/VersionSprawl'
import { Catalog } from './pages/Catalog'
import { SoftwareHosts } from './pages/SoftwareHosts'
import { AuditLog } from './pages/AuditLog'

const qc = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 30_000 } },
})

function RequireAuth({ children }: { children: React.ReactNode }) {
  return localStorage.getItem('token') ? <>{children}</> : <Navigate to="/login" replace />
}

function Guarded({ children }: { children: React.ReactNode }) {
  return <RequireAuth><Layout>{children}</Layout></RequireAuth>
}

function App() {
  return (
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/" element={<Navigate to="/fleet" replace />} />
          <Route path="/fleet" element={<Guarded><Fleet /></Guarded>} />
          <Route path="/agents/:agentID" element={<Guarded><AgentDetail /></Guarded>} />
          <Route path="/hosts/:hostID" element={<Guarded><HostDetail /></Guarded>} />
          <Route path="/reports/unsigned" element={<Guarded><Unsigned /></Guarded>} />
          <Route path="/reports/dormant" element={<Guarded><Dormant /></Guarded>} />
          <Route path="/reports/version-sprawl" element={<Guarded><VersionSprawl /></Guarded>} />
          <Route path="/catalog" element={<Guarded><Catalog /></Guarded>} />
          <Route path="/catalog/:catalogID/hosts" element={<Guarded><SoftwareHosts /></Guarded>} />
          <Route path="/audit" element={<Guarded><AuditLog /></Guarded>} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}

export default App
