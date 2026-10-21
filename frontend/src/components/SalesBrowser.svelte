<script lang="ts">
  import { untrack } from 'svelte'
  import SalesTable from './SalesTable.svelte'
  import Pager from './Pager.svelte'
  import LoadingOverlay from './LoadingOverlay.svelte'
  import type { Sort } from '../lib/sort'
  import { App, model, handleError } from '../lib/api'
  import { formatRp } from '../lib/money'

  // Searchable, paged transaction list. Search and paging run on the server,
  // so long ranges never load every sale at once.
  let {
    from,
    to,
    deleted = false,
    refresh = 0,
    waiting = false,
    emptyText = 'Tidak ada transaksi.',
    highlight = null,
    filter = { category: '', userId: 0, counterId: 0 },
    ondeleted
  }: {
    from: string
    to: string
    deleted?: boolean
    // bump to reload with the same filters
    refresh?: number
    // the parent is still loading something this list depends on
    waiting?: boolean
    emptyText?: string
    // a sale to show at the top of the first page and blink (just made)
    highlight?: number | null
    // optional report filters (category, cashier, counter)
    filter?: { category: string; userId: number; counterId: number }
    ondeleted?: () => void
  } = $props()

  // highlight is applied to the first load only; later pages and searches are normal.
  let pending = untrack(() => highlight)
  let flash = $state<number | null>(null)
  // Newest first unless the user picks a column.
  let sort = $state<Sort>({ key: 'time', dir: 'desc' })

  const pageSize = 100

  let search = $state('')
  let appliedSearch = $state('')
  let page = $state(1)
  let result = $state<model.SalePage | null>(null)
  let loading = $state(false)
  let expanded = $state<Record<number, boolean>>({})
  let requestSeq = 0

  let sales = $derived(result?.sales ?? [])
  let pages = $derived(result ? Math.max(1, Math.ceil(result.count / pageSize)) : 1)
  let allExpanded = $derived(sales.length > 0 && sales.every((s) => expanded[s.id]))

  async function load() {
    const seq = ++requestSeq
    loading = true
    try {
      const out = await App.SearchSales(model.SalePageQuery.createFrom({ from, to, deleted, search: appliedSearch, page, pageSize, sort: sort.key, dir: sort.dir, ...filter }))
      if (seq !== requestSeq) return
      // The last page can empty out after a delete; step back to the new last page.
      if (out.sales.length === 0 && out.count > 0 && page > 1) {
        page = Math.ceil(out.count / pageSize)
        return load()
      }
      if (pending !== null && page === 1 && !appliedSearch && !deleted) {
        const id = pending
        pending = null
        const rest = out.sales.filter((s) => s.id !== id)
        const sale = out.sales.find((s) => s.id === id) ?? (await App.GetSale(id))
        if (seq !== requestSeq) return
        out.sales = [sale, ...rest]
        flash = id
        setTimeout(() => (flash = null), 2000)
      }
      result = out
      expanded = {}
    } catch (e) {
      if (seq === requestSeq) handleError(e)
    } finally {
      if (seq === requestSeq) loading = false
    }
  }

  function goto(p: number) {
    page = Math.min(Math.max(1, p), pages)
    load()
  }

  function toggleAll() {
    const open = !allExpanded
    expanded = Object.fromEntries(sales.map((s) => [s.id, open]))
  }

  // Wait for a pause in typing before searching.
  $effect(() => {
    const value = search.trim()
    const timer = setTimeout(() => (appliedSearch = value), 300)
    return () => clearTimeout(timer)
  })

  // New filters start again from page 1.
  $effect(() => {
    from
    to
    deleted
    appliedSearch
    refresh
    sort
    filter
    untrack(() => {
      page = 1
      load()
    })
  })
</script>

<div class="flex min-h-0 flex-1 flex-col">
  <div class="flex items-center gap-4">
    <input class="input w-96" type="search" placeholder="Cari no. nota, kasir, loket, atau item…" bind:value={search} />
    {#if loading || waiting || !result}
      <span class="h-5 w-56 animate-pulse rounded bg-slate-200" aria-label="Memuat"></span>
    {:else}
      <span class="text-sm text-slate-500">
        {result.count.toLocaleString('id-ID')} {deleted ? 'nota dihapus' : 'transaksi'}{#if !deleted} · <b class="text-slate-700">{formatRp(result.amount)}</b>{/if}
      </span>
    {/if}
    <button class="btn btn-ghost ml-auto px-2 py-1 text-sm" onclick={toggleAll} disabled={sales.length === 0}>{allExpanded ? 'Tutup semua rincian' : 'Buka semua rincian'}</button>
  </div>

  <div class="relative mt-2 flex min-h-0 flex-1 flex-col">
    <div class="card min-h-40 flex-1 overflow-y-auto">
      {#if result}
        <SalesTable {sales} bind:expanded {flash} {sort} onsort={(s) => (sort = s)} empty={appliedSearch ? 'Tidak ada transaksi yang cocok.' : emptyText} ondeleted={() => { load(); ondeleted?.() }} />
      {/if}
    </div>
    <LoadingOverlay show={loading || waiting || !result} />
  </div>

  {#if result}
    <Pager {page} {pageSize} count={result.count} {loading} ongoto={goto} />
  {/if}
</div>
