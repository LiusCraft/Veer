import { Navigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext.jsx'
import CircularProgress from '@mui/material/CircularProgress'
import Box from '@mui/material/Box'

function ProtectedRoute({ children }) {
  const { isAuthenticated, loading } = useAuth()

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <CircularProgress />
      </Box>
    )
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  return children
}

export default ProtectedRoute
