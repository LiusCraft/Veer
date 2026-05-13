import { useState, useEffect } from 'react'
import {
  Box,
  Grid,
  Typography,
  Card,
  CardContent,
  CardHeader,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Alert,
  Skeleton,
  useTheme,
} from '@mui/material'
import TrendingUpIcon from '@mui/icons-material/TrendingUp'
import StorageIcon from '@mui/icons-material/Storage'
import RouteIcon from '@mui/icons-material/Route'
import TodayIcon from '@mui/icons-material/Today'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import { statsApi } from '../api/index.js'
import StatCard from '../components/StatCard.jsx'
import StatusChip from '../components/StatusChip.jsx'

/**
 * Dashboard page showing CDN overview statistics and recent activity.
 */
function Dashboard() {
  const [overview, setOverview] = useState(null)
  const [traffic, setTraffic] = useState([])
  const [logs, setLogs] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const theme = useTheme()
  const isDark = theme.palette.mode === 'dark'

  useEffect(() => {
    loadData()
  }, [])

  const loadData = async () => {
    setLoading(true)
    setError('')
    try {
      const [overviewRes, trafficRes, logsRes] = await Promise.all([
        statsApi.overview(),
        statsApi.traffic(),
        statsApi.logs({ page: 1, page_size: 5 }),
      ])
      setOverview(overviewRes.data)
      setTraffic(trafficRes.data || [])
      setLogs(logsRes.data || [])
    } catch (err) {
      setError(err.message || '加载数据失败')
    } finally {
      setLoading(false)
    }
  }

  const formatDateTime = (dateStr) => {
    if (!dateStr) return '-'
    return new Date(dateStr).toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  }

  return (
    <Box>
      <Typography variant="h5" sx={{ mb: 3, fontWeight: 600 }}>
        概览仪表盘
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      {/* Stat Cards */}
      <Grid container spacing={3} sx={{ mb: 3 }}>
        <Grid item xs={12} sm={6} md={3}>
          <StatCard
            title="总跳转次数"
            value={loading ? '-' : (overview?.total_redirects ?? 0).toLocaleString()}
            icon={<TrendingUpIcon />}
            color="#1976d2"
            loading={loading}
            subtitle="累计 302 跳转"
          />
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <StatCard
            title="活跃节点数"
            value={loading ? '-' : overview?.active_nodes ?? 0}
            icon={<StorageIcon />}
            color="#2e7d32"
            loading={loading}
            subtitle="状态为活跃的节点"
          />
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <StatCard
            title="规则数量"
            value={loading ? '-' : overview?.total_rules ?? 0}
            icon={<RouteIcon />}
            color="#9c27b0"
            loading={loading}
            subtitle="已配置跳转规则"
          />
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <StatCard
            title="今日请求"
            value={loading ? '-' : (overview?.today_requests ?? 0).toLocaleString()}
            icon={<TodayIcon />}
            color="#ed6c02"
            loading={loading}
            subtitle="今日 CDN 请求数"
          />
        </Grid>
      </Grid>

      <Grid container spacing={3}>
        {/* Traffic Chart */}
        <Grid item xs={12} md={8}>
          <Card>
            <CardHeader title="近7天流量趋势" titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }} />
            <CardContent>
              {loading ? (
                <Skeleton variant="rectangular" height={220} />
              ) : (
                <ResponsiveContainer width="100%" height={220}>
                  <LineChart data={traffic} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke={isDark ? 'rgba(255,255,255,0.1)' : '#f0f0f0'} />
                    <XAxis dataKey="date" tick={{ fontSize: 12, fill: isDark ? '#fff' : undefined }} />
                    <YAxis tick={{ fontSize: 12, fill: isDark ? '#fff' : undefined }} allowDecimals={false} />
                    <Tooltip
                      contentStyle={{
                        borderRadius: 8,
                        border: `1px solid ${isDark ? 'rgba(255,255,255,0.2)' : '#e0e0e0'}`,
                        backgroundColor: isDark ? '#2d2d2d' : '#fff',
                        color: isDark ? '#fff' : undefined,
                      }}
                      formatter={(value) => [value, '请求数']}
                    />
                    <Line
                      type="monotone"
                      dataKey="count"
                      stroke={isDark ? '#90caf9' : '#1976d2'}
                      strokeWidth={2.5}
                      dot={{ r: 4, fill: isDark ? '#90caf9' : '#1976d2' }}
                      activeDot={{ r: 6 }}
                    />
                  </LineChart>
                </ResponsiveContainer>
              )}
            </CardContent>
          </Card>
        </Grid>

        {/* Recent Logs */}
        <Grid item xs={12} md={4}>
          <Card sx={{ height: '100%' }}>
            <CardHeader title="最近访问记录" titleTypographyProps={{ variant: 'subtitle1', fontWeight: 600 }} />
            <CardContent sx={{ p: 0 }}>
              {loading ? (
                <Box sx={{ p: 2 }}>
                  {[...Array(5)].map((_, i) => (
                    <Skeleton key={i} variant="text" sx={{ mb: 1 }} height={32} />
                  ))}
                </Box>
              ) : logs.length === 0 ? (
                <Typography variant="body2" color="text.secondary" sx={{ p: 2, textAlign: 'center' }}>
                  暂无访问记录
                </Typography>
              ) : (
                <TableContainer>
                  <Table size="small">
                    <TableHead>
                      <TableRow>
                        <TableCell>规则</TableCell>
                        <TableCell>节点</TableCell>
                        <TableCell>时间</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {logs.map((log) => (
                        <TableRow key={log.id} hover>
                          <TableCell>
                            <Typography variant="caption" sx={{ fontFamily: 'monospace', fontWeight: 600 }}>
                              {log.rule_key}
                            </Typography>
                          </TableCell>
                          <TableCell>
                            <Typography variant="caption" noWrap sx={{ maxWidth: 80, display: 'block' }}>
                              {log.node_name}
                            </Typography>
                          </TableCell>
                          <TableCell>
                            <Typography variant="caption" color="text.secondary">
                              {formatDateTime(log.created_at)}
                            </Typography>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </TableContainer>
              )}
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    </Box>
  )
}

export default Dashboard
