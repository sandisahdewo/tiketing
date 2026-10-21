<script lang="ts">
  import { untrack } from 'svelte'
  import PaymentModal from './PaymentModal.svelte'
  import ReceiptModal from '../components/ReceiptModal.svelte'
  import LoadingOverlay from '../components/LoadingOverlay.svelte'
  import { App, model, handleError } from '../lib/api'
  import { formatMoney, formatRp } from '../lib/money'
  import { colorClass } from '../lib/colors'

  let { onhistory, onshowsale }: { onhistory: () => void; onshowsale: (id: number) => void } = $props()

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
      const sale = await App.CreateSale(items, paid)
      cart = []
      paying = false
      lastSaleId = sale.id
    } catch (e) {
      handleError(e)
      if (String(e).includes('Tiket tidak ditemukan') || String(e).includes('non-aktif')) load()
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
    if (paying || lastSaleId !== null) return
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

<div class="flex h-full gap-4 p-4">
  <section class="flex min-w-0 flex-1 flex-col">
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

  <aside class="card flex w-[420px] shrink-0 flex-col">
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
        <button class="btn btn-secondary py-3" onclick={() => (cart = [])} disabled={cart.length === 0}>Batal</button>
        <button class="btn btn-primary col-span-2 py-3 text-base" onclick={() => (paying = true)} disabled={cart.length === 0}>Bayar (F8)</button>
      </div>
    </div>
  </aside>
</div>

{#if paying}
  <PaymentModal {total} busy={saving} onpay={pay} onclose={() => (paying = false)} />
{/if}

{#if lastSaleId !== null}
  <ReceiptModal
    saleId={lastSaleId}
    title="Pembayaran Berhasil"
    newTransaction
    onclose={newTransaction}
    onview={() => {
      const id = lastSaleId
      newTransaction()
      if (id !== null) onshowsale(id)
    }}
  />
{/if}
