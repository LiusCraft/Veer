import { useState, useEffect, useCallback } from 'react'
import {
  Box, Typography, Card, Button, Table, TableBody, TableCell, TableContainer,
  TableHead, TableRow, IconButton, Tooltip, Dialog, DialogTitle, DialogContent,
  DialogActions, TextField, MenuItem, Alert, CircularProgress, Chip, Stack,
  InputAdornment, TableSortLabel, Checkbox, alpha, Snackbar, Drawer,
  Divider, Avatar, List, ListItem, ListItemText, ListItemAvatar, Toolbar,
} from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import EditIcon from '@mui/icons-material/Edit'
import DeleteIcon from '@mui/icons-material/Delete'
import RefreshIcon from '@mui/icons-material/Refresh'
import SearchIcon from '@mui/icons-material/Search'
import VisibilityIcon from '@mui/icons-material/Visibility'
import DnsIcon from '@mui/icons-material/Dns'
import RouteIcon from '@mui/icons-material/Route'
import { clustersApi } from '../api/index.js'
import StatusChip from '../components/StatusChip.jsx'
import useTableSearch from '../hooks/useTableSearch.js'

const EMPTY_FORM = {
  name: '',
  description: '',
  strategy: 'round-robin',
  region: '',
  isp: '',
  provider: '',
  status: 'active',
}

const REGIONS = ['华东', '华北', '华南', '海外', '其他']
const ISPS = ['电信', '联通', '移动', 'aws', 'azure', '其他']
const PROVIDERS = ['', 'aliyun', 'aws', 'azure', 'self', '其他']
const STRATEGIES = [
  { value: 'round-robin', label: '轮询' },
  { value: 'weighted', label: '权重' },
  { value: 'random', label: '随机' },
]

function Clusters() {
  const [clusters, setClusters] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [selectedCluster, setSelectedCluster] = useState(null)
  const [editingCluster, setEditingCluster] = useState(null)
  const [form, setForm] = useState(EMPTY_FORM)
  const [saving, setSaving] = useState(false)
  const [selected, setSelected] = useState([])
  const [snackbar, setSnackbar] = useState({ open: false, message: '' })
  const [drawerTab, setDrawerTab] = useState('nodes')

  const { search, setSearch, sortField, sortDir, handleSort, filteredData } = useTableSearch(
    clusters,
    ['name', 'region', 'isp'],
    'name',
    'asc'
  )

  useEffect(() => { loadClusters() }, [])

  const loadClusters = async () => {
    setLoading(true)
    setError('')
    try {
      const res = await clustersApi.list()
      setClusters(res.data || [])
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleOpenDialog = (cluster = null) => {
    if (cluster) {
      setEditingCluster(cluster)
      setForm({
        name: cluster.name,
        description: cluster.description || '',
        strategy: cluster.strategy || 'round-robin',
        region: cluster.region,
        isp: cluster.isp,
        provider: cluster.provider || '',
        status: cluster.status,
      })
    } else {
      setEditingCluster(null)
      setForm(EMPTY_FORM)
    }
    setDialogOpen(true)
  }

  const handleCloseDialog = () => {
    setDialogOpen(false)
    setEditingCluster(null)
    setForm(EMPTY_FORM)
  }

  const handleFormChange = (field) => (e) => {
    setForm((prev) => ({ ...prev, [field]: e.target.value }))
  }

  const handleSave = async () => {
    if (!form.name.trim()) { setError('集群名称不能为空'); return }
    if (!form.region) { setError('请选择区域'); return }
    if (!form.isp) { setError('请选择运营商'); return }
    setSaving(true)
    setError('')
    try {
      if (editingCluster) {
        await clustersApi.update(editingCluster.id, form)
      } else {
        await clustersApi.create(form)
      }
      handleCloseDialog()
      loadClusters()
    } catch (err) {
      setError(err.message)
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id) => {
    if (!window.confirm('确定要删除该集群吗？关联的节点将变为未归属状态。')) return
    try {
      await clustersApi.delete(id)
      setSnackbar({ open: true, message: '集群已删除' })
      loadClusters()
    } catch (err) {
      setError(err.message)
    }
  }

  const handleOpenDrawer = async (cluster) => {
    setSelectedCluster(cluster)
    setDrawerTab('nodes')
    setDrawerOpen(true)
  }

  const handleCloseDrawer = () => {
    setDrawerOpen(false)
    setSelectedCluster(null)
  }

  const handleSelectAll = (e) => {
    if (e.target.checked) {
      setSelected(filteredData.map((c) => c.id))
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
    if (!window.confirm(`确定要删除选中的 ${selected.length} 个集群吗？`)) return
    try {
      for (const id of selected) {
        await clustersApi.delete(id)
      }
      setSelected([])
      setSnackbar({ open: true, message: `成功删除 ${selected.length} 个集群` })
      loadClusters()
    } catch (err) {
      setError(err.message)
    }
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

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 3 }}>
        <Typography variant="h5" sx={{ fontWeight: 600 }}>集群管理</Typography>
        <Stack direction="row" spacing={1}>
          <Button variant="outlined" startIcon={<RefreshIcon />}
            onClick={loadClusters} disabled={loading}>刷新</Button>
          <Button variant="contained" startIcon={<AddIcon />}
            onClick={() => handleOpenDialog()}>新建集群</Button>
        </Stack>
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>{error}</Alert>
      )}

      <Card sx={{ p: 2, mb: 2 }}>
        <TextField size="small" placeholder="搜索集群名称、区域、运营商..."
          value={search} onChange={(e) => setSearch(e.target.value)}
          InputProps={{
            startAdornment: (<InputAdornment position="start"><SearchIcon fontSize="small" /></InputAdornment>),
          }} sx={{ width: 320 }} />
      </Card>

      {selected.length > 0 && (
        <Card sx={{ p: 1.5, mb: 2, display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          bgcolor: (theme) => alpha(theme.palette.primary.main, 0.08) }}>
          <Typography variant="body2" sx={{ fontWeight: 500 }}>已选中 {selected.length} 个集群</Typography>
          <Button variant="outlined" color="error" size="small" startIcon={<DeleteIcon />}
            onClick={handleBatchDelete}>批量删除</Button>
        </Card>
      )}

      <Card>
        <TableContainer>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell padding="checkbox">
                  <Checkbox indeterminate={isIndeterminate} checked={isAllSelected} onChange={handleSelectAll} />
                </TableCell>
                {sortCell('name', '集群名称')}
                <TableCell>区域/运营商</TableCell>
                <TableCell align="center">节点</TableCell>
                <TableCell align="center">健康率</TableCell>
                <TableCell align="center">策略</TableCell>
                {sortCell('status', '状态', 'center')}
                <TableCell align="center">操作</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={8} align="center" sx={{ py: 4 }}>
                    <CircularProgress size={32} />
                  </TableCell>
                </TableRow>
              ) : filteredData.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} align="center" sx={{ py: 4, color: 'text.secondary' }}>
                    {search ? '未找到匹配的集群' : '暂无集群，点击「新建集群」添加'}
                  </TableCell>
                </TableRow>
              ) : (
                filteredData.map((cluster) => {
                  const itemSelected = isSelected(cluster.id)
                  return (
                    <TableRow key={cluster.id} hover selected={itemSelected}
                      sx={{ cursor: 'pointer' }}>
                      <TableCell padding="checkbox">
                        <Checkbox checked={itemSelected}
                          onChange={() => handleSelectOne(cluster.id)} />
                      </TableCell>
                      <TableCell>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                          <Typography sx={{ fontWeight: 500 }}>{cluster.name}</Typography>
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Stack direction="row" spacing={0.5}>
                          {cluster.region && <Chip label={cluster.region} size="small" variant="outlined" />}
                          {cluster.isp && <Chip label={cluster.isp} size="small" color="primary" variant="outlined" />}
                        </Stack>
                      </TableCell>
                      <TableCell align="center">
                        <Typography variant="body2">{cluster.node_count || '-'}</Typography>
                      </TableCell>
                      <TableCell align="center">
                        <Typography variant="body2" sx={{
                          fontWeight: 600,
                          color: cluster.health_rate >= 100 ? 'success.main' : cluster.health_rate >= 70 ? 'warning.main' : 'error.main',
                        }}>
                          {cluster.health_rate != null ? `${cluster.health_rate.toFixed(0)}%` : '-'}
                        </Typography>
                      </TableCell>
                      <TableCell align="center">
                        <Chip label={cluster.strategy === 'round-robin' ? '轮询' : cluster.strategy === 'weighted' ? '权重' : '随机'}
                          size="small" color="info" variant="outlined" />
                      </TableCell>
                      <TableCell align="center">
                        <StatusChip status={cluster.status} />
                      </TableCell>
                      <TableCell align="center">
                        <Stack direction="row" spacing={0.5} justifyContent="center">
                          <Tooltip title="查看详情">
                            <IconButton size="small" color="info"
                              onClick={() => handleOpenDrawer(cluster)}>
                              <VisibilityIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title="编辑">
                            <IconButton size="small" color="primary"
                              onClick={() => handleOpenDialog(cluster)}>
                              <EditIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title="删除">
                            <IconButton size="small" color="error"
                              onClick={() => handleDelete(cluster.id)}>
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

      {/* Create/Edit Dialog */}
      <Dialog open={dialogOpen} onClose={handleCloseDialog} maxWidth="sm" fullWidth>
        <DialogTitle>{editingCluster ? '编辑集群' : '新建集群'}</DialogTitle>
        <DialogContent sx={{ pt: 2 }}>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <TextField label="集群名称" value={form.name}
              onChange={handleFormChange('name')} required fullWidth
              placeholder="例如：华北-电信-A" />
            <TextField label="区域" select value={form.region}
              onChange={handleFormChange('region')} required fullWidth>
              {REGIONS.map(r => <MenuItem key={r} value={r}>{r}</MenuItem>)}
            </TextField>
            <TextField label="运营商" select value={form.isp}
              onChange={handleFormChange('isp')} required fullWidth>
              {ISPS.map(i => <MenuItem key={i} value={i}>{i}</MenuItem>)}
            </TextField>
            <TextField label="云厂商" select value={form.provider}
              onChange={handleFormChange('provider')} fullWidth>
              {PROVIDERS.map(p => (
                <MenuItem key={p} value={p}>{p || '未选择'}</MenuItem>
              ))}
            </TextField>
            <TextField label="调度策略" select value={form.strategy}
              onChange={handleFormChange('strategy')} fullWidth>
              {STRATEGIES.map(s => (
                <MenuItem key={s.value} value={s.value}>{s.label}</MenuItem>
              ))}
            </TextField>
            <TextField label="备注" value={form.description}
              onChange={handleFormChange('description')} fullWidth multiline rows={2}
              placeholder="集群用途说明" />
          </Stack>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button onClick={handleCloseDialog} disabled={saving}>取消</Button>
          <Button variant="contained" onClick={handleSave} disabled={saving}>
            {saving ? <CircularProgress size={18} sx={{ mr: 1 }} /> : null}
            {editingCluster ? '保存修改' : '创建集群'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Detail Drawer */}
      <Drawer anchor="right" open={drawerOpen} onClose={handleCloseDrawer}
        PaperProps={{ sx: { width: { xs: '100%', sm: 500 }, p: 0 } }}>
        {selectedCluster && (
          <Box>
            <Toolbar />
            <Box sx={{ px: 3, py: 2.5, borderBottom: '1px solid', borderColor: 'divider' }}>
              <Typography variant="h6" sx={{ fontWeight: 700 }}>{selectedCluster.name}</Typography>
              <Stack direction="row" spacing={1} sx={{ mt: 1 }}>
                <Chip label={selectedCluster.region} size="small" variant="outlined" />
                <Chip label={selectedCluster.isp} size="small" color="primary" variant="outlined" />
                <Chip label={selectedCluster.strategy} size="small" color="info" variant="outlined" />
                <StatusChip status={selectedCluster.status} />
              </Stack>
              {selectedCluster.description && (
                <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                  {selectedCluster.description}
                </Typography>
              )}
            </Box>

            <Box sx={{ px: 3, py: 2 }}>
              <Stack direction="row" spacing={0.5} sx={{ mb: 2 }}>
                <Button size="small" variant={drawerTab === 'nodes' ? 'contained' : 'outlined'}
                  onClick={() => setDrawerTab('nodes')} startIcon={<DnsIcon />}>成员节点</Button>
                <Button size="small" variant={drawerTab === 'rules' ? 'contained' : 'outlined'}
                  onClick={() => setDrawerTab('rules')} startIcon={<RouteIcon />}>关联规则</Button>
              </Stack>

              {drawerTab === 'nodes' ? (
                <DrawerNodesList clusterId={selectedCluster.id} />
              ) : (
                <DrawerRulesList clusterId={selectedCluster.id} />
              )}
            </Box>
          </Box>
        )}
      </Drawer>

      <Snackbar open={snackbar.open} autoHideDuration={3000}
        onClose={() => setSnackbar({ open: false, message: '' })} message={snackbar.message} />
    </Box>
  )
}

function DrawerNodesList({ clusterId }) {
  const [nodes, setNodes] = useState([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const load = async () => {
      try {
        const { data } = await clustersApi.listNodes(clusterId)
        setNodes(data || [])
      } catch { /* ignore */ }
      setLoading(false)
    }
    load()
  }, [clusterId])

  if (loading) return <CircularProgress size={24} />
  if (nodes.length === 0) return <Typography variant="body2" color="text.secondary">暂无节点</Typography>

  return (
    <List disablePadding>
      {nodes.map(node => (
        <ListItem key={node.id} divider sx={{ px: 0 }}>
          <ListItemAvatar sx={{ minWidth: 40 }}>
            <Avatar sx={{ width: 32, height: 32, bgcolor: node.status === 'active' ? 'success.light' : 'grey.300' }}>
              <DnsIcon fontSize="small" />
            </Avatar>
          </ListItemAvatar>
          <ListItemText
            primary={node.name}
            secondary={`${node.ip || '-'} | ${node.status === 'active' ? '活跃' : '离线'} | CPU: ${node.cpu_usage?.toFixed(1) || '-'}%`}
            primaryTypographyProps={{ fontSize: '0.9rem', fontWeight: 600 }}
            secondaryTypographyProps={{ fontSize: '0.8rem' }}
          />
          <Chip label={`${node.latency || '-'}ms`} size="small" variant="outlined"
            color={node.latency > 0 && node.latency < 100 ? 'success' : node.latency >= 300 ? 'error' : 'default'} />
        </ListItem>
      ))}
    </List>
  )
}

function DrawerRulesList({ clusterId }) {
  const [rules, setRules] = useState([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const load = async () => {
      try {
        const { data } = await clustersApi.listRules(clusterId)
        setRules(data || [])
      } catch { /* ignore */ }
      setLoading(false)
    }
    load()
  }, [clusterId])

  if (loading) return <CircularProgress size={24} />
  if (rules.length === 0) return <Typography variant="body2" color="text.secondary">暂无关联规则</Typography>

  return (
    <List disablePadding>
      {rules.map(rule => (
        <ListItem key={rule.id} divider sx={{ px: 0 }}>
          <ListItemAvatar sx={{ minWidth: 40 }}>
            <Avatar sx={{ width: 32, height: 32, bgcolor: 'primary.light' }}>
              <RouteIcon fontSize="small" />
            </Avatar>
          </ListItemAvatar>
          <ListItemText
            primary={rule.name || rule.domain}
            secondary={`${rule.domain || ''} | 命中 ${(rule.hit_count || 0).toLocaleString()}`}
            primaryTypographyProps={{ fontSize: '0.9rem', fontWeight: 600 }}
            secondaryTypographyProps={{ fontSize: '0.8rem' }}
          />
          <StatusChip status={rule.enabled ? 'active' : 'inactive'} />
        </ListItem>
      ))}
    </List>
  )
}

export default Clusters
