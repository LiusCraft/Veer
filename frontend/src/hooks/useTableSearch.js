import { useState, useMemo } from 'react'

export default function useTableSearch(data, searchFields = [], initialSortField = null, initialSortDir = 'asc') {
  const [search, setSearch] = useState('')
  const [sortField, setSortField] = useState(initialSortField)
  const [sortDir, setSortDir] = useState(initialSortDir)

  const filteredData = useMemo(() => {
    let result = [...data]

    if (search.trim()) {
      const keyword = search.toLowerCase()
      result = result.filter(item =>
        searchFields.some(field => {
          const val = item[field]
          return val != null && String(val).toLowerCase().includes(keyword)
        })
      )
    }

    if (sortField) {
      result.sort((a, b) => {
        let valA = a[sortField]
        let valB = b[sortField]
        if (typeof valA === 'string') valA = valA.toLowerCase()
        if (typeof valB === 'string') valB = valB.toLowerCase()
        if (valA < valB) return sortDir === 'asc' ? -1 : 1
        if (valA > valB) return sortDir === 'asc' ? 1 : -1
        return 0
      })
    }

    return result
  }, [data, search, searchFields, sortField, sortDir])

  const handleSort = (field) => {
    if (sortField === field) {
      setSortDir(prev => prev === 'asc' ? 'desc' : 'asc')
    } else {
      setSortField(field)
      setSortDir('asc')
    }
  }

  return { search, setSearch, sortField, sortDir, handleSort, filteredData }
}
