export function exportToCSV(data, filename, columns) {
  const BOM = '\uFEFF'
  const header = columns.map(c => c.label).join(',')
  const rows = data.map(item =>
    columns.map(c => {
      let val = item[c.key] ?? ''
      val = String(val).replace(/"/g, '""')
      if (/[,"\n\r]/.test(val)) val = `"${val}"`
      return val
    }).join(',')
  )
  const csv = BOM + header + '\n' + rows.join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = filename
  link.click()
  URL.revokeObjectURL(link.href)
}
