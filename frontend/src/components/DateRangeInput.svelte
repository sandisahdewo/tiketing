<script lang="ts">
  import MonthCalendar from './MonthCalendar.svelte'
  import { formatDate, parseDate, toISODate } from '../lib/date'

  // A date range picker: quick ranges on the left (applied at once), then two
  // calendars, each with its own month. The first click on either picks the
  // first day, the second click the last one (the two are swapped when needed),
  // with a grey preview in between. Days after today cannot be picked.
  // Terapkan applies the range.
  let {
    from = $bindable(''),
    to = $bindable(''),
    id
  }: {
    // YYYY-MM-DD
    from?: string
    to?: string
    id?: string
  } = $props()

  function addDays(d: Date, n: number) {
    return new Date(d.getFullYear(), d.getMonth(), d.getDate() + n)
  }
  function startOfWeek(d: Date) {
    return addDays(d, -((d.getDay() + 6) % 7))
  }
  function monthOf(iso: string) {
    const d = iso ? parseDate(iso) : new Date()
    return new Date(d.getFullYear(), d.getMonth(), 1)
  }

  // Each quick range as [first day, last day].
  const presets: { label: string; range: () => [Date, Date] }[] = [
    { label: 'Hari ini', range: () => [new Date(), new Date()] },
    { label: 'Kemarin', range: () => [addDays(new Date(), -1), addDays(new Date(), -1)] },
    { label: 'Minggu ini', range: () => [startOfWeek(new Date()), new Date()] },
    { label: '7 hari terakhir', range: () => [addDays(new Date(), -6), new Date()] },
    { label: 'Minggu lalu', range: () => [addDays(startOfWeek(new Date()), -7), addDays(startOfWeek(new Date()), -1)] },
    { label: 'Bulan ini', range: () => [new Date(new Date().getFullYear(), new Date().getMonth(), 1), new Date()] },
    {
      label: 'Bulan lalu',
      range: () => [new Date(new Date().getFullYear(), new Date().getMonth() - 1, 1), new Date(new Date().getFullYear(), new Date().getMonth(), 0)]
    },
    { label: 'Tahun ini', range: () => [new Date(new Date().getFullYear(), 0, 1), new Date()] }
  ]
  let activePreset = $derived(
    presets.findIndex((p) => {
      const [a, b] = p.range()
      return toISODate(a) === from && toISODate(b) === to
    })
  )

  let open = $state(false)
  let root: HTMLDivElement
  let button: HTMLButtonElement
  let draftFrom = $state('')
  let draftTo = $state('')
  let viewFrom = $state(new Date())
  let viewTo = $state(new Date())
  // After the first click: waiting for the last day, previewing up to the mouse.
  let pending = $state(false)
  let hover = $state('')
  let previewFrom = $derived(pending && hover ? (hover < draftFrom ? hover : draftFrom) : '')
  let previewTo = $derived(pending && hover ? (hover < draftFrom ? draftFrom : hover) : '')
  // The quick range under the mouse (-1 for none).
  let hoverPreset = $state(-1)

  function rangeText(a: string, b: string) {
    return a === b ? formatDate(a) : `${formatDate(a)} – ${formatDate(b)}`
  }

  // The summary under the calendars: what a click on the thing under the
  // mouse would give (muted), or else the range picked so far.
  let summary = $derived.by((): { text: string; hint?: string; preview: boolean } => {
    if (hoverPreset >= 0) {
      const [a, b] = presets[hoverPreset].range()
      return { text: rangeText(toISODate(a), toISODate(b)), preview: true }
    }
    if (pending && hover) return { text: rangeText(previewFrom, previewTo), preview: true }
    if (pending) return { text: formatDate(draftFrom), hint: 'klik tanggal akhir', preview: false }
    if (hover) return { text: formatDate(hover), hint: 'klik tanggal akhir', preview: true }
    return { text: draftFrom ? rangeText(draftFrom, draftTo) : '', preview: false }
  })
  function pickDay(iso: string) {
    if (!pending) {
      draftFrom = draftTo = iso
      pending = true
    } else {
      if (iso < draftFrom) [draftFrom, draftTo] = [iso, draftFrom]
      else draftTo = iso
      pending = false
    }
    hover = ''
  }

  function show() {
    draftFrom = from
    draftTo = to || from
    pending = false
    hover = ''
    hoverPreset = -1
    viewTo = monthOf(draftTo)
    viewFrom = monthOf(draftFrom)
    // Both ends in one month: show the month before on the left rather than
    // the same month twice.
    if (viewFrom.getTime() === viewTo.getTime()) viewFrom = new Date(viewTo.getFullYear(), viewTo.getMonth() - 1, 1)
    open = true
  }

  function close() {
    open = false
    button?.focus()
  }

  function apply() {
    from = draftFrom
    to = draftTo
    close()
  }

  function pickPreset(i: number) {
    const [a, b] = presets[i].range()
    from = toISODate(a)
    to = toISODate(b)
    close()
  }

  // composedPath: switching a calendar's view removes the clicked button
  // before this runs.
  function onwindowclick(e: MouseEvent) {
    if (open && !e.composedPath().includes(root)) open = false
  }

  function onkeydown(e: KeyboardEvent) {
    if (open && e.key === 'Escape') {
      e.stopPropagation()
      close()
    }
  }

  let label = $derived(!from ? 'Pilih tanggal' : from === to ? formatDate(from) : `${formatDate(from)} – ${formatDate(to)}`)
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
    <span class="truncate {from ? '' : 'text-slate-400'}">{label}</span>
    <svg class="h-4 w-4 shrink-0 text-slate-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="5" width="18" height="16" rx="2" /><path d="M16 3v4M8 3v4M3 10h18" /></svg>
  </button>

  {#if open}
    <div class="absolute top-full left-0 z-30 mt-1 flex rounded-lg border border-slate-200 bg-white shadow-lg" role="dialog" aria-label="Pilih rentang tanggal">
      <div class="flex w-40 flex-col gap-1 border-r border-slate-100 p-2">
        {#each presets as p, i}
          <button
            type="button"
            class="rounded-md px-3 py-1.5 text-left text-sm {i === activePreset ? 'bg-brand-600 font-semibold text-white' : 'text-slate-700 hover:bg-slate-100'}"
            onclick={() => pickPreset(i)}
            onmouseenter={() => (hoverPreset = i)}
            onmouseleave={() => (hoverPreset = -1)}
          >
            {p.label}
          </button>
        {/each}
      </div>

      <div class="p-3">
        <div class="flex gap-4">
          <MonthCalendar bind:view={viewFrom} maxMonth={viewTo} rangeFrom={draftFrom} rangeTo={draftTo} {previewFrom} {previewTo} onpick={pickDay} onhover={(iso) => (hover = iso)} />
          <div class="w-px bg-slate-100"></div>
          <MonthCalendar bind:view={viewTo} minMonth={viewFrom} rangeFrom={draftFrom} rangeTo={draftTo} {previewFrom} {previewTo} onpick={pickDay} onhover={(iso) => (hover = iso)} />
        </div>

        <div class="mt-3 flex items-center justify-between gap-3 border-t border-slate-100 pt-3">
          <span class="text-sm">
            <span class="font-semibold {summary.preview ? 'text-slate-500' : ''}">{summary.text}</span>
            {#if summary.hint}– <span class="text-slate-500">{summary.hint}</span>{/if}
          </span>
          <div class="flex gap-2">
          <button type="button" class="btn btn-secondary px-3 py-1.5" onclick={close}>Batal</button>
          <button type="button" class="btn btn-primary px-3 py-1.5" onclick={apply} disabled={!draftFrom || !draftTo}>Terapkan</button>
          </div>
        </div>
      </div>
    </div>
  {/if}
</div>
