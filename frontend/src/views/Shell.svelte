<script lang="ts">
  import { untrack } from 'svelte'
  import Pos from './Pos.svelte'
  import Tickets from './Tickets.svelte'
  import Counters from './Counters.svelte'
  import ShiftTypes from './ShiftTypes.svelte'
  import Users from './Users.svelte'
  import Reports from './Reports.svelte'
  import Settings from './Settings.svelte'
  import Modal from '../components/Modal.svelte'
  import ShiftReportModal from '../components/ShiftReportModal.svelte'
  import { App } from '../lib/api'
  import { session, isAdmin } from '../lib/session.svelte'
  import { shift, refreshShift, clearShift, isOverdue } from '../lib/shift.svelte'
  import { correction, endCorrection } from '../lib/correction.svelte'
  import { clockText } from '../lib/date'

  let { onlogout, onreset }: { onlogout: () => void; onreset: () => void } = $props()

  type View = 'pos' | 'tickets' | 'counters' | 'shifts' | 'users' | 'reports' | 'settings'

  const nav: { view: View; label: string; admin: boolean }[] = [
    { view: 'pos', label: 'Kasir', admin: false },
    { view: 'tickets', label: 'Tiket', admin: true },
    { view: 'counters', label: 'Loket', admin: true },
    { view: 'shifts', label: 'Shift', admin: true },
    { view: 'users', label: 'Pengguna', admin: true },
    { view: 'reports', label: 'Laporan', admin: false },
    { view: 'settings', label: 'Pengaturan', admin: false }
  ]

  let view = $state<View>('pos')
  // Set when the cashier screen opens Laporan on today's transactions, and
  // the sale to put on top when it opens it from a finished payment.
  let reportsToday = $state(false)
  let reportsSale = $state<number | null>(null)

  function show(v: View, today = false, sale: number | null = null) {
    reportsToday = today
    reportsSale = sale
    view = v
  }
  // Correcting a sale happens at the cashier screen, wherever it started.
  $effect(() => {
    if (correction.sale) untrack(() => show('pos'))
  })

  let now = $state(new Date())

  $effect(() => {
    const timer = setInterval(() => (now = new Date()), 1000)
    return () => clearInterval(timer)
  })

  // The server knows this PC's counter; it may have changed since last time.
  $effect(() => {
    App.MyCounter()
      .then(() => App.GetAppInfo())
      .then((info) => (session.info = info))
      .catch(() => {})
  })

  // The user's open shift, shown in the header; past its schedule it turns
  // amber as a hint to close it.
  let own = $derived(shift.status?.own ?? null)
  let overdue = $derived(isOverdue(own?.shift, now))
  let shiftShown = $state<number | null>(null)

  let loggingOut = $state(false)
  // Logging out leaves the shift open (the cashier may come back to it), so
  // a cashier still on a shift is asked first.
  let confirmLogout = $state(false)

  async function logout() {
    if (loggingOut) return
    const status = await refreshShift().catch(() => null)
    if (status?.own && !confirmLogout) {
      confirmLogout = true
      return
    }
    loggingOut = true
    confirmLogout = false
    await App.Logout()
    clearShift()
    endCorrection()
    session.user = null
    onlogout()
  }

  function closeShiftFirst() {
    confirmLogout = false
    if (own) shiftShown = own.shift.id
  }
</script>

<div class="flex h-full flex-col">
  <header class="flex items-center gap-6 bg-brand-600 px-4 text-white shadow">
    <div class="py-3 text-lg font-bold tracking-wide">TIKET</div>
    <nav class="flex self-stretch">
      {#each nav.filter((n) => !n.admin || isAdmin()) as n}
        <button
          class="border-b-4 px-4 text-sm font-semibold transition {view === n.view ? 'border-white bg-brand-700' : 'border-transparent hover:bg-brand-700/60'}"
          onclick={() => show(n.view)}
        >
          {n.label}
        </button>
      {/each}
    </nav>
    <div class="ml-auto flex items-center gap-5 text-sm">
      <span class="tabular-nums">{clockText(now)}</span>
      {#if session.info?.terminal}
        <span class="rounded bg-brand-800/50 px-2 py-1">{session.info.terminal}</span>
      {:else}
        <span class="rounded bg-amber-400 px-2 py-1 font-semibold text-amber-950">Belum ada loket</span>
      {/if}
      {#if own}
        <button
          class="rounded px-2 py-1 {overdue ? 'bg-amber-400 font-semibold text-amber-950' : 'bg-brand-800/50 hover:bg-brand-800'}"
          title={overdue ? 'Jadwal shift sudah selesai. Klik untuk tutup shift.' : 'Lihat atau tutup shift'}
          onclick={() => (shiftShown = own.shift.id)}
        >
          Shift {own.shift.name}{overdue ? ` · lewat ${own.shift.endTime}` : ''}
        </button>
      {/if}
      <span>
        <b>{session.user?.fullName}</b>
        <span class="opacity-75">({session.user?.role === 'admin' ? 'Admin' : 'Kasir'})</span>
      </span>
      <button class="rounded px-3 py-1.5 font-semibold hover:bg-brand-700" onclick={logout} disabled={loggingOut}>Keluar</button>
    </div>
  </header>

  <main class="min-h-0 flex-1">
    {#if view === 'pos'}
      <Pos onhistory={() => show('reports', true)} onshowsale={(id) => show('reports', true, id)} onlogout={logout} />
    {:else if view === 'tickets'}
      <Tickets />
    {:else if view === 'counters'}
      <Counters />
    {:else if view === 'shifts'}
      <ShiftTypes />
    {:else if view === 'users'}
      <Users />
    {:else if view === 'reports'}
      <Reports todaySales={reportsToday} highlightSale={reportsSale} />
    {:else}
      <Settings {onreset} />
    {/if}
  </main>
</div>

{#if shiftShown !== null}
  <ShiftReportModal shiftId={shiftShown} onclose={() => (shiftShown = null)} onchange={() => refreshShift().catch(() => {})} />
{/if}

{#if confirmLogout && own}
  <Modal title="Shift Masih Terbuka" onclose={() => (confirmLogout = false)} width="max-w-md">
    <p>
      Shift <b>{own.shift.name}</b> Anda di {own.shift.counterName} masih terbuka. Kalau sudah selesai bertugas, tutup shift dulu.
      Kalau keluar saja, shift tetap terbuka dan bisa dilanjutkan saat Anda login lagi.
    </p>
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (confirmLogout = false)}>Batal</button>
      <button class="btn btn-secondary" onclick={logout} disabled={loggingOut}>Keluar saja</button>
      <button class="btn btn-primary" onclick={closeShiftFirst}>Tutup Shift</button>
    {/snippet}
  </Modal>
{/if}
