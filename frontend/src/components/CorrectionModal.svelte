<script lang="ts">
  import Modal from './Modal.svelte'
  import Field from './Field.svelte'
  import { App, handleError, type model } from '../lib/api'
  import { FormState, required, maxLength } from '../lib/form.svelte'
  import { formatRp } from '../lib/money'
  import { startCorrection } from '../lib/correction.svelte'

  // Asks why a sale is corrected; the cashier screen then takes its items
  // into the cart to fix them. Only an admin corrects.
  let { sale, onclose }: { sale: model.Sale; onclose: () => void } = $props()

  // The new sale goes into the sale's shift while it is open (the money
  // changes hands at that drawer), else into the admin's own shift here.
  let target = $state<model.Shift | null>(null)
  let loading = $state(true)
  $effect(() => {
    const id = sale.shiftId
    if (!id) {
      loading = false
      return
    }
    App.GetShift(id)
      .then((r) => (target = r.shift.closedAt ? null : r.shift))
      .catch(handleError)
      .finally(() => (loading = false))
  })

  let reason = $state('')
  const form = new FormState(() => ({
    reason: required(reason, 'Alasan koreksi wajib diisi') ?? maxLength(reason, 200, 'Alasan maksimal 200 karakter')
  }))

  function submit(e: SubmitEvent) {
    e.preventDefault()
    if (loading || !form.check()) return
    startCorrection(sale, reason.trim(), target)
    onclose()
  }
</script>

<Modal title="Koreksi Nota" {onclose} width="max-w-md">
  <form id="correct-sale-form" class="space-y-3" onsubmit={submit} novalidate>
    <p>
      Koreksi nota <b class="font-mono">{sale.invoiceNo}</b> ({formatRp(sale.total)}, kasir {sale.cashierName})? Itemnya masuk ke keranjang untuk
      diperbaiki. Nota ini baru dihapus saat nota penggantinya dibayar.
    </p>
    {#if !loading}
      <p class="rounded-md bg-slate-50 px-3 py-2 text-sm text-slate-600">
        {#if target}
          Nota pengganti dicatat di shift <b>{target.name}</b> milik <b>{target.cashierName}</b> ({target.counterName}), karena uangnya dikembalikan
          atau diterima di laci itu.
        {:else}
          Shift nota ini sudah ditutup, jadi nota pengganti dicatat di shift Anda di komputer ini. Buka shift dulu kalau belum.
        {/if}
      </p>
    {/if}
    <Field label="Alasan" id="correct-reason" {form} name="reason">
      <!-- svelte-ignore a11y_autofocus -->
      <input id="correct-reason" class="input w-full" maxlength="200" bind:value={reason} placeholder="contoh: salah jumlah tiket" autofocus />
    </Field>
  </form>
  {#snippet footer()}
    <button class="btn btn-secondary" onclick={onclose}>Batal</button>
    <button class="btn btn-primary" type="submit" form="correct-sale-form" disabled={loading}>Koreksi di Kasir</button>
  {/snippet}
</Modal>
