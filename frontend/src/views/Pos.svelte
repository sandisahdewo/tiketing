<script lang="ts">
  import { untrack } from 'svelte'
  import PaymentModal from './PaymentModal.svelte'
  import ReceiptModal from '../components/ReceiptModal.svelte'
  import LoadingOverlay from '../components/LoadingOverlay.svelte'
  import ShiftReportModal from '../components/ShiftReportModal.svelte'
  import CorrectionModal from '../components/CorrectionModal.svelte'
  import Field from '../components/Field.svelte'
  import FormAlert from '../components/FormAlert.svelte'
  import MoneyInput from '../components/MoneyInput.svelte'
  import { session, isAdmin } from '../lib/session.svelte'
  import { App, model, handleError, errorMessage } from '../lib/api'
  import { formatMoney, formatRp } from '../lib/money'
  import { formatDateTime, today } from '../lib/date'
  import { colorClass } from '../lib/colors'
  import { FormState } from '../lib/form.svelte'
  import { shift, refreshShift, isOverdue, shiftHours } from '../lib/shift.svelte'
  import { correction, endCorrection } from '../lib/correction.svelte'
  import { toast } from '../lib/toast.svelte'

  let { onhistory, onshowsale, onlogout }: { onhistory: () => void; onshowsale: (id: number) => void; onlogout: () => void } = $props()

  // --- shift: nothing is sold without an open shift of this user at this
  // PC's counter. Until there is one the screen says what to do instead.
  let status = $derived(shift.status)
  let own = $derived(status?.own ?? null)
  let gate = $derived.by((): 'loading' | 'elsewhere' | 'nocounter' | 'blocked' | 'open' | 'ready' => {
    if (!status) return 'loading'
    // A correction going into the corrected sale's open shift needs no shift here.
    if (correction.sale && correction.shift) return 'ready'
    if (own && own.shift.counterId !== status.counterId) return 'elsewhere'
    if (!status.counterId) return 'nocounter'
    if (status.blocking) return 'blocked'
    return own ? 'ready' : 'open'
  })

  async function reloadShift() {
    try {
      await refreshShift()
    } catch (e) {
      handleError(e)
    }
  }

  $effect(() => {
    reloadShift()
  })

  // Past its scheduled end a shift is only suggested to close; the check
  // follows the clock.
  let now = $state(new Date())
  $effect(() => {
    const timer = setInterval(() => (now = new Date()), 1000)
    return () => clearInterval(timer)
  })
  let overdue = $derived(isOverdue(own?.shift, now))
  let scheduledEnd = $derived(
    own ? (own.shift.scheduledEndAt.slice(0, 10) === today() ? `pukul ${own.shift.endTime}` : formatDateTime(own.shift.scheduledEndAt)) : ''
  )

  // The shift shown in the report modal (to close it or see it).
  let shiftShown = $state<number | null>(null)

  let typeId = $state(0)
  let openingCash = $state<number | null>(null)
  let openingInvalid = $state(false)
  let opening = $state(false)
  const openForm = new FormState(() => ({
    type: status?.types.some((t) => t.id === typeId) ? undefined : 'Pilih shift yang dibuka',
    cash: openingInvalid ? 'Modal awal tidak valid' : openingCash == null ? 'Modal awal wajib diisi' : undefined
  }))

  // Start on the suggested shift whenever the choices change.
  $effect(() => {
    const st = shift.status
    if (st && !st.types.some((t) => t.id === typeId)) typeId = st.suggestedTypeId
  })

  async function openShift(e: SubmitEvent) {
    e.preventDefault()
    if (opening || !openForm.check()) return
    opening = true
    try {
      const s = await App.OpenShift(typeId, openingCash!)
      toast(`Shift ${s.name} dibuka`)
      openingCash = null
      openForm.reset()
      await refreshShift()
      searchEl?.focus()
    } catch (err) {
      openForm.fail(errorMessage(err), { cash: /modal/i, type: /pilih shift|non-aktif/i })
      refreshShift().catch(() => {})
    } finally {
      opening = false
    }
  }

  // --- correcting a sale: its items fill the cart, at their price now, and
  // paying saves the new sale in its place.
  let correctingSale = $state<model.Sale | null>(null)

  $effect(() => {
    const sale = correction.sale
    if (sale) untrack(() => fillCart(sale))
  })

  async function fillCart(sale: model.Sale) {
    try {
      const tickets = new Map((await App.ListTickets('', false, true)).map((t) => [t.id, t]))
      const missing: string[] = []
      cart = []
      for (const it of sale.items) {
        const t = tickets.get(it.ticketId)
        if (t) cart.push({ ticket: t, qty: it.qty })
        else missing.push(it.ticketName)
      }
      if (missing.length > 0) toast(`Tidak bisa dijual lagi (non-aktif atau dihapus): ${missing.join(', ')}`, 'error')
    } catch (e) {
      handleError(e)
    }
  }

  function cancelCorrection() {
    endCorrection()
    cart = []
  }

  // Right after paying: the mistake is fixed on the sale just made.
  async function correctLast() {
    const id = lastSaleId
    newTransaction()
    if (id === null) return
    try {
      correctingSale = await App.GetSale(id)
    } catch (e) {
      handleError(e)
    }
  }

  type CartItem = { ticket: model.Ticket; qty: number }

  // Products load page by page as the list is scrolled; search runs on the server.
  const pageSize = 60

  let tickets = $state<model.Ticket[]>([])
  let count = $state(0)
  let page = $state(0)
  let search = $state('')
  let appliedSearch = $state('')
  let loading = $state(true)
  let loadingMore = $state(false)
  let sentinelVisible = $state(false)
  let requestSeq = 0
  let cart = $state<CartItem[]>([])
  let paying = $state(false)
  let saving = $state(false)
  let lastSaleId = $state<number | null>(null)
  let searchEl: HTMLInputElement

  let hasMore = $derived(tickets.length < count)
  let sections = $derived(
    [
      { title: 'Daftar Tiket', items: tickets.filter((t) => t.category !== 'topup') },
      { title: 'Topup Card', items: tickets.filter((t) => t.category === 'topup') }
    ].filter((s) => s.items.length > 0)
  )
  let total = $derived(cart.reduce((sum, c) => sum + c.ticket.price * c.qty, 0))
  let totalQty = $derived(cart.reduce((sum, c) => sum + c.qty, 0))

  function fetchPage(p: number) {
    return App.SearchTickets(model.TicketQuery.createFrom({ search: appliedSearch, status: 'active', page: p, pageSize }))
  }

  // load starts the list again from the first page.
  async function load() {
    const seq = ++requestSeq
    loading = true
    try {
      const out = await fetchPage(1)
      if (seq !== requestSeq) return
      tickets = out.tickets
      count = out.count
      page = 1
    } catch (e) {
      if (seq === requestSeq) handleError(e)
    } finally {
      if (seq === requestSeq) loading = false
    }
  }

  async function loadMore() {
    const seq = requestSeq
    loadingMore = true
    try {
      const out = await fetchPage(page + 1)
      if (seq !== requestSeq) return
      const seen = new Set(tickets.map((t) => t.id))
      tickets = [...tickets, ...out.tickets.filter((t) => !seen.has(t.id))]
      count = out.count
      page += 1
    } catch (e) {
      handleError(e)
    } finally {
      loadingMore = false
    }
  }

  $effect(() => {
    const value = search.trim()
    const timer = setTimeout(() => (appliedSearch = value), 250)
    return () => clearTimeout(timer)
  })

  $effect(() => {
    appliedSearch
    untrack(load)
  })

  // Keep loading while the end of the list is on screen.
  $effect(() => {
    if (sentinelVisible && hasMore && !loading && !loadingMore) loadMore()
  })

  function watchEnd(node: HTMLElement) {
    const observer = new IntersectionObserver(([entry]) => (sentinelVisible = entry.isIntersecting), {
      root: node.closest('.overflow-y-auto'),
      rootMargin: '300px'
    })
    observer.observe(node)
    return { destroy: () => observer.disconnect() }
  }

  function add(t: model.Ticket) {
    const item = cart.find((c) => c.ticket.id === t.id)
    if (item) item.qty += 1
    else cart.push({ ticket: t, qty: 1 })
  }

  function setQty(item: CartItem, qty: number) {
    if (!Number.isFinite(qty) || qty < 1) {
      cart = cart.filter((c) => c !== item)
      return
    }
    item.qty = Math.min(Math.floor(qty), 10000)
  }

  // Enter adds the product when the search leaves exactly one.
  async function onSearchKey(e: KeyboardEvent) {
    if (e.key !== 'Enter') return
    if (appliedSearch !== search.trim()) {
      appliedSearch = search.trim()
      await load()
    }
    if (count === 1 && tickets.length === 1) {
      add(tickets[0])
      search = ''
    }
  }

  async function pay(paid: number) {
    // Guards against a second submit before the first sale is saved.
    if (saving) return
    saving = true
    try {
      const items = cart.map((c) => model.SaleItemInput.createFrom({ ticketId: c.ticket.id, qty: c.qty }))
      const fixing = correction.sale
      const sale = fixing ? await App.CorrectSale(fixing.id, correction.reason, items, paid) : await App.CreateSale(items, paid)
      if (fixing) {
        toast(`Nota ${fixing.invoiceNo} diganti ${sale.invoiceNo}`)
        endCorrection()
      }
      cart = []
      paying = false
      lastSaleId = sale.id
    } catch (e) {
      const msg = handleError(e)
      if (msg.includes('Tiket tidak ditemukan') || msg.includes('non-aktif')) load()
      // The shift was closed meanwhile, e.g. by an admin on another PC.
      if (/shift/i.test(msg)) {
        paying = false
        refreshShift().catch(() => {})
      }
    } finally {
      saving = false
    }
  }

  function newTransaction() {
    lastSaleId = null
    search = ''
    searchEl?.focus()
  }

  function onkeydown(e: KeyboardEvent) {
    if (gate !== 'ready' || shiftShown !== null || correctingSale !== null || paying || lastSaleId !== null) return
    if (e.key === 'F2') {
      e.preventDefault()
      searchEl?.focus()
    } else if (e.key === 'F8' && cart.length > 0) {
      e.preventDefault()
      paying = true
    }
  }
</script>

<svelte:window {onkeydown} />

<div class="relative flex h-full gap-4 p-4">
  <section class="flex min-w-0 flex-1 flex-col" inert={gate !== 'ready'}>
    {#if own && gate === 'ready'}
      <div class="mb-3 flex items-center justify-between gap-3 rounded-md border px-4 py-2 text-sm {overdue ? 'border-amber-200 bg-amber-50 text-amber-900' : 'border-slate-200 bg-white text-slate-700'}">
        <div>
          {#if overdue}
            <b>Jadwal shift {own.shift.name} sudah selesai {scheduledEnd}.</b> Tutup shift kalau Anda sudah selesai bertugas.
          {:else}
            <b>Shift {own.shift.name}</b> · {shiftHours(own.shift)} · dibuka {formatDateTime(own.shift.openedAt)} · modal awal {formatRp(own.shift.openingCash)}
          {/if}
        </div>
        <button class="btn {overdue ? 'btn-primary' : 'btn-secondary'} shrink-0 px-3 py-1" onclick={() => (shiftShown = own.shift.id)}>Tutup Shift</button>
      </div>
    {/if}
    {#if correction.sale}
      {@const diff = total - correction.sale.total}
      <div class="mb-3 flex items-center justify-between gap-3 rounded-md border border-sky-200 bg-sky-50 px-4 py-2 text-sm text-sky-900">
        <div>
          <b>Koreksi nota {correction.sale.invoiceNo}</b> ({formatRp(correction.sale.total)}) · alasan: {correction.reason}.
          Perbaiki item lalu bayar total baru; nota lama dihapus saat nota baru dibayar.
          {#if correction.shift}Dicatat di shift {correction.shift.name} {correction.shift.cashierName} ({correction.shift.counterName}).{/if}
          {#if cart.length > 0}
            <b>{diff < 0 ? `Kembalikan ${formatRp(-diff)} ke pelanggan.` : diff > 0 ? `Pelanggan menambah ${formatRp(diff)}.` : 'Total sama dengan nota lama.'}</b>
          {/if}
        </div>
        <button class="btn btn-secondary shrink-0 px-3 py-1" onclick={cancelCorrection}>Batal koreksi</button>
      </div>
    {/if}
    <div class="flex gap-2">
      <div class="relative flex-1">
        <svg class="pointer-events-none absolute top-1/2 left-3 h-5 w-5 -translate-y-1/2 text-slate-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
        <input
          bind:this={searchEl}
          class="input py-2.5 pl-10 text-base"
          placeholder="Cari tiket… (F2)"
          bind:value={search}
          onkeydown={onSearchKey}
        />
      </div>
      <button class="btn btn-primary px-4" onclick={load} title="Muat ulang daftar tiket" aria-label="Muat ulang">
        <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-3-6.7L21 8" /><path d="M21 3v5h-5" /></svg>
      </button>
    </div>

    <div class="relative mt-4 flex min-h-0 flex-1 flex-col">
    <div class="card min-h-40 flex-1 overflow-y-auto p-3">
      {#if tickets.length === 0}
        {#if !loading}
          <p class="py-10 text-center text-slate-500">{appliedSearch ? 'Tiket tidak ditemukan.' : 'Belum ada tiket. Admin bisa menambah tiket di menu Tiket.'}</p>
        {/if}
      {:else}
        {#each sections as section, i (section.title)}
          <h2 class="mb-2 font-semibold text-brand-700 {i > 0 ? 'mt-5' : ''}">{section.title}</h2>
          <div class="grid grid-cols-3 gap-3 xl:grid-cols-4 2xl:grid-cols-5">
            {#each section.items as t (t.id)}
              <!-- content-visibility: tiles off screen are not laid out or painted until scrolled to -->
              <button
                class="flex min-h-24 flex-col items-center justify-center rounded-md px-3 py-4 text-center text-white shadow-sm transition-transform [contain-intrinsic-size:auto_6rem] [content-visibility:auto] active:scale-[0.98] {colorClass(t.color)}"
                onclick={() => add(t)}
              >
                <span class="font-semibold break-words uppercase">{t.name}</span>
                <span class="mt-1 text-sm opacity-90">({formatMoney(t.price)})</span>
              </button>
            {/each}
          </div>
        {/each}
        <div use:watchEnd class="flex items-center justify-center gap-2 py-3 text-sm text-slate-400">
          {#if loadingMore}
            <span class="h-4 w-4 animate-spin rounded-full border-2 border-slate-300 border-t-brand-600"></span>
            Memuat tiket berikutnya…
          {:else if hasMore}
            {tickets.length} dari {count} tiket
          {/if}
        </div>
      {/if}
    </div>
    <LoadingOverlay show={loading} text="Memuat tiket…" />
    </div>
  </section>

  <aside class="card flex w-[420px] shrink-0 flex-col" inert={gate !== 'ready'}>
    <div class="flex items-center justify-between border-b border-slate-200 px-4 py-3">
      <h2 class="font-semibold">Pesanan</h2>
      <button class="btn btn-ghost px-2 py-1 text-brand-700" onclick={onhistory}>Riwayat hari ini</button>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto">
      {#if cart.length === 0}
        <p class="py-16 text-center text-slate-400">Belum ada item</p>
      {:else}
        <ul class="divide-y divide-slate-100">
          {#each cart as item (item.ticket.id)}
            <li class="px-4 py-3">
              <div class="flex items-start justify-between gap-2">
                <div class="min-w-0">
                  <div class="font-medium break-words">{item.ticket.name}</div>
                  <div class="text-sm text-slate-500">{formatRp(item.ticket.price)}</div>
                </div>
                <button class="text-slate-400 hover:text-red-600" onclick={() => setQty(item, 0)} aria-label="Hapus item">✕</button>
              </div>
              <div class="mt-2 flex items-center justify-between">
                <div class="flex items-center">
                  <button class="h-8 w-8 rounded-l-md border border-slate-300 text-lg hover:bg-slate-50" onclick={() => setQty(item, item.qty - 1)}>−</button>
                  <input
                    class="h-8 w-16 [appearance:textfield] border-y border-slate-300 text-center text-sm outline-none [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                    type="number"
                    min="1"
                    value={item.qty}
                    onchange={(e) => setQty(item, Number(e.currentTarget.value))}
                  />
                  <button class="h-8 w-8 rounded-r-md border border-slate-300 text-lg hover:bg-slate-50" onclick={() => setQty(item, item.qty + 1)}>+</button>
                </div>
                <div class="font-semibold">{formatRp(item.ticket.price * item.qty)}</div>
              </div>
            </li>
          {/each}
        </ul>
      {/if}
    </div>

    <div class="border-t border-slate-200 p-4">
      <div class="flex items-end justify-between">
        <div>
          <div class="text-sm text-slate-500">Total ({totalQty} item)</div>
          <div class="text-3xl font-bold text-brand-700">{formatRp(total)}</div>
        </div>
      </div>
      <div class="mt-4 grid grid-cols-3 gap-2">
        <button class="btn btn-secondary py-3" onclick={cancelCorrection} disabled={cart.length === 0 && !correction.sale}>Batal</button>
        <button class="btn btn-primary col-span-2 py-3 text-base" onclick={() => (paying = true)} disabled={cart.length === 0}>Bayar (F8)</button>
      </div>
    </div>
  </aside>

  <!-- Until this user has a shift open at this PC's counter, the screen says what to do instead. -->
  {#if gate !== 'ready'}
    <div class="absolute inset-0 z-10 flex items-center justify-center overflow-y-auto bg-slate-100 p-6">
      {#if gate === 'loading' || !status}
        <LoadingOverlay show text="Memuat shift…" />
      {:else if gate === 'elsewhere' && own}
        <div class="card w-full max-w-lg p-6">
          <h2 class="text-xl font-semibold">Shift Anda masih terbuka di {own.shift.counterName}</h2>
          <p class="mt-2 text-slate-600">
            Shift <b>{own.shift.name}</b> dibuka {formatDateTime(own.shift.openedAt)} di <b>{own.shift.counterName}</b>. Untuk bertransaksi
            {status.counterName ? `di ${status.counterName}` : 'di komputer ini'}, tutup shift itu dulu. Hitung uang di laci {own.shift.counterName} sebelum menutupnya.
          </p>
          <div class="mt-5 flex justify-end gap-2">
            <button class="btn btn-secondary" onclick={reloadShift}>Muat ulang</button>
            <button class="btn btn-primary" onclick={() => (shiftShown = own.shift.id)}>Tutup Shift</button>
          </div>
        </div>
      {:else if gate === 'nocounter'}
        <div class="card w-full max-w-lg p-6">
          <h2 class="text-xl font-semibold">Komputer ini belum punya loket</h2>
          <p class="mt-2 text-slate-600">
            Komputer ini belum bisa bertransaksi.
            {isAdmin() ? 'Pilih loket di menu Pengaturan atau Loket.' : 'Minta admin memilih loket untuk komputer ini.'}
          </p>
          <div class="mt-5 flex justify-end"><button class="btn btn-secondary" onclick={reloadShift}>Muat ulang</button></div>
        </div>
      {:else if gate === 'blocked' && status.blocking}
        {@const b = status.blocking}
        <div class="card w-full max-w-lg p-6">
          <h2 class="text-xl font-semibold">{status.counterName} masih dipakai shift lain</h2>
          <p class="mt-2 text-slate-600">
            Shift <b>{b.name}</b> milik <b>{b.cashierName}</b> (dibuka {formatDateTime(b.openedAt)}) belum ditutup. Shift itu harus ditutup oleh
            {b.cashierName} atau admin sebelum Anda bisa membuka shift di loket ini.
          </p>
          <div class="mt-5 flex justify-end gap-2">
            <button class="btn btn-secondary" onclick={reloadShift}>Muat ulang</button>
            <button class="btn btn-secondary" onclick={onlogout}>Keluar</button>
            {#if isAdmin()}
              <button class="btn btn-primary" onclick={() => (shiftShown = b.id)}>Tutup Shift Ini</button>
            {/if}
          </div>
        </div>
      {:else}
        <form class="card w-full max-w-xl p-6" onsubmit={openShift} novalidate>
          <h2 class="text-xl font-semibold">Buka Shift</h2>
          <p class="text-sm text-slate-500">{session.user?.fullName} · {status.counterName}</p>
          {#if status.types.length === 0}
            <p class="mt-4 rounded-md bg-amber-50 px-4 py-3 text-sm text-amber-900">
              Belum ada shift aktif. {isAdmin() ? 'Tambahkan di menu Shift.' : 'Minta admin mengatur shift di menu Shift.'}
            </p>
          {:else}
            <div class="mt-4 space-y-4">
              <FormAlert message={openForm.general} />
              <Field label="Shift" form={openForm} name="type">
                <div class="grid grid-cols-3 gap-2">
                  {#each status.types as t (t.id)}
                    <button
                      type="button"
                      class="flex flex-col rounded-md border px-3 py-2 text-left transition {typeId === t.id ? 'border-brand-600 bg-brand-50 ring-2 ring-brand-600' : 'border-slate-300 hover:bg-slate-50'}"
                      onclick={() => {
                        typeId = t.id
                        openForm.clear('type')
                      }}
                    >
                      <div class="font-semibold">{t.name}</div>
                      <div class="text-xs text-slate-500 tabular-nums">{shiftHours(t)}</div>
                      {#if t.id === status.suggestedTypeId}<div class="mt-0.5 text-xs font-medium text-brand-700">Disarankan</div>{/if}
                    </button>
                  {/each}
                </div>
              </Field>
              <Field label="Modal awal (uang di laci)" id="opening-cash" form={openForm} name="cash" hint="Hitung uang di laci sebelum mulai. Isi 0 kalau laci kosong.">
                <MoneyInput id="opening-cash" bind:value={openingCash} bind:invalid={openingInvalid} autofocus large />
              </Field>
              <button class="btn btn-primary w-full py-3 text-base" disabled={opening}>{opening ? 'Membuka…' : 'Buka Shift'}</button>
            </div>
          {/if}
        </form>
      {/if}
    </div>
  {/if}
</div>

{#if correctingSale}
  <CorrectionModal sale={correctingSale} onclose={() => (correctingSale = null)} />
{/if}

{#if shiftShown !== null}
  <ShiftReportModal shiftId={shiftShown} onclose={() => (shiftShown = null)} onchange={reloadShift} />
{/if}

{#if paying}
  <PaymentModal {total} busy={saving} onpay={pay} onclose={() => (paying = false)} />
{/if}

{#if lastSaleId !== null}
  <ReceiptModal
    saleId={lastSaleId}
    title="Pembayaran Berhasil"
    newTransaction
    autoPrint
    onclose={newTransaction}
    oncorrect={isAdmin() ? correctLast : undefined}
    onview={() => {
      const id = lastSaleId
      newTransaction()
      if (id !== null) onshowsale(id)
    }}
  />
{/if}
