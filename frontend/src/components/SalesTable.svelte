<script lang="ts">
  import Modal from './Modal.svelte'
  import ReceiptModal from './ReceiptModal.svelte'
  import { App, model, handleError } from '../lib/api'
  import { formatRp } from '../lib/money'
  import { formatDateTime } from '../lib/date'
  import { categoryLabel } from '../lib/colors'
  import { toast } from '../lib/toast.svelte'
  import { isAdmin } from '../lib/session.svelte'

  // Transaction list with expandable item detail, receipt preview and (admin) delete.
  let {
    sales,
    expanded = $bindable({}),
    empty = 'Tidak ada transaksi.',
    ondeleted
  }: {
    sales: model.Sale[]
    expanded?: Record<number, boolean>
    empty?: string
    ondeleted?: () => void
  } = $props()

  const admin = isAdmin()

  let selectedSale = $state<number | null>(null)

  let deleting = $state<model.Sale | null>(null)
  let deleteReason = $state('')
  let deleteBusy = $state(false)

  function askDelete(s: model.Sale) {
    deleting = s
    deleteReason = ''
  }

  async function removeSale(e: SubmitEvent) {
    e.preventDefault()
    if (!deleting || deleteBusy) return
    deleteBusy = true
    try {
      await App.DeleteSale(deleting.id, deleteReason)
      toast(`Nota ${deleting.invoiceNo} dihapus`)
      deleting = null
      ondeleted?.()
    } catch (err) {
      handleError(err)
    } finally {
      deleteBusy = false
    }
  }
</script>

<table class="table">
  <thead class="sticky top-0"><tr><th class="w-8"></th><th>Waktu</th><th>No. Nota</th><th>Kasir</th><th>Loket</th><th class="text-right">Total</th><th class="text-center">Cetak</th><th></th></tr></thead>
  <tbody>
    {#each sales as s (s.id)}
      <tr class="cursor-pointer hover:bg-slate-50 {s.deletedAt ? 'text-slate-400' : ''}" onclick={() => (expanded[s.id] = !expanded[s.id])}>
        <td class="text-slate-400">{expanded[s.id] ? '▾' : '▸'}</td>
        <td>{formatDateTime(s.createdAt)}</td>
        <td class="font-mono {s.deletedAt ? 'line-through' : ''}">{s.invoiceNo}</td>
        <td>{s.cashierName}</td>
        <td>{s.terminal}</td>
        <td class="text-right font-medium {s.deletedAt ? 'line-through' : ''}">{formatRp(s.total)}</td>
        <td class="text-center text-slate-500">{s.printCount}×</td>
        <td class="whitespace-nowrap text-right">
          <button class="btn btn-secondary px-3 py-1" onclick={(e) => { e.stopPropagation(); selectedSale = s.id }}>Nota</button>
          {#if admin && !s.deletedAt}
            <button class="btn px-3 py-1 text-red-600 hover:bg-red-50" onclick={(e) => { e.stopPropagation(); askDelete(s) }}>Hapus</button>
          {/if}
        </td>
      </tr>
      {#if s.deletedAt}
        <tr class="bg-red-50/60">
          <td></td>
          <td colspan="7" class="text-sm text-red-700">Dihapus {formatDateTime(s.deletedAt)} oleh {s.deletedByName}: {s.deleteReason}</td>
        </tr>
      {/if}
      {#if expanded[s.id]}
        <tr class="bg-slate-50">
          <td></td>
          <td colspan="7" class="py-2">
            <table class="w-full text-sm [&_td]:border-0 [&_td]:py-0.5">
              <tbody>
                {#each s.items as it (it.id)}
                  <tr>
                    <td>{it.ticketName} <span class="text-xs text-slate-400">{categoryLabel(it.category)}</span></td>
                    <td class="w-40 text-right text-slate-500">{it.qty} × {formatRp(it.price)}</td>
                    <td class="w-36 text-right font-medium">{formatRp(it.subtotal)}</td>
                  </tr>
                {/each}
                <tr class="border-t border-slate-200">
                  <td></td>
                  <td class="pt-1! text-right text-slate-500">Total</td>
                  <td class="pt-1! text-right font-semibold">{formatRp(s.total)}</td>
                </tr>
                <tr class="text-slate-500">
                  <td></td>
                  <td class="text-right">Bayar</td>
                  <td class="text-right">{formatRp(s.paid)}</td>
                </tr>
                <tr class="text-slate-500">
                  <td></td>
                  <td class="text-right">Kembali</td>
                  <td class="text-right">{formatRp(s.changeAmount)}</td>
                </tr>
              </tbody>
            </table>
          </td>
        </tr>
      {/if}
    {:else}
      <tr><td colspan="8" class="py-10 text-center text-slate-500">{empty}</td></tr>
    {/each}
  </tbody>
</table>

{#if selectedSale !== null}
  <ReceiptModal saleId={selectedSale} onclose={() => (selectedSale = null)} />
{/if}

{#if deleting}
  <Modal title="Hapus Nota" onclose={() => (deleting = null)} width="max-w-md">
    <form id="delete-sale-form" class="space-y-3" onsubmit={removeSale}>
      <p>
        Hapus nota <b class="font-mono">{deleting.invoiceNo}</b> ({formatRp(deleting.total)}, kasir {deleting.cashierName})?
        Nota tidak dihitung lagi di laporan dan tidak bisa dicetak ulang. Nomor nota tidak dipakai ulang.
      </p>
      <div>
        <label class="label" for="delete-reason">Alasan</label>
        <!-- svelte-ignore a11y_autofocus -->
        <input id="delete-reason" class="input w-full" maxlength="200" bind:value={deleteReason} placeholder="contoh: salah input, transaksi batal" autofocus />
      </div>
    </form>
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (deleting = null)}>Batal</button>
      <button class="btn btn-danger" type="submit" form="delete-sale-form" disabled={deleteBusy || !deleteReason.trim()}>{deleteBusy ? 'Menghapus…' : 'Hapus Nota'}</button>
    {/snippet}
  </Modal>
{/if}
