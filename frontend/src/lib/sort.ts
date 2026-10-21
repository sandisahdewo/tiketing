// Column sorting shared by the tables. Big, paged lists sort on the server;
// the small lists that come whole (report rows, users) sort here.

export type SortDir = 'asc' | 'desc'
export type Sort = { key: string; dir: SortDir }

// nextSort is the sort after clicking a column: the same column flips its
// direction, another column starts with firstDir.
export function nextSort(current: Sort, key: string, firstDir: SortDir = 'asc'): Sort {
  if (current.key === key) return { key, dir: current.dir === 'asc' ? 'desc' : 'asc' }
  return { key, dir: firstDir }
}

const collator = new Intl.Collator('id', { sensitivity: 'base', numeric: true })

// sortRows returns a sorted copy; text compares case-insensitively.
export function sortRows<T>(rows: T[], value: (row: T) => string | number | null | undefined, dir: SortDir): T[] {
  const sign = dir === 'asc' ? 1 : -1
  return rows
    .map((row, i) => ({ row, i, v: value(row) }))
    .sort((a, b) => {
      let c: number
      if (a.v == null || b.v == null) c = a.v == null ? (b.v == null ? 0 : -1) : 1
      else if (typeof a.v === 'number' && typeof b.v === 'number') c = a.v - b.v
      else c = collator.compare(String(a.v), String(b.v))
      return c * sign || a.i - b.i
    })
    .map((x) => x.row)
}
