import { useState, useEffect, useCallback } from 'react'
import {
  Box, Typography, Card, Button, Chip, Stack, CircularProgress,
  Alert, Grid, IconButton, Tooltip, FormControl, InputLabel, Select, MenuItem,
} from '@mui/material'
import RefreshIcon from '@mui/icons-material/Refresh'
import FiberManualRecordIcon from '@mui/icons-material/FiberManualRecord'
import { viewsApi } from '../api/index.js'

const HEALTH_COLORS = {
  healthy: { color: '#4caf50', label: '健康' },
  overload: { color: '#ff9800', label: '过载' },
  degraded: { color: '#f44336', label: '故障' },
  down: { color: '#9e9e9e', label: '离线' },
}

const NODE_COLORS = {
  healthy: '#4caf50',
  overload: '#ff9800',
  inactive: '#f44336',
  offline: '#9e9e9e',
}

function Views() {
  const [topology, setTopology] = useState(null)
  const [healthMatrix, setHealthMatrix] = useState([])
  const [traffic, setTraffic] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [filterRegion, setFilterRegion] = useState('')
  const [filterIsp, setFilterIsp] = useState('')
  const [lastUpdated, setLastUpdated] = useState(null)

  const loadData = useCallback(async () => {
    setError('')
    try {
      const [topoRes, healthRes, trafficRes] = await Promise.all([
        viewsApi.topology(),
        viewsApi.healthMatrix(),
        viewsApi.trafficDistribution(),
      ])
      setTopology(topoRes.data)
      setHealthMatrix(healthRes.data || [])
      setTraffic(trafficRes.data)
      setLastUpdated(new Date())
    } catch (err) {
      setError(err.message)
    }
    setLoading(false)
  }, [])

  useEffect(() => { loadData() }, [loadData])

  useEffect(() => {
    if (!autoRefresh) return
    const interval = setInterval(loadData, 30000)
    return () => clearInterval(interval)
  }, [autoRefresh, loadData])

  // Filter health matrix
  const filteredHealth = healthMatrix.filter(h => {
    if (filterRegion && h.region !== filterRegion) return false
    if (filterIsp && h.isp !== filterIsp) return false
    return true
  })

  // Extract unique regions and isps for filters
  const regions = [...new Set(healthMatrix.map(h => h.region).filter(Boolean))]
  const isps = [...new Set(healthMatrix.map(h => h.isp).filter(Boolean))]

  // Node heatmap data from topology
  const heatmapData = topology?.clusters || []

  // Cluster health status for clustered nodes
  const getNodeColor = (node) => {
    if (node.status === 'inactive') return NODE_COLORS.inactive
    const fiveMinAgo = new Date(Date.now() - 5 * 60 * 1000)
    if (!node.last_heartbeat || new Date(node.last_heartbeat) < fiveMinAgo) return NODE_COLORS.offline
    if (node.cpu_usage >= 70 || node.latency >= 300) return NODE_COLORS.overload
    return NODE_COLORS.healthy
  }

  const maxBandwidth = traffic?.clusters?.length
    ? Math.max(...traffic.clusters.map(c => c.bandwidth_gbps), 0.1)
    : 1

  if (loading) return <Box sx={{ textAlign: 'center', py: 8 }}><CircularProgress size={32} /></Box>

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 3 }}>
        <Typography variant="h5" sx={{ fontWeight: 600 }}>调度视图</Typography>
        <Stack direction="row" spacing={1} alignItems="center">
          <Chip label={autoRefresh ? '自动刷新 30s' : '自动刷新 关'}
            size="small" color={autoRefresh ? 'success' : 'default'}
            onClick={() => setAutoRefresh(!autoRefresh)}
            variant="outlined" sx={{ cursor: 'pointer' }} />
          <IconButton onClick={loadData} size="small"><RefreshIcon /></IconButton>
        </Stack>
      </Box>

      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

      {/* Filters */}
      <Card sx={{ p: 2, mb: 2, display: 'flex', gap: 2, alignItems: 'center' }}>
        <Typography variant="body2" sx={{ fontWeight: 600 }}>筛选:</Typography>
        <FormControl size="small" sx={{ minWidth: 120 }}>
          <InputLabel>区域</InputLabel>
          <Select value={filterRegion} label="区域" onChange={e => setFilterRegion(e.target.value)}>
            <MenuItem value="">全部</MenuItem>
            {regions.map(r => <MenuItem key={r} value={r}>{r}</MenuItem>)}
          </Select>
        </FormControl>
        <FormControl size="small" sx={{ minWidth: 120 }}>
          <InputLabel>运营商</InputLabel>
          <Select value={filterIsp} label="运营商" onChange={e => setFilterIsp(e.target.value)}>
            <MenuItem value="">全部</MenuItem>
            {isps.map(i => <MenuItem key={i} value={i}>{i}</MenuItem>)}
          </Select>
        </FormControl>
      </Card>

      <Grid container spacing={2}>
        {/* Health Matrix */}
        <Grid item xs={12}>
          <Card sx={{ p: 2 }}>
            <Typography variant="subtitle1" sx={{ fontWeight: 700, mb: 2 }}>集群健康矩阵</Typography>
            {filteredHealth.length === 0 ? (
              <Typography variant="body2" color="text.secondary">暂无数据</Typography>
            ) : (
              <Box>
                {/* Table header */}
                <Box sx={{ display: 'flex', px: 2, py: 1, bgcolor: (theme) => theme.palette.mode === 'dark' ? 'grey.800' : 'grey.100', borderRadius: 1, mb: 1 }}>
                  <Typography variant="caption" sx={{ flex: 2, fontWeight: 700 }}>集群</Typography>
                  <Typography variant="caption" sx={{ flex: 1, fontWeight: 700, textAlign: 'center' }}>在/总</Typography>
                  <Typography variant="caption" sx={{ flex: 1, fontWeight: 700, textAlign: 'center' }}>CPU</Typography>
                  <Typography variant="caption" sx={{ flex: 1, fontWeight: 700, textAlign: 'center' }}>延迟</Typography>
                  <Typography variant="caption" sx={{ flex: 1, fontWeight: 700, textAlign: 'center' }}>带宽</Typography>
                  <Typography variant="caption" sx={{ flex: 1, fontWeight: 700, textAlign: 'center' }}>状态</Typography>
                </Box>
                {filteredHealth.map(h => {
                  const hc = HEALTH_COLORS[h.health_status] || HEALTH_COLORS.healthy
                  return (
                    <Box key={h.cluster_id} sx={{
                      display: 'flex', px: 2, py: 1.5, alignItems: 'center',
                      borderBottom: '1px solid', borderColor: 'divider',
                      '&:hover': { bgcolor: 'action.hover' },
                    }}>
                      <Typography variant="body2" sx={{ flex: 2, fontWeight: 600 }}>
                        {h.cluster_name}
                      </Typography>
                      <Typography variant="body2" sx={{ flex: 1, textAlign: 'center' }}>
                        {h.online_nodes}/{h.total_nodes}
                      </Typography>
                      <Typography variant="body2" sx={{ flex: 1, textAlign: 'center' }}>
                        {h.avg_cpu?.toFixed(0) || '-'}%
                      </Typography>
                      <Typography variant="body2" sx={{ flex: 1, textAlign: 'center' }}>
                        {h.avg_latency?.toFixed(0) || '-'}ms
                      </Typography>
                      <Typography variant="body2" sx={{ flex: 1, textAlign: 'center' }}>
                        {h.total_bandwidth_mbps ? `${(h.total_bandwidth_mbps / 1000).toFixed(1)}Gbps` : '-'}
                      </Typography>
                      <Box sx={{ flex: 1, textAlign: 'center' }}>
                        <Chip icon={<FiberManualRecordIcon sx={{ fontSize: 10 }} />}
                          label={hc.label} size="small"
                          sx={{ bgcolor: hc.color, color: 'white', fontWeight: 600,
                            '& .MuiChip-icon': { color: 'white' } }} />
                      </Box>
                    </Box>
                  )
                })}
              </Box>
            )}
          </Card>
        </Grid>

        {/* Node Heatmap */}
        <Grid item xs={12} md={6}>
          <Card sx={{ p: 2 }}>
            <Typography variant="subtitle1" sx={{ fontWeight: 700, mb: 2 }}>节点热力图</Typography>
            {heatmapData.length === 0 ? (
              <Typography variant="body2" color="text.secondary">暂无数据</Typography>
            ) : (
              heatmapData.map(cl => {
                const filteredNodes = (cl.nodes || []).filter(n => {
                  // Apply same region/isp filter
                  return true
                })
                return (
                  <Box key={cl.id} sx={{ mb: 1.5 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                      <Typography variant="body2" sx={{ fontWeight: 600, minWidth: 140 }}>
                        {cl.name}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        {filteredNodes.filter(n => n.status === 'active').length}/{filteredNodes.length}
                      </Typography>
                    </Box>
                    <Stack direction="row" spacing={0.5}>
                      {filteredNodes.map(node => (
                        <Tooltip key={node.id}
                          title={`${node.name} | CPU: ${node.cpu_usage?.toFixed(1) || '-'}% | 延迟: ${node.latency || '-'}ms`}>
                          <FiberManualRecordIcon sx={{
                            fontSize: 20,
                            color: getNodeColor(node),
                          }} />
                        </Tooltip>
                      ))}
                    </Stack>
                  </Box>
                )
              })
            )}
            <Box sx={{ display: 'flex', gap: 2, mt: 2, pt: 1, borderTop: '1px solid', borderColor: 'divider' }}>
              {Object.entries(NODE_COLORS).map(([key, color]) => (
                <Box key={key} sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                  <FiberManualRecordIcon sx={{ fontSize: 14, color }} />
                  <Typography variant="caption">
                    {key === 'healthy' ? '健康' : key === 'overload' ? '高负载' : key === 'inactive' ? '故障' : '离线'}
                  </Typography>
                </Box>
              ))}
            </Box>
          </Card>
        </Grid>

        {/* Traffic Distribution */}
        <Grid item xs={12} md={6}>
          <Card sx={{ p: 2 }}>
            <Typography variant="subtitle1" sx={{ fontWeight: 700, mb: 2 }}>流量分布</Typography>
            {!traffic || !traffic.clusters || traffic.clusters.length === 0 ? (
              <Typography variant="body2" color="text.secondary">暂无流量数据</Typography>
            ) : (
              <Box>
                {traffic.clusters.map(c => (
                  <Box key={c.cluster_id} sx={{ mb: 2 }}>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
                      <Typography variant="body2" sx={{ fontWeight: 600 }}>{c.cluster_name}</Typography>
                      <Typography variant="body2" color="text.secondary">
                        {c.bandwidth_gbps.toFixed(1)} Gbps
                      </Typography>
                    </Box>
                    <Box sx={{
                      width: '100%', height: 20, bgcolor: (theme) => theme.palette.mode === 'dark' ? 'grey.800' : 'grey.100', borderRadius: 1,
                      overflow: 'hidden',
                    }}>
                      <Box sx={{
                        width: `${(c.bandwidth_gbps / maxBandwidth) * 100}%`,
                        height: '100%',
                        bgcolor: (theme) => theme.palette.primary.main,
                        borderRadius: 1,
                        transition: 'width 0.3s ease',
                        display: 'flex', alignItems: 'center', justifyContent: 'flex-end',
                        pr: 1,
                      }}>
                        <Typography variant="caption" sx={{ color: 'white', fontWeight: 600, fontSize: 10 }}>
                          {(c.bandwidth_gbps / maxBandwidth) * 100 > 15 ? `${c.requests.toLocaleString()} req` : ''}
                        </Typography>
                      </Box>
                    </Box>
                  </Box>
                ))}
                <Box sx={{ mt: 2, pt: 1, borderTop: '1px solid', borderColor: 'divider' }}>
                  <Typography variant="body2" color="text.secondary">
                    总计: {traffic.total_requests?.toLocaleString() || 0} 请求 | {traffic.total_bandwidth_gbps?.toFixed(1) || 0} Gbps
                  </Typography>
                </Box>
              </Box>
            )}
          </Card>
        </Grid>
      </Grid>

      {lastUpdated && (
        <Typography variant="caption" color="text.disabled" sx={{ display: 'block', mt: 2, textAlign: 'right' }}>
          最后更新: {lastUpdated.toLocaleString()}
        </Typography>
      )}
    </Box>
  )
}

export default Views
