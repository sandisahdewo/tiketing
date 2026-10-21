<script lang="ts">
  import { monthsLong, toISODate, today } from '../lib/date'

  // One month calendar of DateRangeInput, with its own ‹ › and a title that
  // switches to a month grid, then a year grid. It shows the chosen range in
  // blue and, while the second day is being picked, a grey preview.
  let {
    view = $bindable(),
    rangeFrom,
    rangeTo,
    previewFrom = '',
    previewTo = '',
    min = '',
    max = today(),
    minMonth = null,
    maxMonth = null,
    onpick,
    onhover
  }: {
    // the month on screen, as its first day
    view: Date
    rangeFrom: string
    rangeTo: string
    // days from the first pick to the one under the mouse ('' for none)
    previewFrom?: string
    previewTo?: string
    // days outside min..max cannot be picked (YYYY-MM-DD; '' for no limit)
    min?: string
    max?: string
    // the months this calendar may show (first days; null for no limit), so
    // the right calendar never shows a month before the left one
    minMonth?: Date | null
    maxMonth?: Date | null
    onpick: (iso: string) => void
    onhover?: (iso: string) => void
  } = $props()

  const weekdays = ['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min']
  const monthsShort = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']
  const yearsPerPage = 12

  let mode = $state<'days' | 'months' | 'years'>('days')
  let firstYear = $derived(view.getFullYear() - (view.getFullYear() % yearsPerPage))
  let now = new Date()

  let cells = $derived.by(() => {
    const start = new Date(view)
    start.setDate(1 - ((start.getDay() + 6) % 7))
    return Array.from({ length: 42 }, (_, i) => new Date(start.getFullYear(), start.getMonth(), start.getDate() + i))
  })

  function clamp(d: Date): Date {
    if (minMonth && d < minMonth) return minMonth
    if (maxMonth && d > maxMonth) return maxMonth
    return d
  }
  const monthAllowed = (y: number, m: number) => clamp(new Date(y, m, 1)).getTime() === new Date(y, m, 1).getTime()
  const yearAllowed = (y: number) => (!minMonth || y >= minMonth.getFullYear()) && (!maxMonth || y <= maxMonth.getFullYear())

  // ‹ › move by a month, a year or a page of years, stopping at the limits.
  function stepTarget(dir: 1 | -1): Date {
    const n = mode === 'days' ? 1 : mode === 'months' ? 12 : 12 * yearsPerPage
    return clamp(new Date(view.getFullYear(), view.getMonth() + dir * n, 1))
  }
  const canStep = (dir: 1 | -1) => stepTarget(dir).getTime() !== view.getTime()

  function step(dir: 1 | -1) {
    view = stepTarget(dir)
  }

  const allowed = (iso: string) => (!min || iso >= min) && (!max || iso <= max)

  function dayClass(d: Date, iso: string) {
    if (d.getMonth() !== view.getMonth()) return 'invisible'
    if (!allowed(iso)) return 'cursor-not-allowed text-slate-300'
    if (iso === rangeFrom || iso === rangeTo) return 'bg-brand-600 font-semibold text-white'
    // The mouse only greys the day; the chosen range stays as it is until a click.
    if (iso > rangeFrom && iso < rangeTo) return 'bg-brand-100 text-brand-800 hover:bg-slate-200'
    if (previewFrom && iso >= previewFrom && iso <= previewTo) return 'bg-slate-100 hover:bg-slate-200'
    if (iso === today()) return 'font-semibold text-brand-700 ring-1 ring-brand-500 ring-inset hover:bg-slate-200'
    return 'hover:bg-slate-200'
  }

  // Months ("YYYY-MM") and years ("YYYY") show the range like the days do.
  function spanClass(key: string, current: boolean) {
    const a = rangeFrom.slice(0, key.length)
    const b = rangeTo.slice(0, key.length)
    if (a && (key === a || key === b)) return 'bg-brand-600 font-semibold text-white'
    if (a && key > a && key < b) return 'bg-brand-100 text-brand-800 hover:bg-slate-200'
    if (current) return 'font-semibold text-brand-700 ring-1 ring-brand-500 ring-inset hover:bg-slate-200'
    return 'hover:bg-slate-200'
  }
  const monthKey = (y: number, m: number) => `${y}-${String(m + 1).padStart(2, '0')}`
</script>

<div class="flex w-64 flex-col">
  <div class="mb-2 flex items-center justify-between">
    <button type="button" class="btn btn-ghost px-2 py-1" onclick={() => step(-1)} disabled={!canStep(-1)} aria-label="Sebelumnya">‹</button>
    {#if mode === 'years'}
      <span class="px-2 py-1 text-sm font-semibold">{firstYear} – {firstYear + yearsPerPage - 1}</span>
    {:else}
      <button
        type="button"
        class="btn btn-ghost px-2 py-1 text-sm font-semibold"
        onclick={() => (mode = mode === 'days' ? 'months' : 'years')}
        title={mode === 'days' ? 'Pilih bulan' : 'Pilih tahun'}
      >
        {mode === 'days' ? `${monthsLong[view.getMonth()]} ${view.getFullYear()}` : view.getFullYear()}
        <span class="text-xs text-slate-400">▾</span>
      </button>
    {/if}
    <button type="button" class="btn btn-ghost px-2 py-1" onclick={() => step(1)} disabled={!canStep(1)} aria-label="Berikutnya">›</button>
  </div>

  {#if mode === 'days'}
    <div class="grid grid-cols-7 gap-y-0.5 text-center text-sm" role="presentation" onmouseleave={() => onhover?.('')}>
      {#each weekdays as w}
        <div class="py-1 text-xs font-medium text-slate-500">{w}</div>
      {/each}
      {#each cells as d (d.getTime())}
        {@const iso = toISODate(d)}
        <button
          type="button"
          class="rounded py-1.5 {dayClass(d, iso)}"
          disabled={d.getMonth() !== view.getMonth() || !allowed(iso)}
          onclick={() => onpick(iso)}
          onmouseenter={() => onhover?.(allowed(iso) ? iso : '')}
        >
          {d.getDate()}
        </button>
      {/each}
    </div>
  {:else}
    <div class="grid flex-1 grid-cols-3 content-start gap-1 text-center text-sm">
      {#if mode === 'months'}
        {#each monthsShort as name, i}
          <button
            type="button"
            class="rounded py-3 {monthAllowed(view.getFullYear(), i)
              ? spanClass(monthKey(view.getFullYear(), i), now.getFullYear() === view.getFullYear() && now.getMonth() === i)
              : 'cursor-not-allowed text-slate-300'}"
            disabled={!monthAllowed(view.getFullYear(), i)}
            onclick={() => {
              view = new Date(view.getFullYear(), i, 1)
              mode = 'days'
            }}
          >
            {name}
          </button>
        {/each}
      {:else}
        {#each Array.from({ length: yearsPerPage }, (_, i) => firstYear + i) as y}
          <button
            type="button"
            class="rounded py-3 {yearAllowed(y) ? spanClass(String(y), now.getFullYear() === y) : 'cursor-not-allowed text-slate-300'}"
            disabled={!yearAllowed(y)}
            onclick={() => {
              view = clamp(new Date(y, view.getMonth(), 1))
              mode = 'months'
            }}
          >
            {y}
          </button>
        {/each}
      {/if}
    </div>
  {/if}

</div>
