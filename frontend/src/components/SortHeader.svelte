<script lang="ts">
  import { nextSort, type Sort, type SortDir } from '../lib/sort'

  // A clickable column header: click sorts by this column, click again flips it.
  let {
    label,
    key,
    sort,
    onsort,
    align = 'left',
    firstDir = 'asc'
  }: {
    label: string
    key: string
    sort: Sort
    onsort: (sort: Sort) => void
    align?: 'left' | 'right' | 'center'
    // numbers and dates usually start from the largest/newest
    firstDir?: SortDir
  } = $props()

  let active = $derived(sort.key === key)
</script>

<th class={align === 'right' ? 'text-right' : align === 'center' ? 'text-center' : ''} aria-sort={active ? (sort.dir === 'asc' ? 'ascending' : 'descending') : 'none'}>
  <button
    type="button"
    class="inline-flex items-center gap-1 font-semibold hover:text-slate-900 {active ? 'text-brand-700' : ''}"
    onclick={() => onsort(nextSort(sort, key, firstDir))}
    title="Urutkan berdasarkan {label}"
  >
    {label}
    <span class="w-3 text-xs {active ? '' : 'text-slate-300'}">{active ? (sort.dir === 'asc' ? '▲' : '▼') : '↕'}</span>
  </button>
</th>
