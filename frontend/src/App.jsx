import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuth } from './context/AuthContext.jsx'
import Layout from './components/Layout.jsx'
import Dashboard from './pages/Dashboard.jsx'
import Nodes from './pages/Nodes.jsx'
import Clusters from './pages/Clusters.jsx'
import Views from './pages/Views.jsx'
import Rules from './pages/Rules.jsx'
import Logs from './pages/Logs.jsx'
import Login from './pages/Login.jsx'
import ProtectedRoute from './components/ProtectedRoute.jsx'

function App() {
  const { isAuthenticated } = useAuth()

  return (
    <Routes>
      <Route
        path="/login"
        element={isAuthenticated ? <Navigate to="/dashboard" replace /> : <Login />}
      />
      <Route path="/" element={<ProtectedRoute><Layout><Dashboard /></Layout></ProtectedRoute>} />
      <Route path="/dashboard" element={<ProtectedRoute><Layout><Dashboard /></Layout></ProtectedRoute>} />
      <Route path="/nodes" element={<ProtectedRoute><Layout><Nodes /></Layout></ProtectedRoute>} />
      <Route path="/clusters" element={<ProtectedRoute><Layout><Clusters /></Layout></ProtectedRoute>} />
      <Route path="/views" element={<ProtectedRoute><Layout><Views /></Layout></ProtectedRoute>} />
      <Route path="/rules" element={<ProtectedRoute><Layout><Rules /></Layout></ProtectedRoute>} />
      <Route path="/logs" element={<ProtectedRoute><Layout><Logs /></Layout></ProtectedRoute>} />
    </Routes>
  )
}

export default App
