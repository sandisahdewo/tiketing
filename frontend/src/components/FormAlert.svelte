<script lang="ts">
  // An error that is not about one field (wrong login, server unreachable,
  // printer failed, ...), shown above the fields of a form.
  let { message }: { message: string } = $props()
</script>

{#if message}
  <div class="flex items-start gap-2 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700" role="alert">
    <svg class="mt-0.5 h-4 w-4 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="9" /><path d="M12 8v5M12 16h.01" /></svg>
    <span>{message}</span>
  </div>
{/if}
