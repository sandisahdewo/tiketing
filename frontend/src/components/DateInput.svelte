<script lang="ts">
  import { formatDate, monthsLong, parseDate, toISODate, today } from '../lib/date'

  // An Indonesian date picker. The webview's built-in one follows the system
  // language (English, month/day/year) and closes when its month changes.
  // Browsing months here does not change the value; picking a day does.
  // Click the title to jump by month, then by year.
  let {
    value = $bindable(''),
    id,
    placeholder = 'Pilih tanggal'
  }: {
    // YYYY-MM-DD, or '' for none
    value?: string
    id?: string
    placeholder?: string
  } = $props()

  const weekdays = ['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min']
  const monthsShort = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']
  const yearsPerPage = 12

  let open = $state(false)
  let mode = $state<'days' | 'months' | 'years'>('days')
  let root: HTMLDivElement
  let button: HTMLButtonElement
  // The month on screen, as its first day.
  let view = $state(new Date())

  let selected = $derived(value ? parseDate(value) : null)
  let now = new Date()
  let firstYear = $derived(view.getFullYear() - (view.getFullYear() % yearsPerPage))

  function show() {
    const d = selected ?? new Date()
    view = new Date(d.getFullYear(), d.getMonth(), 1)
    mode = 'days'
    open = true
  }

  function close() {
    open = false
    button?.focus()
  }

  // ‹ › step by a month, a year or a page of years depending on the view.
  function step(dir: 1 | -1) {
    const months = mode === 'days' ? 1 : mode === 'months' ? 12 : 12 * yearsPerPage
    view = new Date(view.getFullYear(), view.getMonth() + dir * months, 1)
  }

  function pickDay(d: Date) {
    value = toISODate(d)
    close()
  }

  function pickMonth(m: number) {
    view = new Date(view.getFullYear(), m, 1)
    mode = 'days'
  }

  function pickYear(y: number) {
    view = new Date(y, view.getMonth(), 1)
    mode = 'months'
  }

  // Six weeks starting on the Monday on or before the 1st.
  let cells = $derived.by(() => {
    const start = new Date(view)
    start.setDate(1 - ((start.getDay() + 6) % 7))
    return Array.from({ length: 42 }, (_, i) => new Date(start.getFullYear(), start.getMonth(), start.getDate() + i))
  })

  let title = $derived(
    mode === 'days' ? `${monthsLong[view.getMonth()]} ${view.getFullYear()}` : mode === 'months' ? `${view.getFullYear()}` : `${firstYear} – ${firstYear + yearsPerPage - 1}`
  )
  const stepLabel = { days: 'bulan', months: 'tahun', years: '12 tahun' }

  // composedPath, not contains(): switching views removes the clicked button
  // before this runs, and a detached target would look like an outside click.
  function onwindowclick(e: MouseEvent) {
    if (open && !e.composedPath().includes(root)) open = false
  }

  function onkeydown(e: KeyboardEvent) {
    if (open && e.key === 'Escape') {
      e.stopPropagation()
      close()
    }
  }

  const cellClass = (isSelected: boolean, isCurrent: boolean, muted = false) =>
    isSelected
      ? 'bg-brand-600 font-semibold text-white'
      : isCurrent
        ? 'font-semibold text-brand-700 ring-1 ring-brand-500 ring-inset hover:bg-brand-50'
        : muted
          ? 'text-slate-300 hover:bg-slate-50'
          : 'hover:bg-slate-100'
</script>

<svelte:window onclick={onwindowclick} />

<div class="relative" bind:this={root} {onkeydown} role="presentation">
  <button
    bind:this={button}
    {id}
    type="button"
    class="input flex items-center justify-between gap-3 text-left"
    aria-haspopup="dialog"
    aria-expanded={open}
    onclick={() => (open ? close() : show())}
  >
    <span class={value ? '' : 'text-slate-400'}>{value ? formatDate(value) : placeholder}</span>
    <svg class="h-4 w-4 shrink-0 text-slate-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="5" width="18" height="16" rx="2" /><path d="M16 3v4M8 3v4M3 10h18" /></svg>
  </button>

  {#if open}
    <div class="absolute top-full left-0 z-30 mt-1 w-72 rounded-lg border border-slate-200 bg-white p-3 shadow-lg" role="dialog" aria-label="Pilih tanggal">
      <div class="mb-2 flex items-center justify-between">
        <button type="button" class="btn btn-ghost px-2 py-1" onclick={() => step(-1)} aria-label="{stepLabel[mode]} sebelumnya">‹</button>
        {#if mode === 'years'}
          <span class="px-2 py-1 text-sm font-semibold">{title}</span>
        {:else}
          <button
            type="button"
            class="btn btn-ghost px-2 py-1 text-sm font-semibold"
            onclick={() => (mode = mode === 'days' ? 'months' : 'years')}
            title={mode === 'days' ? 'Pilih bulan' : 'Pilih tahun'}
          >
            {title} <span class="text-xs text-slate-400">▾</span>
          </button>
        {/if}
        <button type="button" class="btn btn-ghost px-2 py-1" onclick={() => step(1)} aria-label="{stepLabel[mode]} berikutnya">›</button>
      </div>

      {#if mode === 'days'}
        <div class="grid grid-cols-7 gap-0.5 text-center text-sm">
          {#each weekdays as w}
            <div class="py-1 text-xs font-medium text-slate-500">{w}</div>
          {/each}
          {#each cells as d (d.getTime())}
            {@const iso = toISODate(d)}
            <button type="button" class="rounded py-1.5 {cellClass(iso === value, iso === today(), d.getMonth() !== view.getMonth())}" onclick={() => pickDay(d)}>
              {d.getDate()}
            </button>
          {/each}
        </div>
      {:else if mode === 'months'}
        <div class="grid grid-cols-3 gap-1 text-center text-sm">
          {#each monthsShort as m, i}
            <button
              type="button"
              class="rounded py-3 {cellClass(
                selected?.getFullYear() === view.getFullYear() && selected?.getMonth() === i,
                now.getFullYear() === view.getFullYear() && now.getMonth() === i
              )}"
              onclick={() => pickMonth(i)}
            >
              {m}
            </button>
          {/each}
        </div>
      {:else}
        <div class="grid grid-cols-3 gap-1 text-center text-sm">
          {#each Array.from({ length: yearsPerPage }, (_, i) => firstYear + i) as y}
            <button type="button" class="rounded py-3 {cellClass(selected?.getFullYear() === y, now.getFullYear() === y)}" onclick={() => pickYear(y)}>
              {y}
            </button>
          {/each}
        </div>
      {/if}

      <div class="mt-2 flex justify-end border-t border-slate-100 pt-2">
        <button type="button" class="btn btn-ghost px-2 py-1 text-sm text-brand-700" onclick={() => pickDay(new Date())}>Hari ini</button>
      </div>
    </div>
  {/if}
</div>
