<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import ReceiptModal from '../components/ReceiptModal.svelte'
  import { App, model, handleError } from '../lib/api'
  import { today, formatTime } from '../lib/date'
  import { formatRp } from '../lib/money'

  let { onclose }: { onclose: () => void } = $props()

  let sales = $state<model.Sale[]>([])
  let loading = $state(true)
  let selected = $state<number | null>(null)

  $effect(() => {
    const d = today()
    App.ListSales(model.SaleQuery.createFrom({ from: d, to: d }))
      .then((s) => (sales = s))
      .catch(handleError)
      .finally(() => (loading = false))
  })

  let total = $derived(sales.reduce((sum, s) => sum + s.total, 0))
</script>

{#if selected !== null}
  <ReceiptModal saleId={selected} title="Cetak Ulang Nota" onclose={() => (selected = null)} />
{:else}
  <Modal title="Transaksi Hari Ini" {onclose} width="max-w-2xl">
    {#if loading}
      <p class="py-8 text-center text-slate-500">Memuat…</p>
    {:else if sales.length === 0}
      <p class="py-8 text-center text-slate-500">Belum ada transaksi hari ini.</p>
    {:else}
      <table class="table">
        <thead>
          <tr><th>Jam</th><th>No. Nota</th><th>Kasir</th><th class="text-right">Total</th><th class="text-center">Cetak</th><th></th></tr>
        </thead>
        <tbody>
          {#each sales as s (s.id)}
            <tr class="hover:bg-slate-50">
              <td>{formatTime(s.createdAt)}</td>
              <td class="font-mono">{s.invoiceNo}</td>
              <td>{s.cashierName}</td>
              <td class="text-right font-medium">{formatRp(s.total)}</td>
              <td class="text-center text-slate-500">{s.printCount}×</td>
              <td class="text-right"><button class="btn btn-secondary px-3 py-1" onclick={() => (selected = s.id)}>Lihat / Cetak</button></td>
            </tr>
          {/each}
        </tbody>
        <tfoot>
          <tr><td colspan="3" class="font-semibold">{sales.length} transaksi</td><td class="text-right font-bold">{formatRp(total)}</td><td colspan="2"></td></tr>
        </tfoot>
      </table>
    {/if}
  </Modal>
{/if}
