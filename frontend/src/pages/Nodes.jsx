import { useState, useEffect } from 'react'
import {
  Box,
  Typography,
  Card,
  Button,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  IconButton,
  Tooltip,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  MenuItem,
  Alert,
  CircularProgress,
  Chip,
  Stack,
  InputAdornment,
  TableSortLabel,
  Checkbox,
  alpha,
  Snackbar,
  Drawer,
  LinearProgress,
} from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import EditIcon from '@mui/icons-material/Edit'
import DeleteIcon from '@mui/icons-material/Delete'
import NetworkCheckIcon from '@mui/icons-material/NetworkCheck'
import VisibilityIcon from '@mui/icons-material/Visibility'
import RefreshIcon from '@mui/icons-material/Refresh'
import SearchIcon from '@mui/icons-material/Search'
import FileDownloadIcon from '@mui/icons-material/FileDownload'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import PauseIcon from '@mui/icons-material/Pause'
import { nodesApi, clustersApi } from '../api/index.js'
import StatusChip from '../components/StatusChip.jsx'
import useTableSearch from '../hooks/useTableSearch.js'
import { exportToCSV } from '../utils/csv.js'

const EMPTY_FORM = {
  name: '', url: '', weight: 1, region: '', status: 'active',
  cluster_id: 0, ip: '', isp: '', provider: '', node_type: 'edge',
  bandwidth_mbps: 1000, max_connections: 10000,
}

/**
 * Nodes page for managing CDN nodes with CRUD, batch operations, and health check.
 */
function Nodes() {
  const [nodes, setNodes] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingNode, setEditingNode] = useState(null)
  const [form, setForm] = useState(EMPTY_FORM)
  const [saving, setSaving] = useState(false)
  const [testingId, setTestingId] = useState(null)
  const [selected, setSelected] = useState([])
  const [snackbar, setSnackbar] = useState({ open: false, message: '' })
  const [clusters, setClusters] = useState([])
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [detailNode, setDetailNode] = useState(null)

  const { search, setSearch, sortField, sortDir, handleSort, filteredData } = useTableSearch(
    nodes,
    ['name', 'url', 'region'],
    'name',
    'asc'
  )

  useEffect(() => {
    loadNodes()
    loadClusters()
  }, [])

  const loadClusters = async () => {
    try {
      const res = await clustersApi.list()
      setClusters(res.data || [])
    } catch {}
  }

  const loadNodes = async () => {
    setLoading(true)
    setError('')
    try {
      const res = await nodesApi.list()
      setNodes(res.data || [])
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleOpenDialog = (node = null) => {
    if (node) {
      setEditingNode(node)
      setForm({
        name: node.name, url: node.url, weight: node.weight,
        region: node.region, status: node.status,
        cluster_id: node.cluster_id || 0, ip: node.ip || '', isp: node.isp || '',
        provider: node.provider || '', node_type: node.node_type || 'edge',
        bandwidth_mbps: node.bandwidth_mbps || 1000, max_connections: node.max_connections || 10000,
      })
    } else {
      setEditingNode(null)
      setForm(EMPTY_FORM)
    }
    setDialogOpen(true)
  }

  const handleCloseDialog = () => {
    setDialogOpen(false)
    setEditingNode(null)
    setForm(EMPTY_FORM)
  }

  const handleFormChange = (field) => (e) => {
    setForm((prev) => ({ ...prev, [field]: e.target.value }))
  }

  const handleSave = async () => {
    if (!form.name.trim() || !form.url.trim()) {
      setError('名称和URL不能为空')
      return
    }
    setSaving(true)
    setError('')
    try {
      const payload = { ...form, weight: Number(form.weight) || 1 }
      if (editingNode) {
        await nodesApi.update(editingNode.id, payload)
      } else {
        await nodesApi.create(payload)
      }
      handleCloseDialog()
      loadNodes()
    } catch (err) {
      setError(err.message)
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id) => {
    if (!window.confirm('确定要删除该节点吗？')) return
    try {
      await nodesApi.delete(id)
      loadNodes()
    } catch (err) {
      setError(err.message)
    }
  }

  const handleTest = async (id) => {
    setTestingId(id)
    try {
      const res = await nodesApi.test(id)
      setNodes((prev) =>
        prev.map((n) => (n.id === id ? { ...n, latency: res.latency } : n))
      )
    } catch (err) {
      setError(err.message)
    } finally {
      setTestingId(null)
    }
  }

  const handleOpenDrawer = (node) => {
    setDetailNode(node)
    setDrawerOpen(true)
  }

  // Batch selection handlers
  const handleSelectAll = (e) => {
    if (e.target.checked) {
      setSelected(filteredData.map((n) => n.id))
    } else {
      setSelected([])
    }
  }

  const handleSelectOne = (id) => {
    setSelected((prev) =>
      prev.includes(id) ? prev.filter((s) => s !== id) : [...prev, id]
    )
  }

  const isSelected = (id) => selected.includes(id)
  const isAllSelected = filteredData.length > 0 && selected.length === filteredData.length
  const isIndeterminate = selected.length > 0 && selected.length < filteredData.length

  const handleBatchDelete = async () => {
    if (selected.length === 0) return
    if (!window.confirm(`确定要删除选中的 ${selected.length} 个节点吗？`)) return
    try {
      await nodesApi.batchDelete(selected)
      setSelected([])
      setSnackbar({ open: true, message: `成功删除 ${selected.length} 个节点` })
      loadNodes()
    } catch (err) {
      setError(err.message)
    }
  }

  const handleBatchStatusUpdate = async (status) => {
    if (selected.length === 0) return
    const label = status === 'active' ? '启用' : '停用'
    if (!window.confirm(`确定要${label}选中的 ${selected.length} 个节点吗？`)) return
    try {
      await nodesApi.batchUpdateStatus(selected, status)
      setSelected([])
      setSnackbar({ open: true, message: `成功${label} ${selected.length} 个节点` })
      loadNodes()
    } catch (err) {
      setError(err.message)
    }
  }

  const handleExportCSV = () => {
    const today = new Date().toISOString().slice(0, 10)
    const filename = `nodes-${today}.csv`
    const columns = [
      { key: 'name', label: '节点名称' },
      { key: 'url', label: 'URL地址' },
      { key: 'ip', label: 'IP地址' },
      { key: 'region', label: '区域' },
      { key: 'isp', label: '运营商' },
      { key: 'provider', label: '云厂商' },
      { key: 'node_type', label: '节点类型' },
      { key: 'cluster_id', label: '集群ID' },
      { key: 'weight', label: '权重' },
      { key: 'bandwidth_mbps', label: '带宽' },
      { key: 'cpu_usage', label: 'CPU(%)' },
      { key: 'status', label: '状态' },
      { key: 'latency', label: '延迟(ms)' },
      { key: 'created_at', label: '创建时间' },
    ]
    exportToCSV(filteredData, filename, columns)
  }

  const sortCell = (field, label, align = 'left') => (
    <TableCell align={align}>
      <TableSortLabel
        active={sortField === field}
        direction={sortField === field ? sortDir : 'asc'}
        onClick={() => handleSort(field)}
      >
        {label}
      </TableSortLabel>
    </TableCell>
  )

  const [filterCluster, setFilterCluster] = useState('')
  const [filterNodeType, setFilterNodeType] = useState('')
  const [filterProvider, setFilterProvider] = useState('')

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 3 }}>
        <Typography variant="h5" sx={{ fontWeight: 600 }}>
          资源管理
        </Typography>
        <Stack direction="row" spacing={1}>
          <Button
            variant="outlined"
            startIcon={<RefreshIcon />}
            onClick={loadNodes}
            disabled={loading}
          >
            刷新
          </Button>
          <Button
            variant="outlined"
            startIcon={<FileDownloadIcon />}
            onClick={handleExportCSV}
            disabled={loading || filteredData.length === 0}
          >
            导出 CSV
          </Button>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={() => handleOpenDialog()}
          >
            新增节点
          </Button>
        </Stack>
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      {/* Search bar */}
      <Card sx={{ p: 2, mb: 2 }}>
        <TextField
          size="small"
          placeholder="搜索节点名称、URL、区域..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">
                <SearchIcon fontSize="small" />
              </InputAdornment>
            ),
          }}
          sx={{ width: 320 }}
        />
        <TextField size="small" select value={filterCluster} onChange={e => setFilterCluster(e.target.value)}
          sx={{ minWidth: 140, ml: 2 }}>
          <MenuItem value="">全部集群</MenuItem>
          {clusters.map(cl => <MenuItem key={cl.id} value={cl.id}>{cl.name}</MenuItem>)}
        </TextField>
        <TextField size="small" select value={filterNodeType} onChange={e => setFilterNodeType(e.target.value)}
          sx={{ minWidth: 120, ml: 1 }}>
          <MenuItem value="">全部类型</MenuItem>
          <MenuItem value="edge">Edge</MenuItem>
          <MenuItem value="scheduler">Scheduler</MenuItem>
        </TextField>
        <TextField size="small" select value={filterProvider} onChange={e => setFilterProvider(e.target.value)}
          sx={{ minWidth: 120, ml: 1 }}>
          <MenuItem value="">全部厂商</MenuItem>
          <MenuItem value="aliyun">阿里云</MenuItem>
          <MenuItem value="aws">AWS</MenuItem>
          <MenuItem value="azure">Azure</MenuItem>
          <MenuItem value="self">自建</MenuItem>
          <MenuItem value="其他">其他</MenuItem>
        </TextField>
      </Card>

      {/* Batch action toolbar */}
      {selected.length > 0 && (
        <Card
          sx={{
            p: 1.5,
            mb: 2,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            bgcolor: (theme) => alpha(theme.palette.primary.main, 0.08),
          }}
        >
          <Typography variant="body2" sx={{ fontWeight: 500 }}>
            已选中 {selected.length} 个节点
          </Typography>
          <Stack direction="row" spacing={1}>
            <Button
              variant="outlined"
              color="success"
              size="small"
              startIcon={<PlayArrowIcon />}
              onClick={() => handleBatchStatusUpdate('active')}
            >
              批量启用
            </Button>
            <Button
              variant="outlined"
              color="warning"
              size="small"
              startIcon={<PauseIcon />}
              onClick={() => handleBatchStatusUpdate('inactive')}
            >
              批量停用
            </Button>
            <Button
              variant="outlined"
              color="error"
              size="small"
              startIcon={<DeleteIcon />}
              onClick={handleBatchDelete}
            >
              批量删除
            </Button>
          </Stack>
        </Card>
      )}

      <Card>
        <TableContainer>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell padding="checkbox">
                  <Checkbox
                    indeterminate={isIndeterminate}
                    checked={isAllSelected}
                    onChange={handleSelectAll}
                  />
                </TableCell>
                {sortCell('name', '节点名称')}
                <TableCell>所属集群</TableCell>
                <TableCell>IP 地址</TableCell>
                <TableCell>URL 地址</TableCell>
                {sortCell('region', '区域')}
                <TableCell align="center">CPU</TableCell>
                {sortCell('weight', '权重', 'center')}
                {sortCell('node_type', '类型', 'center')}
                {sortCell('status', '状态', 'center')}
                {sortCell('latency', '延迟 (ms)', 'center')}
                <TableCell align="center">操作</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={13} align="center" sx={{ py: 4 }}>
                    <CircularProgress size={32} />
                  </TableCell>
                </TableRow>
              ) : filteredData.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={13} align="center" sx={{ py: 4, color: 'text.secondary' }}>
                    {search ? '未找到匹配的节点' : '暂无节点，点击「新增节点」添加'}
                  </TableCell>
                </TableRow>
              ) : (
                filteredData.map((node) => {
                  const isItemSelected = isSelected(node.id)
                  return (
                    <TableRow
                      key={node.id}
                      hover
                      selected={isItemSelected}
                      sx={{ cursor: 'pointer' }}
                    >
                      <TableCell padding="checkbox">
                        <Checkbox
                          checked={isItemSelected}
                          onChange={() => handleSelectOne(node.id)}
                        />
                      </TableCell>
                      <TableCell sx={{ fontWeight: 500 }}>{node.name}</TableCell>
                      <TableCell>
                        {node.cluster_id > 0 ? (
                          <Chip label={clusters.find(c => c.id === node.cluster_id)?.name || `#${node.cluster_id}`}
                            size="small" color="primary" variant="outlined" />
                        ) : (
                          <Typography variant="body2" color="text.disabled">-</Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>
                          {node.ip || '-'}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.8rem', maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {node.url}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        {node.region ? <Chip label={node.region} size="small" variant="outlined" /> : '-'}
                      </TableCell>
                      <TableCell align="center">
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                          <LinearProgress variant="determinate" value={Math.min(node.cpu_usage || 0, 100)}
                            sx={{ width: 40, height: 6, borderRadius: 3,
                              bgcolor: theme => theme.palette.mode === 'dark' ? 'grey.800' : 'grey.200',
                              '& .MuiLinearProgress-bar': {
                                bgcolor: (node.cpu_usage || 0) >= 70 ? 'error.main' : (node.cpu_usage || 0) >= 50 ? 'warning.main' : 'success.main',
                              }
                            }} />
                          <Typography variant="caption">{node.cpu_usage?.toFixed(0) || 0}%</Typography>
                        </Box>
                      </TableCell>
                      <TableCell align="center">
                        <Chip label={node.weight} size="small" color="info" variant="outlined" />
                      </TableCell>
                      <TableCell align="center">
                        <Chip label={node.node_type || 'edge'} size="small" variant="outlined"
                          color={node.node_type === 'scheduler' ? 'warning' : 'info'} />
                      </TableCell>
                      <TableCell align="center">
                        <StatusChip status={node.status} />
                      </TableCell>
                      <TableCell align="center">
                        {node.latency > 0 ? (
                          <Typography variant="body2" sx={{ color: node.latency < 100 ? 'success.main' : node.latency < 300 ? 'warning.main' : 'error.main', fontWeight: 600 }}>
                            {node.latency} ms
                          </Typography>
                        ) : (
                          <Typography variant="body2" color="text.disabled">-</Typography>
                        )}
                      </TableCell>
                      <TableCell align="center">
                        <Stack direction="row" spacing={0.5} justifyContent="center">
                          <Tooltip title="详情">
                            <IconButton size="small" color="info" onClick={() => handleOpenDrawer(node)}>
                              <VisibilityIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title="健康检测">
                            <span>
                              <IconButton
                                size="small"
                                color="info"
                                onClick={() => handleTest(node.id)}
                                disabled={testingId === node.id}
                              >
                                {testingId === node.id ? (
                                  <CircularProgress size={16} />
                                ) : (
                                  <NetworkCheckIcon fontSize="small" />
                                )}
                              </IconButton>
                            </span>
                          </Tooltip>
                          <Tooltip title="编辑">
                            <IconButton size="small" color="primary" onClick={() => handleOpenDialog(node)}>
                              <EditIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title="删除">
                            <IconButton size="small" color="error" onClick={() => handleDelete(node.id)}>
                              <DeleteIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                        </Stack>
                      </TableCell>
                    </TableRow>
                  )
                })
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Card>

      {/* Add/Edit Dialog */}
      <Dialog open={dialogOpen} onClose={handleCloseDialog} maxWidth="sm" fullWidth>
        <DialogTitle>{editingNode ? '编辑节点' : '新增 CDN 节点'}</DialogTitle>
        <DialogContent sx={{ pt: 2 }}>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <TextField
              label="节点名称"
              value={form.name}
              onChange={handleFormChange('name')}
              required
              fullWidth
              placeholder="例如：阿里云-华东"
            />
            <TextField
              label="节点 URL"
              value={form.url}
              onChange={handleFormChange('url')}
              required
              fullWidth
              placeholder="https://cdn.example.com"
            />
            <TextField label="所属集群" select value={form.cluster_id}
              onChange={handleFormChange('cluster_id')} fullWidth>
              <MenuItem value={0}>未归属</MenuItem>
              {clusters.map(cl => (
                <MenuItem key={cl.id} value={cl.id}>{cl.name}</MenuItem>
              ))}
            </TextField>
            <TextField label="IP 地址" value={form.ip}
              onChange={handleFormChange('ip')} fullWidth placeholder="管理 IP" />
            <TextField label="运营商" select value={form.isp}
              onChange={handleFormChange('isp')} fullWidth>
              <MenuItem value="">未选择</MenuItem>
              <MenuItem value="电信">电信</MenuItem>
              <MenuItem value="联通">联通</MenuItem>
              <MenuItem value="移动">移动</MenuItem>
              <MenuItem value="aws">AWS</MenuItem>
              <MenuItem value="azure">Azure</MenuItem>
              <MenuItem value="其他">其他</MenuItem>
            </TextField>
            <TextField label="云厂商" select value={form.provider}
              onChange={handleFormChange('provider')} fullWidth>
              <MenuItem value="">未选择</MenuItem>
              <MenuItem value="aliyun">阿里云</MenuItem>
              <MenuItem value="aws">AWS</MenuItem>
              <MenuItem value="azure">Azure</MenuItem>
              <MenuItem value="self">自建</MenuItem>
              <MenuItem value="其他">其他</MenuItem>
            </TextField>
            <TextField label="节点类型" select value={form.node_type}
              onChange={handleFormChange('node_type')} fullWidth>
              <MenuItem value="edge">Edge 缓存节点</MenuItem>
              <MenuItem value="scheduler">Scheduler 调度节点</MenuItem>
            </TextField>
            <TextField label="带宽上限 (Mbps)" type="number" value={form.bandwidth_mbps}
              onChange={handleFormChange('bandwidth_mbps')} fullWidth
              inputProps={{ min: 1 }} />
            <TextField label="最大连接数" type="number" value={form.max_connections}
              onChange={handleFormChange('max_connections')} fullWidth
              inputProps={{ min: 1 }} />
            <TextField
              label="区域"
              value={form.region}
              onChange={handleFormChange('region')}
              fullWidth
              placeholder="例如：华东、华南、美国西部"
            />
            <TextField
              label="权重"
              type="number"
              value={form.weight}
              onChange={handleFormChange('weight')}
              fullWidth
              inputProps={{ min: 1, max: 100 }}
              helperText="权重越高，加权轮询时命中概率越大"
            />
            <TextField
              label="状态"
              select
              value={form.status}
              onChange={handleFormChange('status')}
              fullWidth
            >
              <MenuItem value="active">活跃</MenuItem>
              <MenuItem value="inactive">停用</MenuItem>
            </TextField>
          </Stack>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button onClick={handleCloseDialog} disabled={saving}>
            取消
          </Button>
          <Button variant="contained" onClick={handleSave} disabled={saving}>
            {saving ? <CircularProgress size={18} sx={{ mr: 1 }} /> : null}
            {editingNode ? '保存修改' : '新增节点'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Detail Drawer */}
      <Drawer anchor="right" open={drawerOpen} onClose={() => setDrawerOpen(false)}
        PaperProps={{ sx: { width: { xs: '100%', sm: 400 } } }}>
        {detailNode && (
          <Box sx={{ p: 3 }}>
            <Typography variant="h6" sx={{ fontWeight: 700, mb: 2 }}>{detailNode.name}</Typography>
            <Stack spacing={2}>
              <Box>
                <Typography variant="caption" color="text.secondary">URL</Typography>
                <Typography variant="body2">{detailNode.url}</Typography>
              </Box>
              <Box>
                <Typography variant="caption" color="text.secondary">IP 地址</Typography>
                <Typography variant="body2">{detailNode.ip || '-'}</Typography>
              </Box>
              <Box>
                <Typography variant="caption" color="text.secondary">所属集群</Typography>
                <Typography variant="body2">
                  {detailNode.cluster_id > 0
                    ? clusters.find(c => c.id === detailNode.cluster_id)?.name || `#${detailNode.cluster_id}`
                    : '未归属'}
                </Typography>
              </Box>
              <Box>
                <Typography variant="caption" color="text.secondary">状态</Typography>
                <Box><StatusChip status={detailNode.status} /></Box>
              </Box>
              <Box sx={{ display: 'flex', gap: 3 }}>
                <Box>
                  <Typography variant="caption" color="text.secondary">CPU</Typography>
                  <Typography variant="body2">{detailNode.cpu_usage?.toFixed(1) || '-'}%</Typography>
                </Box>
                <Box>
                  <Typography variant="caption" color="text.secondary">内存</Typography>
                  <Typography variant="body2">{detailNode.mem_usage?.toFixed(1) || '-'}%</Typography>
                </Box>
                <Box>
                  <Typography variant="caption" color="text.secondary">延迟</Typography>
                  <Typography variant="body2">{detailNode.latency || '-'}ms</Typography>
                </Box>
              </Box>
              <Box>
                <Typography variant="caption" color="text.secondary">最后心跳</Typography>
                <Typography variant="body2">
                  {detailNode.last_heartbeat ? new Date(detailNode.last_heartbeat).toLocaleString() : '从未上报'}
                </Typography>
              </Box>
              <Box>
                <Typography variant="caption" color="text.secondary">配置</Typography>
                <Typography variant="body2">带宽上限: {detailNode.bandwidth_mbps || 1000} Mbps</Typography>
                <Typography variant="body2">最大连接: {detailNode.max_connections || 10000}</Typography>
                <Typography variant="body2">回源地址: {detailNode.origin_base_url || '-'}</Typography>
                <Typography variant="body2">缓存 TTL: {detailNode.cache_ttl || 300}s</Typography>
              </Box>
            </Stack>
          </Box>
        )}
      </Drawer>

      <Snackbar
        open={snackbar.open}
        autoHideDuration={3000}
        onClose={() => setSnackbar({ open: false, message: '' })}
        message={snackbar.message}
      />
    </Box>
  )
}

export default Nodes
