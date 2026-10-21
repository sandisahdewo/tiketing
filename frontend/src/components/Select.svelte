<script lang="ts" generics="T extends string | number">
  // A dropdown drawn by the app. The webview's built-in one shows its option
  // list as a native menu that follows the desktop theme (black on a dark
  // desktop) and cannot be styled.
  let {
    value = $bindable(),
    options,
    id,
    onchange
  }: {
    value: T
    options: { value: T; label: string }[]
    id?: string
    // called after the user picks an option
    onchange?: (value: T) => void
  } = $props()

  let open = $state(false)
  let active = $state(0)
  let root: HTMLDivElement
  let button: HTMLButtonElement
  let list = $state<HTMLUListElement>()

  let selectedIndex = $derived(options.findIndex((o) => o.value === value))
  let label = $derived(options[selectedIndex]?.label ?? '')

  function show() {
    active = Math.max(0, selectedIndex)
    open = true
    requestAnimationFrame(() => list?.children[active]?.scrollIntoView({ block: 'nearest' }))
  }

  function close() {
    open = false
    button?.focus()
  }

  function pick(i: number) {
    const o = options[i]
    if (!o) return
    close()
    if (o.value !== value) {
      value = o.value
      onchange?.(o.value)
    }
  }

  function moveTo(i: number) {
    active = Math.min(Math.max(0, i), options.length - 1)
    list?.children[active]?.scrollIntoView({ block: 'nearest' })
  }

  function onkeydown(e: KeyboardEvent) {
    if (!open) {
      if (['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(e.key)) {
        e.preventDefault()
        show()
      }
      return
    }
    if (e.key === 'ArrowDown') moveTo(active + 1)
    else if (e.key === 'ArrowUp') moveTo(active - 1)
    else if (e.key === 'Home') moveTo(0)
    else if (e.key === 'End') moveTo(options.length - 1)
    else if (e.key === 'Enter' || e.key === ' ') pick(active)
    else if (e.key === 'Escape') {
      e.stopPropagation()
      close()
    } else if (e.key === 'Tab') open = false
    else if (e.key.length === 1) {
      // jump to the next option starting with the typed letter
      const k = e.key.toLowerCase()
      const n = options.length
      for (let step = 1; step <= n; step++) {
        const i = (active + step) % n
        if (options[i].label.toLowerCase().startsWith(k)) return moveTo(i)
      }
      return
    } else return
    e.preventDefault()
  }

  // composedPath: an option may be gone from the page by the time this runs.
  function onwindowclick(e: MouseEvent) {
    if (open && !e.composedPath().includes(root)) open = false
  }
</script>

<svelte:window onclick={onwindowclick} />

<div class="relative" bind:this={root}>
  <button
    bind:this={button}
    {id}
    type="button"
    class="input flex items-center justify-between gap-2 text-left"
    aria-haspopup="listbox"
    aria-expanded={open}
    onclick={() => (open ? close() : show())}
    {onkeydown}
  >
    <span class="truncate">{label}</span>
    <svg class="h-4 w-4 shrink-0 text-slate-400 transition {open ? 'rotate-180' : ''}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m6 9 6 6 6-6" /></svg>
  </button>

  {#if open}
    <ul
      bind:this={list}
      class="absolute top-full left-0 z-30 mt-1 max-h-64 min-w-full overflow-y-auto rounded-md border border-slate-200 bg-white py-1 text-sm text-slate-800 shadow-lg"
      role="listbox"
    >
      {#each options as o, i (o.value)}
        <!-- svelte-ignore a11y_click_events_have_key_events (the button handles the keyboard) -->
        <li
          role="option"
          aria-selected={i === selectedIndex}
          class="flex cursor-pointer items-center justify-between gap-3 px-3 py-1.5 whitespace-nowrap {i === active ? 'bg-brand-50' : ''} {i === selectedIndex ? 'font-semibold text-brand-700' : ''}"
          onmouseenter={() => (active = i)}
          onclick={() => pick(i)}
        >
          {o.label}
          {#if i === selectedIndex}<span aria-hidden="true">✓</span>{/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>
