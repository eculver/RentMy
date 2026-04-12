import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router'
import { QueryClientProvider } from '@tanstack/react-query'
import queryClient from './lib/query'
import { isAuthenticated } from './lib/auth'
import './index.css'
import App from './App'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import ReviewQueue from './pages/ReviewQueue'
import ReviewDetail from './pages/ReviewDetail'
import AgentDecisions from './pages/AgentDecisions'
import AgentLearning from './pages/AgentLearning'
import Alerts from './pages/Alerts'
import Referrals from './pages/Referrals'

function AuthGate({ children }: { children: React.ReactNode }) {
  if (!isAuthenticated()) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            path="/"
            element={
              <AuthGate>
                <App />
              </AuthGate>
            }
          >
            <Route index element={<Dashboard />} />
            <Route path="reviews" element={<ReviewQueue />} />
            <Route path="reviews/:flagId" element={<ReviewDetail />} />
            <Route path="decisions" element={<AgentDecisions />} />
            <Route path="learning" element={<AgentLearning />} />
            <Route path="alerts" element={<Alerts />} />
            <Route path="fraud" element={<ReviewQueue />} />
            <Route path="referrals" element={<Referrals />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
)
