<script lang="ts">
  // Page navigation for a server-paged list. Hidden when everything fits on one page.
  let {
    page,
    pageSize,
    count,
    loading = false,
    ongoto
  }: {
    page: number
    pageSize: number
    count: number
    loading?: boolean
    ongoto: (page: number) => void
  } = $props()

  let pages = $derived(Math.max(1, Math.ceil(count / pageSize)))
  let firstRow = $derived(count > 0 ? (page - 1) * pageSize + 1 : 0)
  let lastRow = $derived(Math.min(page * pageSize, count))
</script>

{#if count > pageSize}
  <div class="mt-2 flex items-center justify-end gap-2 text-sm">
    <span class="mr-2 text-slate-500">{firstRow.toLocaleString('id-ID')}–{lastRow.toLocaleString('id-ID')} dari {count.toLocaleString('id-ID')}</span>
    <button class="btn btn-secondary px-2 py-1" onclick={() => ongoto(1)} disabled={loading || page <= 1} aria-label="Halaman pertama">«</button>
    <button class="btn btn-secondary px-3 py-1" onclick={() => ongoto(page - 1)} disabled={loading || page <= 1}>‹ Sebelumnya</button>
    <span class="px-2">Halaman {page.toLocaleString('id-ID')} / {pages.toLocaleString('id-ID')}</span>
    <button class="btn btn-secondary px-3 py-1" onclick={() => ongoto(page + 1)} disabled={loading || page >= pages}>Berikutnya ›</button>
    <button class="btn btn-secondary px-2 py-1" onclick={() => ongoto(pages)} disabled={loading || page >= pages} aria-label="Halaman terakhir">»</button>
  </div>
{/if}
