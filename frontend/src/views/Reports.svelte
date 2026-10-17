<script lang="ts">
  import ReceiptModal from '../components/ReceiptModal.svelte'
  import { App, model, handleError } from '../lib/api'
  import { formatRp } from '../lib/money'
  import { formatDateTime, formatPeriod, toISODate, today } from '../lib/date'
  import { categories } from '../lib/colors'
  import { toast } from '../lib/toast.svelte'

  type Period = 'daily' | 'weekly' | 'monthly'

  const periods: { value: Period; label: string; unit: string }[] = [
    { value: 'daily', label: 'Harian', unit: 'hari' },
    { value: 'weekly', label: 'Mingguan', unit: 'minggu' },
    { value: 'monthly', label: 'Bulanan', unit: 'bulan' }
  ]

  function startOfMonth(): string {
    const d = new Date()
    return toISODate(new Date(d.getFullYear(), d.getMonth(), 1))
  }

  let period = $state<Period>('daily')
  let from = $state(startOfMonth())
  let to = $state(today())
  let report = $state<model.Report | null>(null)
  let sales = $state<model.Sale[]>([])
  let loading = $state(false)
  let tab = $state<'period' | 'ticket' | 'sales'>('period')
  let selectedSale = $state<number | null>(null)

  let itemGroups = $derived(
    categories
      .map((c) => {
        const items = report?.tickets.filter((t) => t.category === c.value) ?? []
        return {
          label: c.label,
          items,
          qty: items.reduce((sum, t) => sum + t.qty, 0),
          total: items.reduce((sum, t) => sum + t.total, 0)
        }
      })
      .filter((g) => g.items.length > 0)
  )

  function preset(kind: string) {
    const now = new Date()
    if (kind === 'today') {
      from = to = today()
    } else if (kind === 'week') {
      const d = new Date(now)
      d.setDate(d.getDate() - ((d.getDay() + 6) % 7))
      from = toISODate(d)
      to = today()
    } else if (kind === 'month') {
      from = startOfMonth()
      to = today()
    } else if (kind === 'year') {
      from = `${now.getFullYear()}-01-01`
      to = today()
    }
    load()
  }

  async function load() {
    loading = true
    try {
      const [r, s] = await Promise.all([
        App.Report(model.ReportQuery.createFrom({ period, from, to })),
        App.ListSales(model.SaleQuery.createFrom({ from, to }))
      ])
      report = r
      sales = s
    } catch (e) {
      handleError(e)
    } finally {
      loading = false
    }
  }

  let exporting = $state<'pdf' | 'xlsx' | null>(null)

  async function exportReport(format: 'pdf' | 'xlsx') {
    exporting = format
    try {
      const path = await App.ExportReport(model.ReportQuery.createFrom({ period, from, to }), format)
      if (path) toast('Laporan disimpan: ' + path)
    } catch (e) {
      handleError(e)
    } finally {
      exporting = null
    }
  }

  $effect(() => {
    period
    load()
  })
</script>

<div class="flex h-full flex-col p-6">
  <div class="flex items-center justify-between">
    <h1 class="text-2xl font-bold">Laporan Penjualan</h1>
    <div class="flex gap-2">
      <button class="btn btn-secondary" onclick={() => exportReport('pdf')} disabled={exporting !== null || !report}>{exporting === 'pdf' ? 'Menyiapkan…' : 'Cetak PDF'}</button>
      <button class="btn btn-secondary" onclick={() => exportReport('xlsx')} disabled={exporting !== null || !report}>{exporting === 'xlsx' ? 'Menyiapkan…' : 'Export Excel'}</button>
    </div>
  </div>

  <div class="card mt-4 flex flex-wrap items-end gap-4 p-4">
    <div>
      <span class="label">Periode</span>
      <div class="flex rounded-md border border-slate-300 bg-white p-0.5 text-sm">
        {#each periods as p}
          <button class="rounded px-4 py-1.5 {period === p.value ? 'bg-brand-600 text-white' : 'text-slate-600'}" onclick={() => (period = p.value)}>{p.label}</button>
        {/each}
      </div>
    </div>
    <div>
      <label class="label" for="from">Dari</label>
      <input id="from" class="input" type="date" bind:value={from} />
    </div>
    <div>
      <label class="label" for="to">Sampai</label>
      <input id="to" class="input" type="date" bind:value={to} />
    </div>
    <button class="btn btn-primary" onclick={load} disabled={loading}>{loading ? 'Memuat…' : 'Tampilkan'}</button>
    <div class="ml-auto flex gap-1 text-sm">
      <button class="btn btn-ghost px-2 py-1" onclick={() => preset('today')}>Hari ini</button>
      <button class="btn btn-ghost px-2 py-1" onclick={() => preset('week')}>Minggu ini</button>
      <button class="btn btn-ghost px-2 py-1" onclick={() => preset('month')}>Bulan ini</button>
      <button class="btn btn-ghost px-2 py-1" onclick={() => preset('year')}>Tahun ini</button>
    </div>
  </div>

  {#if report}
    <div class="mt-4 grid grid-cols-4 gap-4">
      <div class="card p-4">
        <div class="text-sm text-slate-500">Jumlah transaksi</div>
        <div class="text-2xl font-bold">{report.transactions.toLocaleString('id-ID')}</div>
      </div>
      <div class="card p-4">
        <div class="text-sm text-slate-500">Tiket ({report.ticketQty.toLocaleString('id-ID')} terjual)</div>
        <div class="text-2xl font-bold">{formatRp(report.ticketTotal)}</div>
      </div>
      <div class="card p-4">
        <div class="text-sm text-slate-500">Topup Card ({report.topupQty.toLocaleString('id-ID')} terjual)</div>
        <div class="text-2xl font-bold">{formatRp(report.topupTotal)}</div>
      </div>
      <div class="card p-4">
        <div class="text-sm text-slate-500">Total pendapatan</div>
        <div class="text-2xl font-bold text-brand-700">{formatRp(report.total)}</div>
      </div>
    </div>

    <div class="mt-4 flex gap-1 border-b border-slate-200">
      <button class="-mb-px border-b-2 px-4 py-2 text-sm font-medium {tab === 'period' ? 'border-brand-600 text-brand-700' : 'border-transparent text-slate-500'}" onclick={() => (tab = 'period')}>Per {periods.find((p) => p.value === report?.period)?.unit}</button>
      <button class="-mb-px border-b-2 px-4 py-2 text-sm font-medium {tab === 'ticket' ? 'border-brand-600 text-brand-700' : 'border-transparent text-slate-500'}" onclick={() => (tab = 'ticket')}>Per item</button>
      <button class="-mb-px border-b-2 px-4 py-2 text-sm font-medium {tab === 'sales' ? 'border-brand-600 text-brand-700' : 'border-transparent text-slate-500'}" onclick={() => (tab = 'sales')}>Daftar transaksi ({sales.length})</button>
    </div>

    <div class="card mt-2 min-h-0 flex-1 overflow-y-auto">
      {#if tab === 'period'}
        <table class="table">
          <thead class="sticky top-0">
            <tr><th>Periode</th><th class="text-right">Transaksi</th><th class="text-right">Tiket</th><th class="text-right">Rp Tiket</th><th class="text-right">Topup</th><th class="text-right">Rp Topup</th><th class="text-right">Total</th></tr>
          </thead>
          <tbody>
            {#each report.rows as r}
              <tr>
                <td>{formatPeriod(r.period, report.period)}</td>
                <td class="text-right">{r.transactions}</td>
                <td class="text-right">{r.ticketQty}</td>
                <td class="text-right">{formatRp(r.ticketTotal)}</td>
                <td class="text-right">{r.topupQty}</td>
                <td class="text-right">{formatRp(r.topupTotal)}</td>
                <td class="text-right font-medium">{formatRp(r.total)}</td>
              </tr>
            {:else}
              <tr><td colspan="7" class="py-10 text-center text-slate-500">Tidak ada penjualan pada rentang ini.</td></tr>
            {/each}
          </tbody>
        </table>
      {:else if tab === 'ticket'}
        <table class="table">
          <thead class="sticky top-0"><tr><th>Item</th><th class="text-right">Terjual</th><th class="text-right">Total</th></tr></thead>
          {#each itemGroups as g (g.label)}
            <tbody>
              <tr class="bg-slate-50"><td colspan="3" class="font-semibold text-brand-700">{g.label}</td></tr>
              {#each g.items as r (r.ticketId)}
                <tr><td class="pl-8">{r.ticketName}</td><td class="text-right">{r.qty}</td><td class="text-right font-medium">{formatRp(r.total)}</td></tr>
              {/each}
              <tr class="font-semibold"><td class="pl-8">Subtotal {g.label}</td><td class="text-right">{g.qty}</td><td class="text-right">{formatRp(g.total)}</td></tr>
            </tbody>
          {:else}
            <tbody><tr><td colspan="3" class="py-10 text-center text-slate-500">Tidak ada penjualan pada rentang ini.</td></tr></tbody>
          {/each}
        </table>
      {:else}
        <table class="table">
          <thead class="sticky top-0"><tr><th>Waktu</th><th>No. Nota</th><th>Kasir</th><th>Loket</th><th class="text-right">Total</th><th class="text-center">Cetak</th><th></th></tr></thead>
          <tbody>
            {#each sales as s (s.id)}
              <tr class="hover:bg-slate-50">
                <td>{formatDateTime(s.createdAt)}</td>
                <td class="font-mono">{s.invoiceNo}</td>
                <td>{s.cashierName}</td>
                <td>{s.terminal}</td>
                <td class="text-right font-medium">{formatRp(s.total)}</td>
                <td class="text-center text-slate-500">{s.printCount}×</td>
                <td class="text-right"><button class="btn btn-secondary px-3 py-1" onclick={() => (selectedSale = s.id)}>Nota</button></td>
              </tr>
            {:else}
              <tr><td colspan="7" class="py-10 text-center text-slate-500">Tidak ada transaksi pada rentang ini.</td></tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  {/if}
</div>

{#if selectedSale !== null}
  <ReceiptModal saleId={selectedSale} onclose={() => (selectedSale = null)} />
{/if}
