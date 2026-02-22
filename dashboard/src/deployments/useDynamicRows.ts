import { useRef, useState } from 'react'

export type Row = { id: number }

export function useDynamicRows<T extends Row>(factory: (id: number) => T, initialRows: T[] = []) {
  const nextId = useRef(initialRows.length > 0 ? Math.max(...initialRows.map(r => r.id)) + 1 : 0)
  const [rows, setRows] = useState<T[]>(initialRows)

  const add = () => {
    const id = nextId.current++
    setRows(r => [...r, factory(id)])
  }

  const remove = (id: number) => setRows(r => r.filter(row => row.id !== id))

  const update = (id: number, patch: Partial<Omit<T, 'id'>>) =>
    setRows(r => r.map(row => row.id === id ? { ...row, ...patch } : row))

  const reset = () => setRows([])

  return { rows, add, remove, update, reset }
}
