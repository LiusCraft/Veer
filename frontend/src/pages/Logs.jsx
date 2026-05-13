import { useState, useEffect, useCallback } from 'react'
import {
  Box,
  Typography,
  Card,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Alert,
  CircularProgress,
  Chip,
  Stack,
  TextField,
  TablePagination,
  Button,
} from '@mui/material'
import RefreshIcon from '@mui/icons-material/Refresh'
import ClearIcon from '@mui/icons-material/Clear'
import FileDownloadIcon from '@mui/icons-material/FileDownload'
import { statsApi } from '../api/index.js'
import { exportToCSV } from '../utils/csv.js'

function Logs() {
  const [logs, setLogs] = useState([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [pageSize, setPageSize] = useState(20)
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const loadLogs = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const params = {
        page: page + 1,
        page_size: pageSize,
      }
      if (startTime) {
        params.start_time = startTime
      }
      if (endTime) {
        params.end_time = endTime + 'T23:59:59'
      }
      const res = await statsApi.logs(params)
      setLogs(res.data || [])
      setTotal(res.total || 0)
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, startTime, endTime])

  useEffect(() => {
    loadLogs()
  }, [loadLogs])

  const handleSearch = () => {
    setPage(0)
    loadLogs()
  }

  const handleClearFilter = () => {
    setStartTime('')
    setEndTime('')
    setPage(0)
  }

  const handlePageChange = (_, newPage) => {
    setPage(newPage)
  }

  const handlePageSizeChange = (e) => {
    setPageSize(parseInt(e.target.value, 10))
    setPage(0)
  }

  const formatDateTime = (dateStr) => {
    if (!dateStr) return '-'
    return new Date(dateStr).toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  }

  const handleExportCSV = () => {
    const today = new Date().toISOString().slice(0, 10)
    const filename = `access-logs-${today}.csv`
    const columns = [
      { key: 'domain', label: '域名' },
      { key: 'path', label: '路径' },
      { key: 'node_name', label: '节点名称' },
      { key: 'target_url', label: '目标URL' },
      { key: 'client_ip', label: '客户端IP' },
      { key: 'status_code', label: '状态码' },
      { key: 'created_at', label: '时间' },
    ]
    exportToCSV(logs, filename, columns)
  }

  const hasActiveFilter = startTime || endTime

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 3 }}>
        <Typography variant="h5" sx={{ fontWeight: 600 }}>
          访问日志
        </Typography>
        <Button variant="outlined" startIcon={<RefreshIcon />} onClick={loadLogs} disabled={loading}>
          刷新
        </Button>
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      <Card sx={{ p: 2, mb: 2 }}>
        <Stack direction="row" spacing={2} alignItems="center" flexWrap="wrap">
          <TextField
            size="small"
            label="开始时间"
            type="date"
            value={startTime}
            onChange={(e) => { setStartTime(e.target.value); setPage(0) }}
            InputLabelProps={{ shrink: true }}
            sx={{ width: 170 }}
          />
          <TextField
            size="small"
            label="结束时间"
            type="date"
            value={endTime}
            onChange={(e) => { setEndTime(e.target.value); setPage(0) }}
            InputLabelProps={{ shrink: true }}
            sx={{ width: 170 }}
          />
          <Button variant="contained" size="small" onClick={handleSearch}>
            搜索
          </Button>
          {hasActiveFilter && (
            <Button size="small" startIcon={<ClearIcon />} onClick={handleClearFilter}>
              清除筛选
            </Button>
          )}
          <Button
            variant="outlined"
            size="small"
            startIcon={<FileDownloadIcon />}
            onClick={handleExportCSV}
            disabled={loading || logs.length === 0}
          >
            导出 CSV
          </Button>
          <Typography variant="body2" color="text.secondary" sx={{ ml: 'auto' }}>
            共 {total} 条记录
          </Typography>
        </Stack>
      </Card>

      <Card>
        <TableContainer>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>时间</TableCell>
                <TableCell>域名</TableCell>
                <TableCell>路径</TableCell>
                <TableCell>命中节点</TableCell>
                <TableCell>目标 URL</TableCell>
                <TableCell>来源 IP</TableCell>
                <TableCell align="center">状态码</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={7} align="center" sx={{ py: 4 }}>
                    <CircularProgress size={32} />
                  </TableCell>
                </TableRow>
              ) : logs.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} align="center" sx={{ py: 4, color: 'text.secondary' }}>
                    {hasActiveFilter ? '未找到匹配的日志记录' : '暂无访问记录'}
                  </TableCell>
                </TableRow>
              ) : (
                logs.map((log) => (
                  <TableRow key={log.id} hover>
                    <TableCell>
                      <Typography variant="caption" color="text.secondary" sx={{ whiteSpace: 'nowrap' }}>
                        {formatDateTime(log.created_at)}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      {log.domain ? (
                        <Chip label={log.domain} size="small" variant="outlined" />
                      ) : (
                        <Typography variant="body2" color="text.disabled">-</Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      <Typography variant="caption" sx={{ fontFamily: 'monospace' }}>
                        {log.path || '-'}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Chip label={log.node_name || `Node#${log.node_id}`} size="small" variant="outlined" />
                    </TableCell>
                    <TableCell>
                      <Typography
                        variant="caption"
                        sx={{
                          fontFamily: 'monospace',
                          maxWidth: 280,
                          display: 'block',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap',
                        }}
                      >
                        {log.target_url}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="caption" sx={{ fontFamily: 'monospace' }}>
                        {log.client_ip || '-'}
                      </Typography>
                    </TableCell>
                    <TableCell align="center">
                      <Chip
                        label={log.status_code}
                        size="small"
                        color={log.status_code === 302 ? 'info' : log.status_code < 400 ? 'success' : 'error'}
                      />
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
        <TablePagination
          component="div"
          count={total}
          page={page}
          onPageChange={handlePageChange}
          rowsPerPage={pageSize}
          onRowsPerPageChange={handlePageSizeChange}
          rowsPerPageOptions={[10, 20, 50, 100]}
          labelRowsPerPage="每页条数："
          labelDisplayedRows={({ from, to, count }) => `${from}\u2013${to} / 共 ${count} 条`}
        />
      </Card>
    </Box>
  )
}

export default Logs
