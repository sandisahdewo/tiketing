<script lang="ts">
  // Covers its nearest `relative` parent while data loads, keeping the old
  // content dimmed underneath.
  let { show, text = 'Memuat data…' }: { show: boolean; text?: string } = $props()
</script>

{#if show}
  <div class="absolute inset-0 z-10 flex items-center justify-center rounded-lg bg-white/70" role="status" aria-live="polite">
    <div class="flex items-center gap-3 rounded-md bg-white px-4 py-2 text-sm text-slate-600 shadow">
      <span class="h-5 w-5 animate-spin rounded-full border-2 border-slate-300 border-t-brand-600"></span>
      {text}
    </div>
  </div>
{/if}
