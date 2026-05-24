import Chip from '@mui/material/Chip'

/**
 * StatusChip displays a colored chip based on status value.
 *
 * @param {Object} props
 * @param {string} props.status - Status value: 'active' | 'inactive' | any string
 * @param {Object} [props.sx] - Additional MUI sx styles
 */
function StatusChip({ status, sx = {} }) {
  const statusConfig = {
    active: { label: '活跃', color: 'success' },
    inactive: { label: '停用', color: 'default' },
    'round-robin': { label: '轮询', color: 'primary' },
    weighted: { label: '权重', color: 'secondary' },
    random: { label: '随机', color: 'warning' },
    score: { label: '智能评分', color: 'info' },
  }

  const config = statusConfig[status] || { label: status, color: 'default' }

  return (
    <Chip
      label={config.label}
      color={config.color}
      size="small"
      sx={{ fontWeight: 500, ...sx }}
    />
  )
}

export default StatusChip
