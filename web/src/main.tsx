import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './styles.css'
import { AppRoutes } from './App.tsx'
import { BrowserRouter } from 'react-router'
import { AuthProvider } from './auth.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <AppRoutes />
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>,
)
