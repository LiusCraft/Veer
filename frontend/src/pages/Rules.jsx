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
  OutlinedInput,
  Select,
  FormControl,
  InputLabel,
  FormHelperText,
  Snackbar,
  InputAdornment,
  TableSortLabel,
  Checkbox,
  Toolbar,
  alpha,
} from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import EditIcon from '@mui/icons-material/Edit'
import DeleteIcon from '@mui/icons-material/Delete'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import RefreshIcon from '@mui/icons-material/Refresh'
import SearchIcon from '@mui/icons-material/Search'
import FileDownloadIcon from '@mui/icons-material/FileDownload'
import { rulesApi, nodesApi } from '../api/index.js'
import StatusChip from '../components/StatusChip.jsx'
import useTableSearch from '../hooks/useTableSearch.js'
import { exportToCSV } from '../utils/csv.js'

const EMPTY_FORM = {
  domain: '',
  description: '',
  strategy: 'round-robin',
  node_ids: '[]',
  origin_base_url: '',
}

/**
 * Rules page for managing redirect rules with node binding.
 */
function Rules() {
  const [rules, setRules] = useState([])
  const [nodes, setNodes] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingRule, setEditingRule] = useState(null)
  const [form, setForm] = useState(EMPTY_FORM)
  const [selectedNodeIds, setSelectedNodeIds] = useState([])
  const [saving, setSaving] = useState(false)
  const [snackbar, setSnackbar] = useState({ open: false, message: '' })
  const [selected, setSelected] = useState([])

  const { search, setSearch, sortField, sortDir, handleSort, filteredData } = useTableSearch(
    rules,
    ['domain', 'description', 'strategy'],
    'domain',
    'asc'
  )

  useEffect(() => {
    loadData()
  }, [])

  const loadData = async () => {
    setLoading(true)
    setError('')
    try {
      const [rulesRes, nodesRes] = await Promise.all([rulesApi.list(), nodesApi.list()])
      setRules(rulesRes.data || [])
      setNodes(nodesRes.data || [])
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const parseNodeIds = (nodeIdsStr) => {
    try {
      return JSON.parse(nodeIdsStr || '[]')
    } catch {
      return []
    }
  }

  const handleOpenDialog = (rule = null) => {
    if (rule) {
      setEditingRule(rule)
      const ids = parseNodeIds(rule.node_ids)
      setSelectedNodeIds(ids)
      setForm({
        domain: rule.domain || '',
        description: rule.description || '',
        strategy: rule.strategy,
        node_ids: rule.node_ids,
        origin_base_url: rule.origin_base_url || '',
      })
    } else {
      setEditingRule(null)
      setSelectedNodeIds([])
      setForm(EMPTY_FORM)
    }
    setDialogOpen(true)
  }

  const handleCloseDialog = () => {
    setDialogOpen(false)
    setEditingRule(null)
    setForm(EMPTY_FORM)
    setSelectedNodeIds([])
  }

  const handleFormChange = (field) => (e) => {
    setForm((prev) => ({ ...prev, [field]: e.target.value }))
  }

  const handleNodeSelectChange = (e) => {
    const value = e.target.value
    setSelectedNodeIds(typeof value === 'string' ? value.split(',').map(Number) : value)
  }

  const handleSave = async () => {
    if (!form.domain.trim()) {
      setError('调度域名不能为空')
      return
    }
    setSaving(true)
    setError('')
    try {
      const payload = {
        domain: form.domain.trim(),
        description: form.description,
        strategy: form.strategy,
        node_ids: JSON.stringify(selectedNodeIds),
        origin_base_url: form.origin_base_url.trim(),
      }
      if (editingRule) {
        await rulesApi.update(editingRule.id, payload)
      } else {
        await rulesApi.create(payload)
      }
      handleCloseDialog()
      loadData()
    } catch (err) {
      setError(err.message)
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id) => {
    if (!window.confirm('确定要删除该规则吗？')) return
    try {
      await rulesApi.delete(id)
      loadData()
    } catch (err) {
      setError(err.message)
    }
  }

  const handleCopyLink = (rule) => {
    if (!rule.domain) return
    const link = `${window.location.protocol}//${rule.domain}`
    navigator.clipboard.writeText(link).then(() => {
      setSnackbar({ open: true, message: `已复制调度域名: ${link}` })
    })
  }

  const getNodeNames = (nodeIdsStr) => {
    const ids = parseNodeIds(nodeIdsStr)
    return ids
      .map((id) => nodes.find((n) => n.id === id)?.name || `Node#${id}`)
      .join(', ')
  }

  // Batch selection handlers
  const handleSelectAll = (e) => {
    if (e.target.checked) {
      setSelected(filteredData.map((r) => r.id))
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
    if (!window.confirm(`确定要删除选中的 ${selected.length} 条规则吗？`)) return
    try {
      await rulesApi.batchDelete(selected)
      setSelected([])
      setSnackbar({ open: true, message: `成功删除 ${selected.length} 条规则` })
      loadData()
    } catch (err) {
      setError(err.message)
    }
  }

  const handleExportCSV = () => {
    const today = new Date().toISOString().slice(0, 10)
    const filename = `rules-${today}.csv`
    const columns = [
      { key: 'domain', label: '域名' },
      { key: 'description', label: '描述' },
      { key: 'origin_base_url', label: '回源地址' },
      { key: 'strategy', label: '路由策略' },
      { key: 'hit_count', label: '命中次数' },
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

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 3 }}>
        <Typography variant="h5" sx={{ fontWeight: 600 }}>
          跳转规则管理
        </Typography>
        <Stack direction="row" spacing={1}>
          <Button
            variant="outlined"
            startIcon={<RefreshIcon />}
            onClick={loadData}
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
            新增规则
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
          placeholder="搜索域名、描述、策略..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">
                <SearchIcon fontSize="small" />
              </InputAdornment>
            ),
          }}
          sx={{ width: 360 }}
        />
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
            已选中 {selected.length} 条规则
          </Typography>
          <Button
            variant="outlined"
            color="error"
            size="small"
            startIcon={<DeleteIcon />}
            onClick={handleBatchDelete}
          >
            批量删除
          </Button>
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
                {sortCell('domain', '域名')}
                <TableCell>描述</TableCell>
                {sortCell('strategy', '路由策略', 'center')}
                <TableCell>绑定节点</TableCell>
                {sortCell('hit_count', '命中次数', 'center')}
                <TableCell>调度域名</TableCell>
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
                    {search ? '未找到匹配的规则' : '暂无规则，点击「新增规则」添加'}
                  </TableCell>
                </TableRow>
              ) : (
                filteredData.map((rule) => {
                  const isItemSelected = isSelected(rule.id)
                  return (
                    <TableRow
                      key={rule.id}
                      hover
                      selected={isItemSelected}
                      sx={{ cursor: 'pointer' }}
                    >
                      <TableCell padding="checkbox">
                        <Checkbox
                          checked={isItemSelected}
                          onChange={() => handleSelectOne(rule.id)}
                        />
                      </TableCell>
                      <TableCell>
                        {rule.domain ? (
                          <Chip label={rule.domain} size="small" variant="outlined" color="secondary" />
                        ) : (
                          <Typography variant="body2" color="text.disabled">通用</Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" color="text.secondary">
                          {rule.description || '-'}
                        </Typography>
                      </TableCell>
                      <TableCell align="center">
                        <StatusChip status={rule.strategy} />
                      </TableCell>
                      <TableCell>
                        <Typography
                          variant="body2"
                          sx={{ maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
                        >
                          {getNodeNames(rule.node_ids) || '-'}
                        </Typography>
                      </TableCell>
                      <TableCell align="center">
                        <Chip label={rule.hit_count || 0} size="small" color="default" />
                      </TableCell>
                      <TableCell>
                        <Typography
                          variant="caption"
                          sx={{ fontFamily: 'monospace', color: 'text.secondary' }}
                        >
                          {rule.domain || '-'}
                        </Typography>
                      </TableCell>
                      <TableCell align="center">
                        <Stack direction="row" spacing={0.5} justifyContent="center">
                          <Tooltip title="复制调度域名">
                            <IconButton size="small" color="default" onClick={() => handleCopyLink(rule)}>
                              <ContentCopyIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title="编辑">
                            <IconButton size="small" color="primary" onClick={() => handleOpenDialog(rule)}>
                              <EditIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title="删除">
                            <IconButton size="small" color="error" onClick={() => handleDelete(rule.id)}>
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
        <DialogTitle>{editingRule ? '编辑规则' : '新增跳转规则'}</DialogTitle>
        <DialogContent sx={{ pt: 2 }}>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <TextField
              label="域名"
              value={form.domain}
              onChange={handleFormChange('domain')}
              required
              fullWidth
              placeholder="例如：cdn.example.com"
              helperText="调度器通过此域名匹配请求（Host 头），域名须唯一"
            />
            <TextField
              label="描述"
              value={form.description}
              onChange={handleFormChange('description')}
              fullWidth
              placeholder="规则用途说明"
            />
            <TextField
              label="路由策略"
              select
              value={form.strategy}
              onChange={handleFormChange('strategy')}
              fullWidth
            >
              <MenuItem value="round-robin">轮询 (Round-Robin)</MenuItem>
              <MenuItem value="weighted">权重 (Weighted)</MenuItem>
              <MenuItem value="random">随机 (Random)</MenuItem>
            </TextField>
            <FormControl fullWidth>
              <InputLabel>绑定节点</InputLabel>
              <Select
                multiple
                value={selectedNodeIds}
                onChange={handleNodeSelectChange}
                input={<OutlinedInput label="绑定节点" />}
                renderValue={(selected) =>
                  selected
                    .map((id) => nodes.find((n) => n.id === id)?.name || `Node#${id}`)
                    .join(', ')
                }
              >
                {nodes.map((node) => (
                  <MenuItem key={node.id} value={node.id}>
                    {node.name}
                    {node.status === 'inactive' && (
                      <Chip label="停用" size="small" sx={{ ml: 1 }} />
                    )}
                  </MenuItem>
                ))}
              </Select>
              <FormHelperText>可多选，将按策略负载均衡到所选节点</FormHelperText>
            </FormControl>
            <TextField
              label="回源地址"
              value={form.origin_base_url}
              onChange={handleFormChange('origin_base_url')}
              fullWidth
              placeholder="例如：http://origin-server:80"
              helperText="可选。Edge 节点缓存未命中时从此地址拉取，留空则使用节点默认回源"
            />
          </Stack>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button onClick={handleCloseDialog} disabled={saving}>
            取消
          </Button>
          <Button variant="contained" onClick={handleSave} disabled={saving}>
            {saving ? <CircularProgress size={18} sx={{ mr: 1 }} /> : null}
            {editingRule ? '保存修改' : '新增规则'}
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar
        open={snackbar.open}
        autoHideDuration={3000}
        onClose={() => setSnackbar({ open: false, message: '' })}
        message={snackbar.message}
      />
    </Box>
  )
}

export default Rules
