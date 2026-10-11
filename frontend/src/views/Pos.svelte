<script lang="ts">
  import PaymentModal from './PaymentModal.svelte'
  import HistoryModal from './HistoryModal.svelte'
  import ReceiptModal from '../components/ReceiptModal.svelte'
  import { App, model, handleError } from '../lib/api'
  import { formatMoney, formatRp } from '../lib/money'

  type CartItem = { ticket: model.Ticket; qty: number }

  let tickets = $state<model.Ticket[]>([])
  let search = $state('')
  let loading = $state(true)
  let cart = $state<CartItem[]>([])
  let paying = $state(false)
  let saving = $state(false)
  let showHistory = $state(false)
  let lastSaleId = $state<number | null>(null)
  let searchEl: HTMLInputElement

  let filtered = $derived.by(() => {
    const q = search.trim().toLowerCase()
    return q ? tickets.filter((t) => t.name.toLowerCase().includes(q)) : tickets
  })
  let total = $derived(cart.reduce((sum, c) => sum + c.ticket.price * c.qty, 0))
  let totalQty = $derived(cart.reduce((sum, c) => sum + c.qty, 0))

  async function load() {
    loading = true
    try {
      tickets = await App.ListTickets('', false)
      cart = cart.filter((c) => tickets.some((t) => t.id === c.ticket.id))
    } catch (e) {
      handleError(e)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    load()
  })

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

  function onSearchKey(e: KeyboardEvent) {
    if (e.key === 'Enter' && filtered.length === 1) {
      add(filtered[0])
      search = ''
    }
  }

  async function pay(paid: number) {
    saving = true
    try {
      const items = cart.map((c) => model.SaleItemInput.createFrom({ ticketId: c.ticket.id, qty: c.qty }))
      const sale = await App.CreateSale(items, paid)
      cart = []
      paying = false
      lastSaleId = sale.id
    } catch (e) {
      handleError(e)
      if (String(e).includes('Tiket tidak ditemukan')) load()
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
    if (paying || lastSaleId !== null || showHistory) return
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

    <h2 class="mt-4 mb-2 font-semibold text-brand-700">Daftar Tiket</h2>
    <div class="card min-h-0 flex-1 overflow-y-auto p-3">
      {#if loading}
        <p class="py-10 text-center text-slate-500">Memuat tiket…</p>
      {:else if filtered.length === 0}
        <p class="py-10 text-center text-slate-500">{tickets.length === 0 ? 'Belum ada tiket. Admin bisa menambah tiket di menu Tiket.' : 'Tiket tidak ditemukan.'}</p>
      {:else}
        <div class="grid grid-cols-3 gap-3 xl:grid-cols-4 2xl:grid-cols-5">
          {#each filtered as t (t.id)}
            <button
              class="flex min-h-24 flex-col items-center justify-center rounded-md bg-amber-500 px-3 py-4 text-center text-white shadow-sm transition hover:bg-amber-600 active:scale-[0.98]"
              onclick={() => add(t)}
            >
              <span class="font-semibold break-words uppercase">{t.name}</span>
              <span class="mt-1 text-sm opacity-90">({formatMoney(t.price)})</span>
            </button>
          {/each}
        </div>
      {/if}
    </div>
  </section>

  <aside class="card flex w-[420px] shrink-0 flex-col">
    <div class="flex items-center justify-between border-b border-slate-200 px-4 py-3">
      <h2 class="font-semibold">Pesanan</h2>
      <button class="btn btn-ghost px-2 py-1 text-brand-700" onclick={() => (showHistory = true)}>Riwayat hari ini</button>
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
                    class="h-8 w-16 border-y border-slate-300 text-center text-sm outline-none"
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
          <div class="text-sm text-slate-500">Total ({totalQty} tiket)</div>
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
  <ReceiptModal saleId={lastSaleId} title="Pembayaran Berhasil" newTransaction onclose={newTransaction} />
{/if}

{#if showHistory}
  <HistoryModal onclose={() => (showHistory = false)} />
{/if}
