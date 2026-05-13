import { useState, useEffect, useCallback } from 'react'
import {
  Box, Typography, Card, Button, IconButton, Tooltip,
  TextField, MenuItem, Alert, CircularProgress, Chip, Stack,
  OutlinedInput, Select, FormControl, InputLabel, FormHelperText,
  Snackbar, Switch, Drawer, Checkbox, Divider, alpha, Avatar, Collapse,
} from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import EditIcon from '@mui/icons-material/Edit'
import DeleteIcon from '@mui/icons-material/Delete'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import RefreshIcon from '@mui/icons-material/Refresh'
import SearchIcon from '@mui/icons-material/Search'
import ArrowDownwardIcon from '@mui/icons-material/ArrowDownward'
import AltRouteIcon from '@mui/icons-material/AltRoute'
import HttpIcon from '@mui/icons-material/Http'
import DragHandleIcon from '@mui/icons-material/DragHandle'
import SelectAllIcon from '@mui/icons-material/SelectAll'
import CloseIcon from '@mui/icons-material/Close'
import ArrowForwardIcon from '@mui/icons-material/ArrowForward'
import PublicIcon from '@mui/icons-material/Public'
import MemoryIcon from '@mui/icons-material/Memory'
import BlockIcon from '@mui/icons-material/Block'
import { rulesApi, nodesApi, clustersApi } from '../api/index.js'
import { exportToCSV } from '../utils/csv.js'

const MATCH_TYPE_LABELS = { prefix: '前缀匹配', exact: '精确匹配', regex: '正则匹配' }
const STRATEGY_LABELS = { 'round-robin': '轮询', weighted: '权重', random: '随机' }
const STRATEGY_COLORS = { 'round-robin': 'info', weighted: 'warning', random: 'success' }
const RULE_TYPE_META = {
  domain_routing: { label: '域名路由', icon: <AltRouteIcon sx={{ fontSize: 15 }} />, color: 'primary' },
  url_redirect: { label: 'URL 重定向', icon: <HttpIcon sx={{ fontSize: 15 }} />, color: 'info' },
}

const EMPTY_ROUTING_FORM = {
  name: '', domain: '', description: '', strategy: 'round-robin',
  node_ids: '[]', origin_base_url: '', enabled: true,
  cache_ttl_seconds: null, cache_control_override: '', bypass_cache: false,
}
const EMPTY_REDIRECT_FORM = {
  name: '', domain: '', description: '',
  match_type: 'prefix', source_path: '/', target_host: '',
  target_path: '', redirect_code: 302, enabled: true,
}

function RuleCard({ rule, nodes, onEdit, onToggle, isSelectMode, isSelected, onSelect }) {
  const isRouting = rule.rule_type === 'domain_routing'
  const meta = RULE_TYPE_META[rule.rule_type] || RULE_TYPE_META.domain_routing

  return (
    <Card
      elevation={0}
      sx={{
        display: 'flex', alignItems: 'stretch',
        border: '1px solid',
        borderColor: isSelected ? 'primary.main' : theme => theme.palette.mode === 'dark' ? 'rgba(255,255,255,0.08)' : 'grey.200',
        borderRadius: 2,
        bgcolor: isSelected ? theme => alpha(theme.palette.primary.main, 0.04) : 'background.paper',
        transition: 'all 0.15s ease',
        '&:hover': {
          borderColor: isSelected ? 'primary.main' : 'primary.light',
          boxShadow: theme => `0 2px 8px ${alpha(theme.palette.common.black, 0.08)}`,
        },
      }}
    >
      {isSelectMode && (
        <Box sx={{ display: 'flex', alignItems: 'center', pl: 1.5 }}>
          <Checkbox checked={isSelected} onChange={() => onSelect(rule.id)} size="small" />
        </Box>
      )}

      <Box sx={{ display: 'flex', alignItems: 'center', pl: 0.5, pr: 0.5, cursor: 'grab', color: 'text.disabled',
        '&:hover': { color: 'text.secondary' } }}>
        <DragHandleIcon fontSize="small" />
      </Box>

      <Box sx={{ display: 'flex', alignItems: 'center', pl: 0.5, pr: 0.5 }}>
        <Tooltip title={meta.label}>
          <Avatar sx={{ width: 28, height: 28, bgcolor: theme => alpha(theme.palette[meta.color].main, 0.1), color: `${meta.color}.main` }}>
            {meta.icon}
          </Avatar>
        </Tooltip>
      </Box>

      <Box sx={{ flexGrow: 1, p: 1.5, pl: 0.5 }}>
        <Stack spacing={0.5}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Typography variant="subtitle2" sx={{ fontWeight: 700, fontSize: '0.9rem' }}>
              {rule.name || (isRouting ? rule.domain : (rule.source_path || rule.domain))}
            </Typography>
            <Chip label={meta.label} size="small" variant="outlined"
              color={meta.color} sx={{ height: 20, fontSize: 10 }} />
            {!rule.enabled && (
              <Chip label="已停用" size="small" sx={{ height: 20, fontSize: 10 }} />
            )}
            <Box sx={{ flexGrow: 1 }} />
            <Typography variant="caption" color="text.disabled" sx={{ fontSize: 11, whiteSpace: 'nowrap' }}>
              命中 {(rule.hit_count || 0).toLocaleString()}
            </Typography>
          </Box>

          {isRouting ? (
            <RoutingRulePreview rule={rule} clusters={clusters} nodes={nodes} />
          ) : (
            <UrlRedirectRulePreview rule={rule} />
          )}
        </Stack>
      </Box>


      <Box sx={{ display: 'flex', alignItems: 'center', px: 0.5 }}>
        <Tooltip title={rule.enabled ? '停用' : '启用'}>
          <Switch checked={rule.enabled} onChange={() => onToggle(rule)} size="small"
            sx={{ '& .MuiSwitch-thumb': { boxShadow: 'none' } }} />
        </Tooltip>
      </Box>

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, pr: 1.5, pl: 0.5 }}>
        <Tooltip title="编辑">
          <IconButton size="small" color="primary" onClick={() => onEdit(rule)} sx={{ p: 0.5 }}>
            <EditIcon fontSize="small" />
          </IconButton>
        </Tooltip>
        {isRouting && (
          <Tooltip title="复制域名">
            <IconButton size="small" onClick={() => {
              navigator.clipboard.writeText(`${window.location.protocol}//${rule.domain}`)
            }} sx={{ p: 0.5 }}>
              <ContentCopyIcon fontSize="small" sx={{ fontSize: 14 }} />
            </IconButton>
          </Tooltip>
        )}
      </Box>
    </Card>
  )
}

function RoutingRulePreview({ rule, clusters: allClusters }) {
  const clusterNames = (rule.clusters || []).map(c => {
    const cl = allClusters.find(cl => cl.id === c.cluster_id)
    return cl?.name || `Cluster#${c.cluster_id}`
  })
  const displayNames = clusterNames.length > 0
    ? clusterNames.join(', ')
    : (() => {
        try {
          const ids = JSON.parse(rule.node_ids || '[]')
          return ids.map(id => `Node#${id}`).join(', ') || '-'
        } catch { return '-' }
      })()

  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, bgcolor: theme =>
      theme.palette.mode === 'dark' ? 'rgba(255,255,255,0.03)' : 'grey.50',
      borderRadius: 1.5, p: 1, mt: 0.3 }}>
      <Box>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, flexWrap: 'wrap' }}>
          <Chip icon={<PublicIcon />} label={rule.domain} size="small"
            color="secondary" variant="outlined" sx={{ height: 20, '& .MuiChip-icon': { fontSize: 11 } }} />
          <Chip label={STRATEGY_LABELS[rule.strategy] || rule.strategy} size="small"
            color={STRATEGY_COLORS[rule.strategy] || 'default'} sx={{ height: 20, fontSize: 10, fontWeight: 600 }} />
          {rule.bypass_cache && (
            <Chip icon={<BlockIcon />} label="跳过缓存" size="small"
              color="error" variant="outlined" sx={{ height: 20, fontSize: 10, '& .MuiChip-icon': { fontSize: 11 } }} />
          )}
        </Box>
      </Box>
      <ArrowForwardIcon sx={{ fontSize: 16, color: 'primary.main', flexShrink: 0 }} />
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography variant="body2" sx={{
          fontFamily: 'monospace', fontSize: 12, fontWeight: 600,
          overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
        }}>
          {displayNames}
        </Typography>
        {rule.cache_ttl_seconds != null && rule.cache_ttl_seconds > 0 && (
          <Typography variant="caption" color="success.main" sx={{ fontSize: 10, fontWeight: 600 }}>
            TTL {rule.cache_ttl_seconds}s
          </Typography>
        )}
      </Box>
    </Box>
  )
}

function UrlRedirectRulePreview({ rule }) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, bgcolor: theme =>
      theme.palette.mode === 'dark' ? 'rgba(255,255,255,0.03)' : 'grey.50',
      borderRadius: 1.5, p: 1, mt: 0.3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, flexWrap: 'wrap' }}>
        {rule.domain && <Chip label={rule.domain} size="small" variant="outlined" sx={{ height: 20, fontSize: 10 }} />}
        <Chip label={MATCH_TYPE_LABELS[rule.match_type] || rule.match_type} size="small" sx={{ height: 20, fontSize: 10 }} />
        <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: 12, fontWeight: 600 }}>{rule.source_path}</Typography>
        <Chip label={rule.redirect_code} size="small"
          color={rule.redirect_code === 301 ? 'warning' : 'info'}
          sx={{ height: 18, fontSize: 9, fontWeight: 700 }} />
      </Box>
      <ArrowForwardIcon sx={{ fontSize: 16, color: rule.redirect_code === 301 ? 'warning.main' : 'info.main', flexShrink: 0 }} />
      <Typography variant="body2" sx={{
        fontFamily: 'monospace', fontSize: 12, fontWeight: 600,
        overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', minWidth: 0,
      }}>
        {(rule.target_host || '(当前域名)') + (rule.target_path || rule.source_path)}
      </Typography>
    </Box>
  )
}

function FormSection({ title, optional, children }) {
  return (
    <Box>
      <Typography variant="caption" sx={{ display: 'block', mb: 1, fontWeight: 700, color: 'text.secondary',
        textTransform: 'uppercase', letterSpacing: '0.5px', fontSize: 10 }}>
        {title}
        {optional && <Typography component="span" variant="caption" color="text.disabled"> (可选)</Typography>}
      </Typography>
      <Stack spacing={2}>{children}</Stack>
    </Box>
  )
}

function Rules() {
  const [rules, setRules] = useState([])
  const [nodes, setNodes] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editingRule, setEditingRule] = useState(null)
  const [formType, setFormType] = useState('domain_routing')
  const [routingForm, setRoutingForm] = useState(EMPTY_ROUTING_FORM)
  const [redirectForm, setRedirectForm] = useState(EMPTY_REDIRECT_FORM)
  const [selectedNodeIds, setSelectedNodeIds] = useState([])
  const [clusters, setClusters] = useState([])
  const [selectedClusterBindings, setSelectedClusterBindings] = useState([])
  const [saving, setSaving] = useState(false)
  const [snackbar, setSnackbar] = useState({ open: false, message: '' })
  const [search, setSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const [selectMode, setSelectMode] = useState(false)
  const [selected, setSelected] = useState([])

  const loadData = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const [rulesRes, nodesRes, clustersRes] = await Promise.all([
        rulesApi.list(),
        nodesApi.list(),
        clustersApi.list(),
      ])
      const sorted = (rulesRes.data || []).sort((a, b) => (a.priority || 0) - (b.priority || 0))
      setRules(sorted)
      setNodes(nodesRes.data || [])
      setClusters(clustersRes.data || [])
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { loadData() }, [loadData])
  useEffect(() => { setSelected([]); setSelectMode(false) }, [])

  const parseNodeIds = (str) => { try { return JSON.parse(str || '[]') } catch { return [] } }

  const isRoutingForm = formType === 'domain_routing'

  const filteredData = rules.filter(r => {
    const matchType = !typeFilter || r.rule_type === typeFilter
    if (!search.trim()) return matchType
    const kw = search.toLowerCase()
    return matchType && [r.name, r.domain, r.description, r.source_path, r.target_host, r.target_path, RULE_TYPE_META[r.rule_type]?.label]
      .some(v => v && v.toLowerCase().includes(kw))
  })

  const handleOpenDrawer = (rule = null) => {
    setEditingRule(rule)
    if (rule) {
      setFormType(rule.rule_type || 'domain_routing')
      if (rule.rule_type === 'url_redirect') {
        setRedirectForm({ ...EMPTY_REDIRECT_FORM, ...rule, enabled: rule.enabled })
      } else {
        setSelectedNodeIds(parseNodeIds(rule.node_ids))
        setSelectedClusterBindings(rule.clusters || [])
        setRoutingForm({ ...EMPTY_ROUTING_FORM, ...rule, enabled: rule.enabled, node_ids: rule.node_ids || '[]' })
      }
    } else {
      setFormType('domain_routing')
      setSelectedNodeIds([])
      setSelectedClusterBindings([])
      setRoutingForm(EMPTY_ROUTING_FORM)
      setRedirectForm(EMPTY_REDIRECT_FORM)
    }
    setDrawerOpen(true)
  }

  const handleCloseDrawer = () => {
    setDrawerOpen(false)
    setEditingRule(null)
    setRoutingForm(EMPTY_ROUTING_FORM)
    setRedirectForm(EMPTY_REDIRECT_FORM)
    setSelectedNodeIds([])
    setSelectedClusterBindings([])
  }

  const handleRoutingChange = (f) => (e) => setRoutingForm(p => ({ ...p, [f]: e.target.value }))
  const handleRedirectChange = (f) => (e) => setRedirectForm(p => ({ ...p, [f]: e.target.value }))

  const handleNodeSelectChange = (e) => {
    const v = e.target.value
    setSelectedNodeIds(typeof v === 'string' ? v.split(',').map(Number) : v)
  }

  const handleSave = async () => {
    const form = isRoutingForm ? routingForm : redirectForm
    if (!form.domain?.trim()) { setError('域名字段不能为空'); return }
    if (isRoutingForm && selectedClusterBindings.length === 0 && selectedNodeIds.length === 0) {
      setError('请至少选择一个集群或节点'); return
    }

    setSaving(true)
    setError('')
    const ruleType = editingRule?.rule_type || formType
    try {
      const payload = ruleType === 'url_redirect'
        ? { ...redirectForm, domain: redirectForm.domain.trim(), rule_type: 'url_redirect' }
        : {
            ...routingForm,
            domain: routingForm.domain.trim(),
            node_ids: selectedNodeIds.length > 0 ? JSON.stringify(selectedNodeIds) : '[]',
            clusters: selectedClusterBindings.length > 0 ? selectedClusterBindings : undefined,
          }

      if (editingRule) {
        await rulesApi.update(editingRule.id, payload)
      } else {
        await rulesApi.create({ ...payload, rule_type: formType })
      }
      handleCloseDrawer()
      loadData()
    } catch (err) {
      setError(err.message)
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id) => {
    if (!window.confirm('确定删除该规则？')) return
    try {
      await rulesApi.delete(id)
      setSnackbar({ open: true, message: '规则已删除' })
      loadData()
    } catch (err) { setError(err.message) }
  }

  const handleDeleteFromDrawer = async () => {
    if (!editingRule) return
    if (!window.confirm(`确定删除规则「${editingRule.name || editingRule.domain}」？`)) return
    try {
      await rulesApi.delete(editingRule.id)
      handleCloseDrawer()
      setSnackbar({ open: true, message: '规则已删除' })
      loadData()
    } catch (err) { setError(err.message) }
  }

  const handleToggle = async (rule) => {
    try {
      await rulesApi.batchToggle([rule.id], !rule.enabled)
      setSnackbar({ open: true, message: rule.enabled ? '规则已停用' : '规则已启用' })
      loadData()
    } catch (err) { setError(err.message) }
  }

  const handleSelectAll = () => {
    if (selected.length === filteredData.length) setSelected([])
    else setSelected(filteredData.map(r => r.id))
  }

  const handleBatchDelete = async () => {
    if (!selected.length || !window.confirm(`确定删除选中的 ${selected.length} 条规则？`)) return
    try {
      await rulesApi.batchDelete(selected)
      setSelected([]); setSelectMode(false)
      setSnackbar({ open: true, message: `已删除 ${selected.length} 条规则` })
      loadData()
    } catch (err) { setError(err.message) }
  }

  const handleExportCSV = () => {
    const columns = [
      { key: 'rule_type', label: '类型' },
      { key: 'name', label: '名称' },
      { key: 'domain', label: '域名' },
      { key: 'description', label: '描述' },
      { key: 'hit_count', label: '命中次数' },
      { key: 'enabled', label: '启用' },
    ]
    exportToCSV(filteredData, `跳转规则.csv`, columns)
  }

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
        <Typography variant="h5" sx={{ fontWeight: 700 }}>域名管理</Typography>
        <Stack direction="row" spacing={1}>
          <Button variant="outlined" size="small" startIcon={<RefreshIcon />}
            onClick={loadData} disabled={loading}>刷新</Button>
          <Button variant="outlined" size="small" startIcon={<SelectAllIcon />}
            onClick={() => { setSelectMode(!selectMode); setSelected([]) }}
            color={selectMode ? 'primary' : 'inherit'}>多选</Button>
          <Button variant="contained" size="small" startIcon={<AddIcon />}
            onClick={() => handleOpenDrawer()}>新增规则</Button>
        </Stack>
      </Box>

      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>{error}</Alert>}

      <Card elevation={0} sx={{ p: 1.5, mb: 2, display: 'flex', alignItems: 'center', gap: 1.5,
        border: '1px solid', borderColor: theme => theme.palette.mode === 'dark' ? 'rgba(255,255,255,0.08)' : 'grey.200' }}>
        <SearchIcon sx={{ color: 'text.disabled', fontSize: 20 }} />
        <TextField size="small" variant="standard"
          placeholder="搜索规则名称、域名、类型、描述..."
          value={search} onChange={(e) => setSearch(e.target.value)}
          sx={{ flexGrow: 1 }} InputProps={{ disableUnderline: true }} />
        {search && (
          <IconButton size="small" onClick={() => setSearch('')}><CloseIcon fontSize="small" /></IconButton>
        )}
        <Box sx={{ display: 'flex', gap: 0.5 }}>
          {[{ key: '', label: '全部' }, ...Object.entries(RULE_TYPE_META).map(([k, v]) => ({ key: k, label: v.label }))].map(f => (
            <Chip key={f.key}
              label={f.label}
              size="small"
              variant={typeFilter === f.key || (f.key === '' && typeFilter === '') ? 'filled' : 'outlined'}
              color={typeFilter === f.key || (f.key === '' && typeFilter === '') ? 'primary' : 'default'}
              onClick={() => setTypeFilter(f.key ? f.key : '')}
              sx={{ height: 24, fontSize: 11, cursor: 'pointer' }}
            />
          ))}
        </Box>
        <Typography variant="caption" color="text.disabled" sx={{ whiteSpace: 'nowrap' }}>
          {filteredData.length} / {rules.length} 条
        </Typography>
        <Tooltip title="导出 CSV">
          <span>
            <IconButton size="small" onClick={handleExportCSV} disabled={!filteredData.length}>
              <ArrowDownwardIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
      </Card>

      <Collapse in={selectMode && selected.length > 0}>
        <Card elevation={0} sx={{ p: 1.5, mb: 2, display: 'flex', alignItems: 'center', gap: 1.5,
          bgcolor: theme => alpha(theme.palette.primary.main, 0.06),
          border: '1px solid', borderColor: 'primary.light' }}>
          <Checkbox checked={selected.length === filteredData.length && filteredData.length > 0}
            indeterminate={selected.length > 0 && selected.length < filteredData.length}
            onChange={handleSelectAll} size="small" />
          <Typography variant="body2" sx={{ fontWeight: 600, flexGrow: 1 }}>
            已选择 {selected.length} 条规则
          </Typography>
          <Button variant="outlined" color="error" size="small" startIcon={<DeleteIcon />}
            onClick={handleBatchDelete}>批量删除</Button>
          <Button size="small" onClick={() => setSelectMode(false)}>完成</Button>
        </Card>
      </Collapse>

      {loading ? (
        <Box sx={{ textAlign: 'center', py: 8 }}><CircularProgress size={32} /></Box>
      ) : filteredData.length === 0 ? (
        <Box sx={{ textAlign: 'center', py: 8, color: 'text.secondary' }}>
          <Avatar sx={{ mx: 'auto', mb: 2, width: 56, height: 56, bgcolor: theme =>
            alpha(theme.palette.primary.main, 0.1), color: 'primary.main' }}>
            <AltRouteIcon />
          </Avatar>
          <Typography variant="h6" sx={{ fontWeight: 600, mb: 0.5, color: 'text.primary' }}>
            {search ? '未找到匹配的规则' : '暂无规则'}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            {search ? '尝试其他搜索关键词' : '创建域名路由或 URL 重定向规则来控制流量'}
          </Typography>
          {!search && (
            <Button variant="contained" startIcon={<AddIcon />} onClick={() => handleOpenDrawer()}>
              新增规则
            </Button>
          )}
        </Box>
      ) : (
        <Stack spacing={1}>
          {filteredData.map(rule => (
            <RuleCard key={rule.id} rule={rule} nodes={nodes}
              onEdit={handleOpenDrawer} onToggle={handleToggle}
              isSelectMode={selectMode} isSelected={selected.includes(rule.id)}
              onSelect={(id) => setSelected(p => p.includes(id) ? p.filter(s => s !== id) : [...p, id])} />
          ))}
        </Stack>
      )}

      <Drawer anchor="right" open={drawerOpen} onClose={handleCloseDrawer}
        PaperProps={{ sx: { width: { xs: '100%', sm: 520 }, p: 0 } }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: 3, py: 2,
          borderBottom: '1px solid', borderColor: 'divider' }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
            <Typography variant="h6" sx={{ fontWeight: 700, fontSize: 16 }}>
              {editingRule ? '编辑规则' : '新增规则'}
            </Typography>
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Typography variant="caption" color="text.secondary" sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              <Switch size="small" checked={isRoutingForm ? routingForm.enabled : redirectForm.enabled}
                onChange={() => isRoutingForm
                  ? setRoutingForm(p => ({ ...p, enabled: !p.enabled }))
                  : setRedirectForm(p => ({ ...p, enabled: !p.enabled }))} />
              {isRoutingForm ? (routingForm.enabled ? '启用' : '停用') : (redirectForm.enabled ? '启用' : '停用')}
            </Typography>
            <IconButton onClick={handleCloseDrawer} size="small"><CloseIcon /></IconButton>
          </Box>
        </Box>

        {!editingRule && (
          <Box sx={{ px: 3, pt: 2.5, pb: 1 }}>
            <Typography variant="caption" sx={{ display: 'block', mb: 1, fontWeight: 700, color: 'text.secondary',
              textTransform: 'uppercase', letterSpacing: '0.5px', fontSize: 10 }}>规则类型</Typography>
            <Stack direction="row" spacing={1}>
              {Object.entries(RULE_TYPE_META).map(([k, v]) => (
                <Card key={k} elevation={0}
                  onClick={() => setFormType(k)}
                  sx={{
                    flex: 1, p: 1.5, borderRadius: 2, cursor: 'pointer',
                    border: '2px solid',
                    borderColor: formType === k ? `${v.color}.main` : 'divider',
                    bgcolor: formType === k ? theme => alpha(theme.palette[v.color].main, 0.04) : 'background.paper',
                    transition: 'all 0.15s',
                    '&:hover': { borderColor: formType === k ? `${v.color}.main` : 'text.disabled' },
                  }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                    <Avatar sx={{ width: 24, height: 24, bgcolor: theme =>
                      alpha(theme.palette[v.color].main, 0.1), color: `${v.color}.main` }}>{v.icon}</Avatar>
                    <Typography variant="body2" sx={{ fontWeight: 700 }}>{v.label}</Typography>
                  </Box>
                  <Typography variant="caption" color="text.secondary">
                    {k === 'domain_routing' ? '域名匹配后 302 到 Edge 节点' : '路径匹配后 301/302 到目标 URL'}
                  </Typography>
                </Card>
              ))}
            </Stack>
          </Box>
        )}

        <Box sx={{ flexGrow: 1, overflow: 'auto', px: 3, py: 2.5 }}>
          {isRoutingForm ? (
            <Stack spacing={3}>
              <FormSection title="基本信息">
                <TextField label="规则名称" value={routingForm.name}
                  onChange={handleRoutingChange('name')} fullWidth placeholder="例如：静态资源分发" size="small" />
                <TextField label="域名" value={routingForm.domain}
                  onChange={handleRoutingChange('domain')} required fullWidth
                  placeholder="例如：cdn.veer.local" size="small"
                  helperText="调度器通过此域名匹配请求" />
                <TextField label="描述" value={routingForm.description}
                  onChange={handleRoutingChange('description')} fullWidth placeholder="规则用途说明" size="small" multiline rows={2} />
              </FormSection>
              <Divider />
              <FormSection title="路由配置">
                <TextField label="路由策略" select value={routingForm.strategy}
                  onChange={handleRoutingChange('strategy')} fullWidth size="small">
                  <MenuItem value="round-robin">轮询 — 依次将请求分发给各节点</MenuItem>
                  <MenuItem value="weighted">权重 — 按节点权重比例分发</MenuItem>
                  <MenuItem value="random">随机 — 随机选择一个节点</MenuItem>
                </TextField>
                <FormControl fullWidth size="small">
                  <InputLabel>绑定节点</InputLabel>
                  <Select multiple value={selectedNodeIds} onChange={handleNodeSelectChange}
                    input={<OutlinedInput label="绑定节点" />}
                    renderValue={(sel) => sel.map(id => nodes.find(n => n.id === id)?.name || `#${id}`).join(', ')}>
                    {nodes.map(node => (
                      <MenuItem key={node.id} value={node.id}>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, width: '100%' }}>
                          <span>{node.name}</span>
                          <Box sx={{ flexGrow: 1 }} />
                          <Chip label={node.region || '未知区域'} size="small" variant="outlined" sx={{ height: 18, fontSize: 10 }} />
                          {node.status === 'inactive' && <Chip label="停用" size="small" color="error" sx={{ height: 18, fontSize: 10 }} />}
                        </Box>
                      </MenuItem>
                    ))}
                  </Select>
                  <FormHelperText>按策略将请求均衡分发到所选节点</FormHelperText>
                </FormControl>
                <FormControl fullWidth size="small">
                  <InputLabel>关联集群</InputLabel>
                  <Select multiple value={selectedClusterBindings.map(b => b.cluster_id)}
                    onChange={(e) => {
                      const ids = typeof e.target.value === 'string' ? e.target.value.split(',').map(Number) : e.target.value
                      setSelectedClusterBindings(ids.map(id => ({
                        cluster_id: id, weight: 1, priority: 0,
                      })))
                    }}
                    input={<OutlinedInput label="关联集群" />}
                    renderValue={(sel) => sel.map(id => {
                      const cl = clusters.find(c => c.id === id)
                      return cl?.name || `#${id}`
                    }).join(', ')}>
                    {clusters.map(cl => (
                      <MenuItem key={cl.id} value={cl.id}>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, width: '100%' }}>
                          <span>{cl.name}</span>
                          <Box sx={{ flexGrow: 1 }} />
                          <Chip label={cl.region || ''} size="small" variant="outlined" sx={{ height: 18, fontSize: 10 }} />
                        </Box>
                      </MenuItem>
                    ))}
                  </Select>
                  <FormHelperText>关联集群后，规则将调度到集群下的活跃节点（如同时选择了节点，两者均生效）</FormHelperText>
                </FormControl>
              </FormSection>
              <Divider />
              <FormSection title="回源配置" optional>
                <TextField label="回源地址" value={routingForm.origin_base_url}
                  onChange={handleRoutingChange('origin_base_url')} fullWidth size="small"
                  placeholder="例如：http://origin-server:80"
                  helperText="Edge 节点缓存未命中时从此地址拉取" />
              </FormSection>
              <Divider />
              <FormSection title="缓存配置" optional>
                <TextField label="缓存 TTL（秒）" type="number" value={routingForm.cache_ttl_seconds ?? ''}
                  onChange={e => setRoutingForm(p => ({ ...p, cache_ttl_seconds: e.target.value === '' ? null : Number(e.target.value) }))}
                  fullWidth size="small" placeholder="留空使用全局默认值"
                  helperText="覆盖此域名的全局缓存 TTL。设为 0 表示不缓存" inputProps={{ min: 0 }} />
                <TextField label="Cache-Control 覆盖" value={routingForm.cache_control_override}
                  onChange={handleRoutingChange('cache_control_override')} fullWidth size="small"
                  placeholder="例如：public, max-age=3600"
                  helperText="强制改写源站的 Cache-Control 响应头，留空则使用源站值" />
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
                  <Switch checked={routingForm.bypass_cache}
                    onChange={e => setRoutingForm(p => ({ ...p, bypass_cache: e.target.checked }))} size="small" />
                  <Box>
                    <Typography variant="body2" sx={{ fontWeight: 600 }}>跳过缓存</Typography>
                    <Typography variant="caption" color="text.secondary">
                      开启后 Edge 节点将跳过缓存，每次请求直接回源
                    </Typography>
                  </Box>
                </Box>
              </FormSection>
            </Stack>
          ) : (
            <Stack spacing={3}>
              <FormSection title="基本信息">
                <TextField label="规则名称" value={redirectForm.name}
                  onChange={handleRedirectChange('name')} fullWidth placeholder="例如：API v1→v2 迁移" size="small" />
                <TextField label="匹配域名" value={redirectForm.domain}
                  onChange={handleRedirectChange('domain')} required fullWidth
                  placeholder="例如：api.veer.local" size="small"
                  helperText="仅匹配此域名的请求" />
                <TextField label="描述" value={redirectForm.description}
                  onChange={handleRedirectChange('description')} fullWidth
                  placeholder="规则用途说明（可选）" size="small" multiline rows={2} />
              </FormSection>
              <Divider />
              <FormSection title="匹配规则">
                <TextField label="匹配方式" select value={redirectForm.match_type}
                  onChange={handleRedirectChange('match_type')} fullWidth size="small">
                  <MenuItem value="prefix">前缀匹配 — 匹配指定前缀的所有路径</MenuItem>
                  <MenuItem value="exact">精确匹配 — 完全相等的路径才匹配</MenuItem>
                  <MenuItem value="regex">正则匹配 — 使用正则表达式</MenuItem>
                </TextField>
                <TextField label="匹配路径" value={redirectForm.source_path}
                  onChange={handleRedirectChange('source_path')} required fullWidth size="small"
                  placeholder={redirectForm.match_type === 'regex' ? '^/api/v1/.*' : '/api/v1/*'}
                  helperText={redirectForm.match_type === 'prefix' ? '以 / 开头，匹配该前缀的所有请求'
                    : redirectForm.match_type === 'exact' ? '以 / 开头，完全匹配该路径'
                    : '以 / 开头的正则表达式'} />
              </FormSection>
              <Divider />
              <FormSection title="重定向目标">
                <TextField label="目标 Host" value={redirectForm.target_host}
                  onChange={handleRedirectChange('target_host')} fullWidth size="small"
                  placeholder="留空则保持原域名"
                  helperText="可选，可跨域名重定向" />
                <TextField label="目标路径" value={redirectForm.target_path}
                  onChange={handleRedirectChange('target_path')} fullWidth size="small"
                  placeholder="例如：/api/v2/$1"
                  helperText="可选，支持 $1 捕获路径片段。留空保持原路径" />
                <TextField label="状态码" select value={redirectForm.redirect_code}
                  onChange={handleRedirectChange('redirect_code')} fullWidth size="small">
                  <MenuItem value={301}>301 永久重定向 (Moved Permanently)</MenuItem>
                  <MenuItem value={302}>302 临时重定向 (Found)</MenuItem>
                </TextField>
              </FormSection>
            </Stack>
          )}
        </Box>

        <Box sx={{ px: 3, py: 2, borderTop: '1px solid', borderColor: 'divider',
          display: 'flex', alignItems: 'center' }}>
          {editingRule && (
            <Button variant="outlined" color="error" size="small"
              startIcon={<DeleteIcon />} onClick={handleDeleteFromDrawer} disabled={saving}>
              删除规则
            </Button>
          )}
          <Box sx={{ display: 'flex', gap: 1, ml: 'auto' }}>
            <Button onClick={handleCloseDrawer} disabled={saving}>取消</Button>
            <Button variant="contained" onClick={handleSave} disabled={saving}>
              {saving && <CircularProgress size={16} sx={{ mr: 0.5 }} />}
              {editingRule ? '保存修改' : '创建规则'}
            </Button>
          </Box>
        </Box>
      </Drawer>

      <Snackbar open={snackbar.open} autoHideDuration={3000}
        onClose={() => setSnackbar({ open: false, message: '' })} message={snackbar.message}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }} />
    </Box>
  )
}

export default Rules
