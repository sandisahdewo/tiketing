<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import MoneyInput from '../components/MoneyInput.svelte'
  import Field from '../components/Field.svelte'
  import { formatMoney, formatRp } from '../lib/money'

  let {
    total,
    busy,
    onpay,
    onclose
  }: {
    total: number
    busy: boolean
    onpay: (paid: number) => void
    onclose: () => void
  } = $props()

  let paid = $state<number | null>(null)
  let paidInvalid = $state(false)
  let input: MoneyInput

  let change = $derived(paid == null ? null : paid - total)
  let enough = $derived(paid != null && paid >= total)

  let suggestions = $derived.by(() => {
    const steps = [500000, 1000000, 2000000, 5000000, 10000000]
    const out = new Set<number>([total])
    for (const step of steps) {
      const v = Math.ceil(total / step) * step
      if (v > total) out.add(v)
    }
    return [...out].sort((a, b) => a - b).slice(0, 6)
  })

  function pick(v: number) {
    input.set(v)
  }

  function submit() {
    if (enough && !busy && paid != null) onpay(paid)
  }
</script>

<Modal title="Pembayaran Tunai" {onclose} width="max-w-md" closable={!busy}>
  <div class="rounded-lg bg-brand-50 p-4 text-center">
    <div class="text-sm text-slate-500">Total Belanja</div>
    <div class="text-4xl font-bold text-brand-700">{formatRp(total)}</div>
  </div>

  <Field class="mt-5" label="Uang diterima" id="paid" error={paidInvalid ? 'Angka terlalu besar' : ''}>
    <MoneyInput id="paid" bind:this={input} bind:value={paid} bind:invalid={paidInvalid} autofocus large onenter={submit} />
  </Field>

  <div class="mt-3 grid grid-cols-3 gap-2">
    {#each suggestions as s, i}
      <button class="btn btn-secondary px-2" onclick={() => pick(s)}>{i === 0 ? 'Uang pas' : formatMoney(s)}</button>
    {/each}
  </div>

  <div class="mt-5 flex items-center justify-between rounded-lg border p-4 {change != null && change < 0 ? 'border-red-200 bg-red-50' : 'border-emerald-200 bg-emerald-50'}">
    <span class="font-medium">{change != null && change < 0 ? 'Kurang' : 'Kembalian'}</span>
    <span class="text-2xl font-bold {change != null && change < 0 ? 'text-red-600' : 'text-emerald-700'}">
      {change == null ? '-' : formatRp(Math.abs(change))}
    </span>
  </div>

  {#snippet footer()}
    <button class="btn btn-secondary" onclick={onclose} disabled={busy}>Batal</button>
    <button class="btn btn-primary px-6" onclick={submit} disabled={!enough || busy}>
      {busy ? 'Menyimpan…' : 'Bayar (Enter)'}
    </button>
  {/snippet}
</Modal>
