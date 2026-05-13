import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Card,
  CardContent,
  TextField,
  Button,
  Alert,
  Typography,
  Box,
  Avatar,
  InputAdornment,
  useTheme,
} from '@mui/material'
import LoginIcon from '@mui/icons-material/Login'
import SpeedIcon from '@mui/icons-material/Speed'
import VisibilityIcon from '@mui/icons-material/Visibility'
import VisibilityOffIcon from '@mui/icons-material/VisibilityOff'
import { useAuth } from '../context/AuthContext.jsx'

function Login() {
  const navigate = useNavigate()
  const { login } = useAuth()
  const theme = useTheme()
  const isDark = theme.palette.mode === 'dark'

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [showPassword, setShowPassword] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')

    if (!username.trim() || !password.trim()) {
      setError('请输入用户名和密码')
      return
    }

    try {
      setLoading(true)
      await login(username.trim(), password)
      navigate('/dashboard', { replace: true })
    } catch (err) {
      setError(err.message || '登录失败，请检查用户名和密码')
    } finally {
      setLoading(false)
    }
  }

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') {
      handleSubmit(e)
    }
  }

  return (
    <Box
      sx={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: isDark
          ? 'linear-gradient(135deg, #1a1a2e 0%, #16213e 100%)'
          : 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
        p: 2,
      }}
    >
      {/* Decorative background elements */}
      <Box
        sx={{
          position: 'absolute',
          top: '10%',
          left: '15%',
          width: 200,
          height: 200,
          borderRadius: '50%',
          bgcolor: 'rgba(255,255,255,0.06)',
          filter: 'blur(40px)',
        }}
      />
      <Box
        sx={{
          position: 'absolute',
          bottom: '15%',
          right: '10%',
          width: 250,
          height: 250,
          borderRadius: '50%',
          bgcolor: 'rgba(255,255,255,0.04)',
          filter: 'blur(50px)',
        }}
      />

      <Card
        sx={{
          maxWidth: 420,
          width: '100%',
          borderRadius: 3,
          boxShadow: isDark
            ? '0 20px 60px rgba(0,0,0,0.6)'
            : '0 20px 60px rgba(0,0,0,0.25)',
          overflow: 'visible',
          bgcolor: isDark ? '#1a1a1a' : undefined,
        }}
      >
        <CardContent sx={{ p: 4.5, pb: 5 }}>
          {/* Logo / Header */}
          <Box sx={{ textAlign: 'center', mb: 4 }}>
            <Avatar
              sx={{
                width: 64,
                height: 64,
                mx: 'auto',
                mb: 2,
                bgcolor: isDark
                  ? '#1565c0'
                  : 'linear-gradient(135deg, #1976d2 0%, #1565c0 100%)',
                boxShadow: isDark
                  ? '0 8px 24px rgba(0,0,0,0.4)'
                  : '0 8px 24px rgba(25,118,210,0.35)',
              }}
            >
              <SpeedIcon sx={{ fontSize: 32 }} />
            </Avatar>
            <Typography variant="h4" sx={{ fontWeight: 700, color: 'text.primary', mb: 0.5 }}>
              302CDN
            </Typography>
            <Typography variant="body2" color="text.secondary">
              管理系统登录
            </Typography>
          </Box>

          {/* Error Alert */}
          {error && (
            <Alert severity="error" sx={{ mb: 3, borderRadius: 2 }} onClose={() => setError('')}>
              {error}
            </Alert>
          )}

          {/* Login Form */}
          <form onSubmit={handleSubmit}>
            <TextField
              label="用户名"
              variant="outlined"
              fullWidth
              margin="normal"
              required
              autoFocus
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              onKeyDown={handleKeyDown}
              disabled={loading}
              placeholder="请输入用户名"
              sx={{
                '& .MuiOutlinedInput-root': {
                  borderRadius: 2,
                  transition: 'all 0.2s',
                  '&.Mui-focused': {
                    boxShadow: isDark
                      ? '0 0 0 3px rgba(144,202,249,0.12)'
                      : '0 0 0 3px rgba(25,118,210,0.12)',
                  },
                },
                mb: 2.5,
              }}
            />

            <TextField
              label="密码"
              type={showPassword ? 'text' : 'password'}
              variant="outlined"
              fullWidth
              margin="normal"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              onKeyDown={handleKeyDown}
              disabled={loading}
              placeholder="请输入密码"
              slotProps={{
                input: {
                  endAdornment: (
                    <InputAdornment position="end">
                      <Button
                        onClick={() => setShowPassword(!showPassword)}
                        sx={{ minWidth: 36, px: 0.5 }}
                        size="small"
                        tabIndex={-1}
                      >
                        {showPassword ? (
                          <VisibilityOffIcon fontSize="small" />
                        ) : (
                          <VisibilityIcon fontSize="small" />
                        )}
                      </Button>
                    </InputAdornment>
                  ),
                },
              }}
              sx={{
                '& .MuiOutlinedInput-root': {
                  borderRadius: 2,
                  transition: 'all 0.2s',
                  '&.Mui-focused': {
                    boxShadow: isDark
                      ? '0 0 0 3px rgba(144,202,249,0.12)'
                      : '0 0 0 3px rgba(25,118,210,0.12)',
                  },
                },
                mb: 3.5,
              }}
            />

            <Button
              type="submit"
              variant="contained"
              fullWidth
              size="large"
              loading={loading}
              disabled={loading}
              startIcon={!loading && <LoginIcon />}
              sx={{
                py: 1.5,
                borderRadius: 2,
                textTransform: 'none',
                fontWeight: 600,
                fontSize: '1rem',
                background: isDark
                  ? 'linear-gradient(135deg, #1565c0 0%, #0d47a1 100%)'
                  : 'linear-gradient(135deg, #1976d2 0%, #1565c0 100%)',
                boxShadow: isDark
                  ? '0 4px 14px rgba(0,0,0,0.4)'
                  : '0 4px 14px rgba(25,118,210,0.35)',
                color: '#fff',
                '&:hover': {
                  boxShadow: isDark
                    ? '0 6px 20px rgba(0,0,0,0.5)'
                    : '0 6px 20px rgba(25,118,210,0.45)',
                  background: isDark
                    ? 'linear-gradient(135deg, #0d47a1 0%, #002171 100%)'
                    : 'linear-gradient(135deg, #1565c0 0%, #0d47a1 100%)',
                },
                '& .MuiLoadingIndicator-root': {
                  color: '#fff',
                },
              }}
            >
              {loading ? '登录中...' : '登 录'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </Box>
  )
}

export default Login
