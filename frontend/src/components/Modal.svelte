<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    title,
    onclose,
    width = 'max-w-lg',
    closable = true,
    children,
    footer
  }: {
    title: string
    onclose?: () => void
    width?: string
    closable?: boolean
    children: Snippet
    footer?: Snippet
  } = $props()

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && closable) onclose?.()
  }
</script>

<svelte:window {onkeydown} />

<div class="fixed inset-0 z-40 flex items-center justify-center bg-slate-900/50 p-4">
  <div class="card flex max-h-full w-full {width} flex-col">
    <div class="flex items-center justify-between border-b border-slate-200 px-5 py-3">
      <h2 class="text-lg font-semibold">{title}</h2>
      {#if closable}
        <button class="btn btn-ghost px-2 py-1 text-xl leading-none" onclick={() => onclose?.()} aria-label="Tutup">×</button>
      {/if}
    </div>
    <div class="overflow-y-auto px-5 py-4">
      {@render children()}
    </div>
    {#if footer}
      <div class="flex justify-end gap-2 border-t border-slate-200 px-5 py-3">
        {@render footer()}
      </div>
    {/if}
  </div>
</div>
