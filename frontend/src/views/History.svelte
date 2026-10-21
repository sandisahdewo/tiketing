<script lang="ts">
  import SalesTable from '../components/SalesTable.svelte'
  import { App, model, handleError } from '../lib/api'
  import { formatRp } from '../lib/money'
  import { formatDate, today } from '../lib/date'
  import { isAdmin } from '../lib/session.svelte'

  let sales = $state<model.Sale[]>([])
  let loading = $state(true)
  let search = $state('')
  let expanded = $state<Record<number, boolean>>({})

  // Matches invoice number, cashier, counter or any item name.
  let filtered = $derived.by(() => {
    const q = search.trim().toLowerCase()
    if (!q) return sales
    return sales.filter((s) =>
      [s.invoiceNo, s.cashierName, s.terminal, ...s.items.map((it) => it.ticketName)].some((v) => v.toLowerCase().includes(q))
    )
  })
  let total = $derived(filtered.reduce((sum, s) => sum + s.total, 0))
  let allExpanded = $derived(filtered.length > 0 && filtered.every((s) => expanded[s.id]))

  function toggleAll() {
    const open = !allExpanded
    expanded = Object.fromEntries(filtered.map((s) => [s.id, open]))
  }

  async function load() {
    loading = true
    try {
      const d = today()
      sales = await App.ListSales(model.SaleQuery.createFrom({ from: d, to: d }))
    } catch (e) {
      handleError(e)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    load()
  })
</script>

<div class="flex h-full flex-col p-6">
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-2xl font-bold">Riwayat Hari Ini</h1>
      <p class="text-sm text-slate-500">{formatDate(today())} · {isAdmin() ? 'semua kasir' : 'transaksi Anda'}</p>
    </div>
    <button class="btn btn-secondary" onclick={load} disabled={loading}>{loading ? 'Memuat…' : 'Muat ulang'}</button>
  </div>

  <div class="mt-4 flex items-center gap-4">
    <!-- svelte-ignore a11y_autofocus -->
    <input class="input w-96" type="search" placeholder="Cari no. nota, kasir, loket, atau item…" bind:value={search} autofocus />
    <span class="text-sm text-slate-500">{filtered.length} transaksi · <b class="text-slate-700">{formatRp(total)}</b></span>
    <button class="btn btn-ghost ml-auto px-2 py-1 text-sm" onclick={toggleAll} disabled={filtered.length === 0}>{allExpanded ? 'Tutup semua rincian' : 'Buka semua rincian'}</button>
  </div>

  <div class="card mt-2 min-h-0 flex-1 overflow-y-auto">
    {#if loading && sales.length === 0}
      <p class="py-10 text-center text-slate-500">Memuat…</p>
    {:else}
      <SalesTable
        sales={filtered}
        bind:expanded
        timeOnly
        empty={search.trim() ? 'Tidak ada transaksi yang cocok.' : 'Belum ada transaksi hari ini.'}
        ondeleted={load}
      />
    {/if}
  </div>
</div>
