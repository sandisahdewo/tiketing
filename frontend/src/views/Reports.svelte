<script lang="ts">
  import { untrack } from 'svelte'
  import SalesBrowser from '../components/SalesBrowser.svelte'
  import Pager from '../components/Pager.svelte'
  import LoadingOverlay from '../components/LoadingOverlay.svelte'
  import Field from '../components/Field.svelte'
  import DateRangeInput from '../components/DateRangeInput.svelte'
  import Select from '../components/Select.svelte'
  import SortHeader from '../components/SortHeader.svelte'
  import { sortRows, type Sort } from '../lib/sort'
  import { FormState } from '../lib/form.svelte'
  import { App, model, handleError, errorMessage } from '../lib/api'
  import { formatRp } from '../lib/money'
  import { formatDate, formatPeriod, today } from '../lib/date'
  import { categories } from '../lib/colors'
  import { toast } from '../lib/toast.svelte'
  import { session, isAdmin } from '../lib/session.svelte'

  // Cashiers only get today's report of their own sales; the server enforces this too.
  const admin = isAdmin()

  // todaySales opens the transaction list of today (the cashier's "Riwayat hari ini").
  // highlightSale is put on top of that list and blinks (a sale just made).
  let { todaySales = false, highlightSale = null }: { todaySales?: boolean; highlightSale?: number | null } = $props()
  // Only decides how the page opens; later changes don't matter.
  const openToday = untrack(() => todaySales)


  type Period = 'daily' | 'weekly' | 'monthly'

  const periods: { value: Period; label: string; unit: string }[] = [
    { value: 'daily', label: 'Harian', unit: 'hari' },
    { value: 'weekly', label: 'Mingguan', unit: 'minggu' },
    { value: 'monthly', label: 'Bulanan', unit: 'bulan' }
  ]

  let period = $state<Period>('daily')
  // Opens on today; other ranges are one click away in the date picker.
  let from = $state(today())
  let to = $state(today())
  let report = $state<model.Report | null>(null)
  let loading = $state(false)
  let tab = $state<'period' | 'ticket' | 'sales'>(admin && !openToday ? 'period' : 'sales')
  let showDeleted = $state(false)
  // Only the admin picks dates; cashiers always get today.
  // The range picker keeps the order of the two dates, so only a missing
  // range or a server message can show up here.
  const dateForm = new FormState(() => ({
    range: !from || !to ? 'Tanggal wajib dipilih' : undefined
  }))
  const dateFields = { range: /tanggal/i }

  // Optional filters (admin). They apply right away, like the period buttons.
  let category = $state('')
  let cashierId = $state(0)
  let counterId = $state(0)
  let filterChoices = $state<model.ReportFilters | null>(null)
  let filtered = $derived(category !== '' || cashierId !== 0 || counterId !== 0)
  // The filters of the report on screen; the transaction list follows them.
  let appliedFilter = $state({ category: '', userId: 0, counterId: 0 })

  function filterQuery() {
    return { category, userId: cashierId, counterId }
  }

  function resetFilter() {
    category = ''
    cashierId = 0
    counterId = 0
  }

  // Bumped on every "Tampilkan" so the transaction list reloads too.
  let salesRefresh = $state(0)

  // Both tabs come whole with the report, so they are sorted here: sorting on
  // the server would recompute the whole report on every click.
  let periodSort = $state<Sort>({ key: 'period', dir: 'asc' })
  let itemSort = $state<Sort>({ key: 'total', dir: 'desc' })
  const periodValue: Record<string, (r: model.ReportRow) => string | number> = {
    period: (r) => r.period,
    transactions: (r) => r.transactions,
    ticketQty: (r) => r.ticketQty,
    ticketTotal: (r) => r.ticketTotal,
    topupQty: (r) => r.topupQty,
    topupTotal: (r) => r.topupTotal,
    total: (r) => r.total
  }
  const itemValue: Record<string, (t: model.ReportTicketRow) => string | number> = {
    name: (t) => t.ticketName,
    qty: (t) => t.qty,
    total: (t) => t.total
  }

  function sortPeriods(s: Sort) {
    periodSort = s
    periodPage = 1
  }

  function sortItems(s: Sort) {
    itemSort = s
    itemPage = 1
  }

  // Items stay grouped by category; the sort applies within each group.
  let itemGroups = $derived(
    categories
      .map((c) => {
        const items = sortRows(report?.tickets.filter((t) => t.category === c.value) ?? [], itemValue[itemSort.key], itemSort.dir)
        return {
          label: c.label,
          items,
          qty: items.reduce((sum, t) => sum + t.qty, 0),
          total: items.reduce((sum, t) => sum + t.total, 0)
        }
      })
      .filter((g) => g.items.length > 0)
  )

  // "Per periode" and "Per item" come whole with the report (at most one row
  // per period and per product), so they are paged here on screen.
  const tabPageSize = 50
  let periodPage = $state(1)
  let itemPage = $state(1)
  let sortedPeriods = $derived(sortRows(report?.rows ?? [], periodValue[periodSort.key], periodSort.dir))
  let periodRows = $derived(sortedPeriods.slice((periodPage - 1) * tabPageSize, periodPage * tabPageSize))
  let itemRows = $derived(
    itemGroups.flatMap((g) => g.items.map((item, i) => ({ item, group: g, first: i === 0, last: i === g.items.length - 1 })))
  )
  let itemPageRows = $derived(itemRows.slice((itemPage - 1) * tabPageSize, itemPage * tabPageSize))

  // reloadSales is false after a delete: the list already refreshed itself and
  // should stay on its page.
  async function load(reloadSales = true) {
    if (admin && !dateForm.check()) return
    loading = true
    try {
      // The transaction list is paged and loaded by its own tab.
      const f = filterQuery()
      report = await App.Report(model.ReportQuery.createFrom({ period, from, to, ...f }))
      if (reloadSales) {
        periodPage = itemPage = 1
        appliedFilter = f
        salesRefresh++
      }
    } catch (e) {
      if (admin && /tanggal/i.test(errorMessage(e))) dateForm.fail(errorMessage(e), dateFields)
      else handleError(e)
    } finally {
      loading = false
    }
  }

  let exporting = $state<'pdf' | 'xlsx' | null>(null)

  async function exportReport(format: 'pdf' | 'xlsx') {
    if (exporting || (admin && !dateForm.check())) return
    exporting = format
    try {
      const path = await App.ExportReport(model.ReportQuery.createFrom({ period, from, to, ...filterQuery() }), format)
      if (path) toast('Laporan disimpan: ' + path)
    } catch (e) {
      handleError(e)
    } finally {
      exporting = null
    }
  }

  $effect(() => {
    period
    category
    cashierId
    counterId
    load()
  })

  $effect(() => {
    if (admin) App.ReportFilters().then((f) => (filterChoices = f)).catch(handleError)
  })

</script>

<div class="flex h-full flex-col p-6">
  <div class="flex items-center justify-between">
    {#if admin}
      <h1 class="text-2xl font-bold">Laporan Penjualan</h1>
    {:else}
      <div>
        <h1 class="text-2xl font-bold">Laporan Hari Ini</h1>
        <p class="text-sm text-slate-500">{session.user?.fullName} · {report ? formatDate(report.from) : '…'}</p>
      </div>
    {/if}
    <div class="flex gap-2">
      {#if !admin}
        <button class="btn btn-secondary" onclick={() => load()} disabled={loading}>{loading ? 'Memuat…' : 'Muat ulang'}</button>
      {/if}
      <button class="btn btn-secondary" onclick={() => exportReport('pdf')} disabled={exporting !== null || !report}>{exporting === 'pdf' ? 'Menyiapkan…' : 'Cetak PDF'}</button>
      <button class="btn btn-secondary" onclick={() => exportReport('xlsx')} disabled={exporting !== null || !report}>{exporting === 'xlsx' ? 'Menyiapkan…' : 'Export Excel'}</button>
    </div>
  </div>

  {#if admin}
  <div class="card mt-4 flex flex-wrap items-end gap-3 px-4 pt-4 pb-7">
    <div>
      <span class="label">Periode</span>
      <div class="flex rounded-md border border-slate-300 bg-white p-0.5 text-sm">
        {#each periods as p}
          <button class="rounded px-3 py-1.5 {period === p.value ? 'bg-brand-600 text-white' : 'text-slate-600'}" onclick={() => (period = p.value)}>{p.label}</button>
        {/each}
      </div>
    </div>
    <Field label="Tanggal" id="f-range" form={dateForm} name="range" floatError class="w-64">
      <DateRangeInput id="f-range" bind:from bind:to />
    </Field>
    <Field label="Kategori" id="f-category" class="w-[7.5rem]">
      <Select id="f-category" bind:value={category} options={[{ value: '', label: 'Semua' }, ...categories]} />
    </Field>
    <Field label="Kasir" id="f-cashier" class="w-[8.5rem]">
      <Select
        id="f-cashier"
        bind:value={cashierId}
        options={[{ value: 0, label: 'Semua' }, ...(filterChoices?.cashiers ?? []).map((c) => ({ value: c.id, label: c.fullName + (c.deleted ? ' (dihapus)' : '') }))]}
      />
    </Field>
    <Field label="Loket" id="f-terminal" class="w-28">
      <Select
        id="f-terminal"
        bind:value={counterId}
        options={[{ value: 0, label: 'Semua' }, ...(filterChoices?.counters ?? []).map((c) => ({ value: c.id, label: c.name + (c.active ? '' : ' (non-aktif)') }))]}
      />
    </Field>
    {#if filtered}
      <button class="btn btn-ghost px-2 py-2 text-sm text-brand-700" onclick={resetFilter}>Reset filter</button>
    {/if}
    <button class="btn btn-primary" onclick={() => load()} disabled={loading}>{loading ? 'Memuat…' : 'Tampilkan'}</button>
  </div>
  {/if}

  {#snippet stat(label: string, value: string, sold: number | null = null, accent = false)}
    <div class="card p-4">
      <div class="text-sm text-slate-500">{label}{#if sold !== null && !loading} ({sold.toLocaleString('id-ID')} terjual){/if}</div>
      {#if loading || !report}
        <div class="mt-1 h-8 w-32 animate-pulse rounded bg-slate-200"></div>
      {:else}
        <div class="text-2xl font-bold {accent ? 'text-brand-700' : ''}">{value}</div>
      {/if}
    </div>
  {/snippet}

  {#if report || loading}
    <div class="mt-4 grid grid-cols-4 gap-4">
      {@render stat('Jumlah transaksi', report?.transactions.toLocaleString('id-ID') ?? '')}
      {@render stat('Tiket', formatRp(report?.ticketTotal ?? 0), report?.ticketQty ?? null)}
      {@render stat('Topup Card', formatRp(report?.topupTotal ?? 0), report?.topupQty ?? null)}
      {@render stat('Total pendapatan', formatRp(report?.total ?? 0), null, true)}
    </div>

    <div class="mt-4 flex gap-1 border-b border-slate-200">
      {#if admin}
        <button class="-mb-px border-b-2 px-4 py-2 text-sm font-medium {tab === 'period' ? 'border-brand-600 text-brand-700' : 'border-transparent text-slate-500'}" onclick={() => (tab = 'period')}>Per {periods.find((p) => p.value === report?.period)?.unit}</button>
      {/if}
      <button class="-mb-px border-b-2 px-4 py-2 text-sm font-medium {tab === 'ticket' ? 'border-brand-600 text-brand-700' : 'border-transparent text-slate-500'}" onclick={() => (tab = 'ticket')}>Per item</button>
      <button class="-mb-px border-b-2 px-4 py-2 text-sm font-medium {tab === 'sales' ? 'border-brand-600 text-brand-700' : 'border-transparent text-slate-500'}" onclick={() => (tab = 'sales')}>{showDeleted ? 'Nota dihapus' : 'Daftar transaksi'}</button>
      {#if tab === 'sales' && admin}
        <label class="ml-auto flex items-center gap-2 pb-1 text-sm text-slate-600"><input type="checkbox" bind:checked={showDeleted} /> Tampilkan nota dihapus</label>
      {/if}
    </div>

    {#if tab === 'sales' && !report}
      <div class="relative mt-2 flex min-h-40 flex-1 flex-col">
        <div class="card flex-1"></div>
        <LoadingOverlay show />
      </div>
    {:else if tab === 'sales' && report}
      <div class="mt-2 flex min-h-0 flex-1 flex-col">
        <SalesBrowser
          highlight={untrack(() => highlightSale)}
          from={report.from}
          to={report.to}
          waiting={loading}
          deleted={showDeleted}
          refresh={salesRefresh}
          filter={appliedFilter}
          emptyText={showDeleted ? 'Tidak ada nota dihapus pada rentang ini.' : 'Tidak ada transaksi pada rentang ini.'}
          ondeleted={() => load(false)}
        />
      </div>
    {:else}
    <div class="relative mt-2 flex min-h-0 flex-1 flex-col">
    <div class="card min-h-40 flex-1 overflow-y-auto">
      {#if !report}
        <!-- first load: the overlay below says what is happening -->
      {:else if tab === 'period'}
        <table class="table">
          <thead class="sticky top-0">
            <tr>
              <SortHeader label="Periode" key="period" sort={periodSort} onsort={sortPeriods} />
              <SortHeader label="Transaksi" key="transactions" sort={periodSort} onsort={sortPeriods} align="right" firstDir="desc" />
              <SortHeader label="Tiket" key="ticketQty" sort={periodSort} onsort={sortPeriods} align="right" firstDir="desc" />
              <SortHeader label="Rp Tiket" key="ticketTotal" sort={periodSort} onsort={sortPeriods} align="right" firstDir="desc" />
              <SortHeader label="Topup" key="topupQty" sort={periodSort} onsort={sortPeriods} align="right" firstDir="desc" />
              <SortHeader label="Rp Topup" key="topupTotal" sort={periodSort} onsort={sortPeriods} align="right" firstDir="desc" />
              <SortHeader label="Total" key="total" sort={periodSort} onsort={sortPeriods} align="right" firstDir="desc" />
            </tr>
          </thead>
          <tbody>
            {#each periodRows as r}
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
          {#if report.rows.length > 0}
            <tfoot>
              <tr class="font-semibold">
                <td>Total</td>
                <td class="text-right">{report.transactions}</td>
                <td class="text-right">{report.ticketQty}</td>
                <td class="text-right">{formatRp(report.ticketTotal)}</td>
                <td class="text-right">{report.topupQty}</td>
                <td class="text-right">{formatRp(report.topupTotal)}</td>
                <td class="text-right">{formatRp(report.total)}</td>
              </tr>
            </tfoot>
          {/if}
        </table>
      {:else if tab === 'ticket'}
        <table class="table">
          <thead class="sticky top-0">
            <tr>
              <SortHeader label="Item" key="name" sort={itemSort} onsort={sortItems} />
              <SortHeader label="Terjual" key="qty" sort={itemSort} onsort={sortItems} align="right" firstDir="desc" />
              <SortHeader label="Total" key="total" sort={itemSort} onsort={sortItems} align="right" firstDir="desc" />
            </tr>
          </thead>
          <tbody>
            {#each itemPageRows as row, i (row.item.ticketId)}
              {#if i === 0 || row.first}
                <tr class="bg-slate-50"><td colspan="3" class="font-semibold text-brand-700">{row.group.label}{row.first ? '' : ' (lanjutan)'}</td></tr>
              {/if}
              <tr><td class="pl-8">{row.item.ticketName}</td><td class="text-right">{row.item.qty}</td><td class="text-right font-medium">{formatRp(row.item.total)}</td></tr>
              {#if row.last}
                <tr class="font-semibold"><td class="pl-8">Subtotal {row.group.label}</td><td class="text-right">{row.group.qty}</td><td class="text-right">{formatRp(row.group.total)}</td></tr>
              {/if}
            {:else}
              <tr><td colspan="3" class="py-10 text-center text-slate-500">Tidak ada penjualan pada rentang ini.</td></tr>
            {/each}
          </tbody>
          {#if itemRows.length > 0}
            <tfoot>
              <tr class="font-semibold"><td>Total</td><td class="text-right">{report.qty}</td><td class="text-right">{formatRp(report.total)}</td></tr>
            </tfoot>
          {/if}
        </table>
      {/if}
    </div>
    <LoadingOverlay show={loading} />
    </div>
    {#if !report}
      <!-- no pager before the first report -->
    {:else if tab === 'period'}
      <Pager page={periodPage} pageSize={tabPageSize} count={report.rows.length} ongoto={(p) => (periodPage = p)} />
    {:else if tab === 'ticket'}
      <Pager page={itemPage} pageSize={tabPageSize} count={itemRows.length} ongoto={(p) => (itemPage = p)} />
    {/if}
    {/if}
  {/if}
</div>

