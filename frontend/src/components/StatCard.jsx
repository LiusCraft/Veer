import { Card, CardContent, Typography, Box, Skeleton, useTheme, alpha } from '@mui/material'

/**
 * StatCard displays a single statistic with icon, label, and value.
 *
 * @param {Object} props
 * @param {string} props.title - Card title
 * @param {string|number} props.value - Statistic value to display
 * @param {React.ReactNode} props.icon - MUI icon component
 * @param {string} props.color - MUI color string for icon background
 * @param {boolean} [props.loading] - Show skeleton loader
 * @param {string} [props.subtitle] - Optional subtitle text
 */
function StatCard({ title, value, icon, color = 'primary.main', loading = false, subtitle }) {
  const theme = useTheme()
  const isDark = theme.palette.mode === 'dark'

  return (
    <Card
      sx={{
        height: '100%',
        transition: 'transform 0.2s, box-shadow 0.2s',
        '&:hover': {
          transform: 'translateY(-2px)',
          boxShadow: isDark
            ? '0 4px 20px rgba(0,0,0,0.4)'
            : '0 4px 20px rgba(0,0,0,0.12)',
        },
      }}
    >
      <CardContent sx={{ p: 3 }}>
        <Box sx={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between' }}>
          <Box sx={{ flex: 1 }}>
            <Typography variant="body2" color="text.secondary" gutterBottom sx={{ fontWeight: 500 }}>
              {title}
            </Typography>
            {loading ? (
              <Skeleton variant="text" width={80} height={44} />
            ) : (
              <Typography variant="h4" sx={{ fontWeight: 700, color: 'text.primary', lineHeight: 1.2 }}>
                {value}
              </Typography>
            )}
            {subtitle && (
              <Typography variant="caption" color="text.disabled" sx={{ mt: 0.5, display: 'block' }}>
                {subtitle}
              </Typography>
            )}
          </Box>
          <Box
            sx={{
              width: 56,
              height: 56,
              borderRadius: 2,
              bgcolor: isDark ? alpha(color, 0.2) : color,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              opacity: isDark ? 1 : 0.9,
              flexShrink: 0,
              ml: 2,
            }}
          >
            <Box sx={{ color: isDark ? color : 'white', display: 'flex', alignItems: 'center' }}>
              {icon}
            </Box>
          </Box>
        </Box>
      </CardContent>
    </Card>
  )
}

export default StatCard
