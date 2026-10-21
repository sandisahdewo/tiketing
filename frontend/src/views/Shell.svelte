<script lang="ts">
  import Pos from './Pos.svelte'
  import Tickets from './Tickets.svelte'
  import Counters from './Counters.svelte'
  import Users from './Users.svelte'
  import Reports from './Reports.svelte'
  import Settings from './Settings.svelte'
  import UpdateBanner from './UpdateBanner.svelte'
  import { App } from '../lib/api'
  import { session, isAdmin } from '../lib/session.svelte'
  import { updateState } from '../lib/update.svelte'
  import { clockText } from '../lib/date'

  let { onlogout, onreset }: { onlogout: () => void; onreset: () => void } = $props()

  type View = 'pos' | 'tickets' | 'counters' | 'users' | 'reports' | 'settings'

  const nav: { view: View; label: string; admin: boolean }[] = [
    { view: 'pos', label: 'Kasir', admin: false },
    { view: 'tickets', label: 'Tiket', admin: true },
    { view: 'counters', label: 'Loket', admin: true },
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

  $effect(() => {
    App.CheckUpdate()
      .then((u) => (updateState.info = u))
      .catch(() => {})
  })

  let loggingOut = $state(false)

  async function logout() {
    if (loggingOut) return
    loggingOut = true
    await App.Logout()
    session.user = null
    onlogout()
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
      <span>
        <b>{session.user?.fullName}</b>
        <span class="opacity-75">({session.user?.role === 'admin' ? 'Admin' : 'Kasir'})</span>
      </span>
      <button class="rounded px-3 py-1.5 font-semibold hover:bg-brand-700" onclick={logout} disabled={loggingOut}>Keluar</button>
    </div>
  </header>

  <UpdateBanner />

  <main class="min-h-0 flex-1">
    {#if view === 'pos'}
      <Pos onhistory={() => show('reports', true)} onshowsale={(id) => show('reports', true, id)} />
    {:else if view === 'tickets'}
      <Tickets />
    {:else if view === 'counters'}
      <Counters />
    {:else if view === 'users'}
      <Users />
    {:else if view === 'reports'}
      <Reports todaySales={reportsToday} highlightSale={reportsSale} />
    {:else}
      <Settings {onreset} />
    {/if}
  </main>
</div>
