<script lang="ts">
  import { toasts } from '../lib/toast.svelte'

  const styles = {
    success: 'bg-emerald-600',
    error: 'bg-red-600',
    info: 'bg-slate-800'
  }
</script>

<div class="pointer-events-none fixed top-16 right-4 z-50 flex w-96 max-w-[90vw] flex-col gap-2">
  {#each toasts as t (t.id)}
    <div class="rounded-md px-4 py-3 text-sm break-words text-white shadow-lg {styles[t.kind]}">{t.text}</div>
  {/each}
</div>
