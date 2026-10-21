<script lang="ts">
  import Modal from './Modal.svelte'
  import ReceiptPaper from './ReceiptPaper.svelte'
  import { App, handleError } from '../lib/api'
  import { toast } from '../lib/toast.svelte'
  import type { main } from '../../wailsjs/go/models'

  let {
    saleId,
    title = 'Nota',
    newTransaction = false,
    onclose,
    onview
  }: {
    saleId: number
    title?: string
    newTransaction?: boolean
    onclose: () => void
    // shows the sale in the transaction list
    onview?: () => void
  } = $props()

  let view = $state<main.ReceiptView | null>(null)
  let loading = $state(true)
  let printing = $state(false)

  async function load() {
    try {
      view = await App.ReceiptPreview(saleId)
    } catch (e) {
      handleError(e)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    loading = true
    load()
  })

  async function print() {
    if (printing) return
    printing = true
    try {
      toast(await App.PrintReceipt(saleId))
    } catch (e) {
      handleError(e)
    } finally {
      printing = false
      load()
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'F9' || (e.ctrlKey && e.key.toLowerCase() === 'p')) {
      e.preventDefault()
      if (!printing && !view?.deleted) print()
    }
  }
</script>

<svelte:window {onkeydown} />

<Modal {title} {onclose} width="max-w-2xl">
  {#if view?.deleted}
    <p class="mb-3 rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">
      Nota ini sudah <b>dihapus</b>{view.deleteReason ? `: ${view.deleteReason}` : ''}. Tidak bisa dicetak.
    </p>
  {:else if view && view.printCount > 0}
    <p class="mb-3 rounded-md bg-amber-50 px-3 py-2 text-sm text-amber-800">
      Nota ini sudah dicetak {view.printCount}× — cetakan berikutnya berlabel <b>Reprint {view.printCount}</b>.
    </p>
  {/if}
  <div class="flex justify-center bg-slate-100 p-4">
    {#if loading && !view}
      <p class="py-10 text-slate-500">Memuat nota…</p>
    {:else if view}
      <ReceiptPaper lines={view.lines} logo={view.logo} />
    {/if}
  </div>
  {#snippet footer()}
    {#if onview}
      <button class="btn btn-secondary mr-auto" onclick={onview} disabled={printing}>Lihat Transaksi</button>
    {/if}
    <button class="btn btn-secondary" onclick={print} disabled={printing || !view || view.deleted}>
      {printing ? 'Mencetak…' : view && view.printCount > 0 ? `Cetak Ulang (F9)` : 'Cetak Nota (F9)'}
    </button>
    <button class="btn btn-primary" onclick={onclose}>{newTransaction ? 'Transaksi Baru' : 'Tutup'}</button>
  {/snippet}
</Modal>
