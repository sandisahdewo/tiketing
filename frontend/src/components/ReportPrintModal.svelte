<script lang="ts">
  import Modal from './Modal.svelte'
  import ReceiptPaper from './ReceiptPaper.svelte'
  import { App, errorMessage, handleError } from '../lib/api'
  import { toast } from '../lib/toast.svelte'
  import type { main } from '../../wailsjs/go/models'

  // Shows how a tab of the report page prints, and prints it.
  let { title, request, onclose }: { title: string; request: main.ReportTab; onclose: () => void } = $props()

  let view = $state<main.ReceiptView | null>(null)
  let loading = $state(true)
  let printing = $state(false)
  // Why there is nothing to print, e.g. a list too long for the paper.
  let problem = $state('')

  $effect(() => {
    loading = true
    App.ReportPrintPreview(request)
      .then((v) => (view = v))
      .catch((e) => (problem = errorMessage(e)))
      .finally(() => (loading = false))
  })

  async function print() {
    if (printing || !view) return
    printing = true
    try {
      toast(await App.PrintReport(request))
    } catch (e) {
      handleError(e)
    } finally {
      printing = false
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'F9' || (e.ctrlKey && e.key.toLowerCase() === 'p')) {
      e.preventDefault()
      print()
    }
  }
</script>

<svelte:window {onkeydown} />

<Modal {title} {onclose} width="max-w-2xl">
  {#if problem}
    <p class="rounded-md bg-amber-50 px-3 py-2 text-sm text-amber-800">{problem}</p>
  {:else}
    <div class="flex justify-center bg-slate-100 p-4">
      {#if loading && !view}
        <p class="py-10 text-slate-500">Memuat laporan…</p>
      {:else if view}
        <ReceiptPaper lines={view.lines} logo={view.logo} />
      {/if}
    </div>
  {/if}
  {#snippet footer()}
    <button class="btn btn-secondary" onclick={print} disabled={printing || !view}>{printing ? 'Mencetak…' : 'Cetak (F9)'}</button>
    <button class="btn btn-primary" onclick={onclose}>Tutup</button>
  {/snippet}
</Modal>
