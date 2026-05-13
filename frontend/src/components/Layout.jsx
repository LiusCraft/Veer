import { useState } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import {
  Box, Drawer, List, ListItem, ListItemButton, ListItemIcon, ListItemText,
  Typography, AppBar, Toolbar, Divider, Avatar, IconButton, Tooltip, Collapse,
} from '@mui/material'
import DashboardIcon from '@mui/icons-material/Dashboard'
import StorageIcon from '@mui/icons-material/Storage'
import RouteIcon from '@mui/icons-material/Route'
import ArticleIcon from '@mui/icons-material/Article'
import SpeedIcon from '@mui/icons-material/Speed'
import Brightness4Icon from '@mui/icons-material/Brightness4'
import Brightness7Icon from '@mui/icons-material/Brightness7'
import LogoutIcon from '@mui/icons-material/Logout'
import ExpandLessIcon from '@mui/icons-material/ExpandLess'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import DnsIcon from '@mui/icons-material/Dns'
import MapIcon from '@mui/icons-material/Map'
import GroupWorkIcon from '@mui/icons-material/GroupWork'
import { useAuth } from '../context/AuthContext.jsx'
import { useTheme } from '../context/ThemeContext.jsx'

const DRAWER_WIDTH = 240

const navItems = [
  { label: '概览仪表盘', path: '/dashboard', icon: <DashboardIcon /> },
  { label: '调度视图', path: '/views', icon: <MapIcon /> },
  {
    label: '资源管理', icon: <StorageIcon />,
    children: [
      { label: '集群管理', path: '/clusters', icon: <GroupWorkIcon /> },
      { label: '节点管理', path: '/nodes', icon: <DnsIcon /> },
    ],
  },
  { label: '域名管理', path: '/rules', icon: <RouteIcon /> },
  { label: '访问日志', path: '/logs', icon: <ArticleIcon /> },
]

function findActive(pathname) {
  for (const item of navItems) {
    if (item.path === pathname) return item
    if (item.children) {
      const child = item.children.find(c => c.path === pathname)
      if (child) return child
    }
  }
  return null
}

function Layout({ children }) {
  const navigate = useNavigate()
  const location = useLocation()
  const { user, logout } = useAuth()
  const { mode, toggleTheme } = useTheme()
  const [resourceOpen, setResourceOpen] = useState(true)

  const isDark = mode === 'dark'
  const activeItem = findActive(location.pathname)
  const pageTitle = activeItem?.label || '302CDN 管理系统'

  return (
    <Box sx={{ display: 'flex', height: '100vh' }}>
      <AppBar position="fixed"
        sx={{
          zIndex: theme => theme.zIndex.drawer + 1,
          background: isDark ? 'linear-gradient(135deg, #1e1e1e 0%, #2d2d2d 100%)' : 'linear-gradient(135deg, #1976d2 0%, #1565c0 100%)',
          boxShadow: isDark ? '0 2px 8px rgba(0,0,0,0.5)' : '0 2px 8px rgba(25, 118, 210, 0.3)',
          color: isDark ? '#ffffff' : undefined,
        }}>
        <Toolbar>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mr: 3 }}>
            <Avatar sx={{ bgcolor: 'rgba(255,255,255,0.2)', width: 36, height: 36 }}>
              <SpeedIcon fontSize="small" />
            </Avatar>
            <Typography variant="h6" noWrap sx={{ color: 'white', fontWeight: 700 }}>302CDN</Typography>
          </Box>
          <Divider orientation="vertical" flexItem sx={{ bgcolor: 'rgba(255,255,255,0.3)', mx: 1 }} />
          <Typography variant="subtitle1" sx={{ color: 'rgba(255,255,255,0.9)', ml: 2, flexGrow: 1 }}>
            {pageTitle}
          </Typography>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
            <Tooltip title={isDark ? '切换亮色模式' : '切换暗色模式'}>
              <IconButton onClick={toggleTheme} size="small" sx={{ color: 'rgba(255,255,255,0.85)', '&:hover': { bgcolor: 'rgba(255,255,255,0.12)', color: '#fff' } }}>
                {isDark ? <Brightness7Icon /> : <Brightness4Icon />}
              </IconButton>
            </Tooltip>
            {user && <Typography variant="body2" sx={{ color: 'rgba(255,255,255,0.9)', mx: 1, fontWeight: 500 }}>{user.username}</Typography>}
            <Tooltip title="退出登录">
              <IconButton onClick={logout} size="small" sx={{ color: 'rgba(255,255,255,0.85)', '&:hover': { bgcolor: 'rgba(244,67,54,0.15)', color: '#ef5350' } }}>
                <LogoutIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Box>
        </Toolbar>
      </AppBar>

      <Drawer variant="permanent"
        sx={{
          width: DRAWER_WIDTH, flexShrink: 0,
          '& .MuiDrawer-paper': {
            width: DRAWER_WIDTH, boxSizing: 'border-box',
            borderRight: `1px solid ${isDark ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.08)'}`,
            background: isDark ? '#1e1e1e' : '#fff',
          },
        }}>
        <Toolbar />
        <Box sx={{ overflow: 'auto', pt: 1 }}>
          <List>
            {navItems.map((item) => {
              if (item.children) {
                const childActive = item.children.some(c => location.pathname === c.path)
                return (
                  <Box key={item.label}>
                    <ListItem disablePadding sx={{ px: 1, mb: 0.5 }}>
                      <ListItemButton onClick={() => setResourceOpen(!resourceOpen)}
                        sx={{ borderRadius: 2, bgcolor: childActive ? 'action.selected' : 'transparent' }}>
                        <ListItemIcon sx={{ minWidth: 40, color: childActive ? 'primary.main' : 'text.secondary' }}>
                          {item.icon}
                        </ListItemIcon>
                        <ListItemText primary={item.label}
                          primaryTypographyProps={{ fontSize: '0.875rem', fontWeight: childActive ? 600 : 400 }} />
                        {resourceOpen ? <ExpandLessIcon fontSize="small" /> : <ExpandMoreIcon fontSize="small" />}
                      </ListItemButton>
                    </ListItem>
                    <Collapse in={resourceOpen} timeout="auto" unmountOnExit>
                      <List disablePadding>
                        {item.children.map(child => {
                          const isChildActive = location.pathname === child.path
                          return (
                            <ListItem key={child.path} disablePadding sx={{ pl: 2, pr: 1, mb: 0.3 }}>
                              <ListItemButton
                                onClick={() => { if (!child.disabled) navigate(child.path) }}
                                selected={isChildActive}
                                disabled={child.disabled}
                                sx={{
                                  borderRadius: 2, pl: 5,
                                  '&.Mui-selected': {
                                    backgroundColor: 'primary.main', color: 'white',
                                    '& .MuiListItemIcon-root': { color: 'white' },
                                    '&:hover': { backgroundColor: 'primary.dark' },
                                  },
                                }}>
                                <ListItemIcon sx={{ minWidth: 32, color: isChildActive ? 'white' : 'text.secondary' }}>
                                  {child.icon}
                                </ListItemIcon>
                                <ListItemText primary={child.label}
                                  primaryTypographyProps={{ fontSize: '0.85rem', fontWeight: isChildActive ? 600 : 400 }} />
                              </ListItemButton>
                            </ListItem>
                          )
                        })}
                      </List>
                    </Collapse>
                  </Box>
                )
              }

              const isActive = location.pathname === item.path
              return (
                <ListItem key={item.path} disablePadding sx={{ px: 1, mb: 0.5 }}>
                  <ListItemButton onClick={() => navigate(item.path)} selected={isActive}
                    sx={{
                      borderRadius: 2,
                      '&.Mui-selected': {
                        backgroundColor: 'primary.main', color: 'white',
                        '& .MuiListItemIcon-root': { color: 'white' },
                        '&:hover': { backgroundColor: 'primary.dark' },
                      },
                    }}>
                    <ListItemIcon sx={{ minWidth: 40, color: isActive ? 'white' : 'text.secondary' }}>
                      {item.icon}
                    </ListItemIcon>
                    <ListItemText primary={item.label}
                      primaryTypographyProps={{ fontSize: '0.875rem', fontWeight: isActive ? 600 : 400 }} />
                  </ListItemButton>
                </ListItem>
              )
            })}
          </List>
          <Divider sx={{ mt: 2, mx: 2 }} />
          <Box sx={{ px: 3, py: 2 }}>
            <Typography variant="caption" color="text.disabled">302CDN v2.0.0</Typography>
          </Box>
        </Box>
      </Drawer>

      <Box component="main" sx={{ flexGrow: 1, p: 3, overflow: 'auto', bgcolor: 'background.default' }}>
        <Toolbar />
        {children}
      </Box>
    </Box>
  )
}

export default Layout
