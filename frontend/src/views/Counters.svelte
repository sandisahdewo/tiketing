<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import Field from '../components/Field.svelte'
  import FormAlert from '../components/FormAlert.svelte'
  import SortHeader from '../components/SortHeader.svelte'
  import LoadingOverlay from '../components/LoadingOverlay.svelte'
  import { App, model, handleError, errorMessage } from '../lib/api'
  import { FormState, required, maxLength } from '../lib/form.svelte'
  import { sortRows, type Sort } from '../lib/sort'
  import { formatDateTime, today } from '../lib/date'
  import { session } from '../lib/session.svelte'
  import { toast } from '../lib/toast.svelte'

  // Counters (loket): each PC sells at the counter an admin puts it on, and a
  // counter is used by one PC at most. Counters with sales are turned off,
  // never deleted.
  let counters = $state<model.Counter[]>([])
  let invoicePrefix = $state('')
  let loading = $state(true)
  let busy = $state(false)
  let flash = $state<number | null>(null)
  let thisDevice = $derived(session.info?.deviceId ?? '')

  let sort = $state<Sort>({ key: 'name', dir: 'asc' })
  const sortValue: Record<string, (c: model.Counter) => string | number | null> = {
    name: (c) => c.name,
    invoiceCode: (c) => c.invoiceCode,
    device: (c) => c.deviceName || null,
    lastSeen: (c) => c.lastSeenAt ?? null,
    active: (c) => (c.active ? 1 : 0)
  }
  let sorted = $derived(sortRows(counters, sortValue[sort.key], sort.dir))

  async function load() {
    loading = true
    try {
      counters = await App.ListCounters()
    } catch (e) {
      handleError(e)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    load()
    App.GetSettings()
      .then((s) => (invoicePrefix = s.invoicePrefix))
      .catch(handleError)
  })

  function blink(id: number) {
    flash = id
    setTimeout(() => (flash = null), 2000)
  }

  // Tell the header and Pengaturan when this PC's counter changed.
  async function refreshThisPC() {
    await App.MyCounter()
    session.info = await App.GetAppInfo()
  }

  // --- add / edit
  let editing = $state<{ id: number; name: string; invoiceCode: string } | null>(null)
  let saving = $state(false)
  const form = new FormState(() => ({
    name: required(editing?.name, 'Nama loket wajib diisi') ?? maxLength(editing?.name ?? '', 30, 'Nama loket maksimal 30 karakter'),
    invoiceCode: /^[A-Za-z0-9]{1,4}$/.test((editing?.invoiceCode ?? '').trim()) ? undefined : 'Kode nota 1-4 karakter huruf/angka'
  }))
  let invoiceExample = $derived(`${invoicePrefix}${(editing?.invoiceCode ?? '').trim().toUpperCase()}${today().replaceAll('-', '')}0001`)

  function openForm(c?: model.Counter) {
    form.reset()
    editing = c ? { id: c.id, name: c.name, invoiceCode: c.invoiceCode } : { id: 0, name: '', invoiceCode: '' }
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    if (!editing || saving || !form.check()) return
    saving = true
    try {
      const saved = await App.SaveCounter(model.CounterInput.createFrom(editing))
      toast(editing.id ? 'Loket diperbarui' : 'Loket ditambahkan')
      editing = null
      await load()
      blink(saved.id)
      if (saved.deviceId === thisDevice) await refreshThisPC()
    } catch (err) {
      form.fail(errorMessage(err), { name: /nama/i, invoiceCode: /kode/i })
    } finally {
      saving = false
    }
  }

  // --- confirmed actions
  type Action = { kind: 'use' | 'release' | 'deactivate' | 'activate'; counter: model.Counter }
  let confirming = $state<Action | null>(null)

  async function run() {
    if (!confirming || busy) return
    const { kind, counter } = confirming
    busy = true
    try {
      if (kind === 'use') await App.UseCounter(counter.id, !!counter.deviceId)
      else if (kind === 'release') await App.ReleaseCounter(counter.deviceId)
      else await App.SetCounterActive(counter.id, kind === 'activate')
      toast(
        kind === 'use'
          ? `${counter.name} dipakai di komputer ini`
          : kind === 'release'
            ? `${counter.name} dilepas dari ${counter.deviceName || 'komputer'}`
            : `${counter.name} ${kind === 'activate' ? 'diaktifkan' : 'dinon-aktifkan'}`
      )
      confirming = null
      await load()
      blink(counter.id)
      await refreshThisPC()
    } catch (e) {
      handleError(e)
    } finally {
      busy = false
    }
  }
</script>

<div class="flex h-full flex-col p-6">
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-2xl font-bold">Loket</h1>
      <p class="text-sm text-slate-500">Setiap komputer bertransaksi di loket yang dipilih admin. Satu loket hanya untuk satu komputer.</p>
    </div>
    <button class="btn btn-primary" onclick={() => openForm()}>+ Tambah Loket</button>
  </div>

  <div class="relative mt-4 flex min-h-0 flex-1 flex-col">
    <div class="card min-h-40 flex-1 overflow-y-auto">
      <table class="table">
        <thead class="sticky top-0">
          <tr>
            <SortHeader label="Nama" key="name" {sort} onsort={(s) => (sort = s)} />
            <SortHeader label="Kode nota" key="invoiceCode" {sort} onsort={(s) => (sort = s)} />
            <SortHeader label="Komputer" key="device" {sort} onsort={(s) => (sort = s)} />
            <SortHeader label="Terakhir aktif" key="lastSeen" {sort} onsort={(s) => (sort = s)} firstDir="desc" />
            <SortHeader label="Status" key="active" {sort} onsort={(s) => (sort = s)} firstDir="desc" />
            <th class="w-96"></th>
          </tr>
        </thead>
        <tbody>
          {#if !loading && counters.length === 0}
            <tr><td colspan="6" class="py-10 text-center text-slate-500">Belum ada loket.</td></tr>
          {/if}
          {#each sorted as c (c.id)}
            <tr class="hover:bg-slate-50 {c.active ? '' : 'text-slate-400'} {c.id === flash ? 'row-flash' : ''}">
              <td class="font-medium">{c.name}</td>
              <td class="font-mono">{c.invoiceCode}</td>
              <td>
                {#if c.deviceId}
                  {c.deviceName || 'Komputer'}
                  {#if c.deviceId === thisDevice}<span class="ml-1 rounded-full bg-brand-50 px-2 py-0.5 text-xs font-medium text-brand-700">komputer ini</span>{/if}
                {:else}
                  <span class="text-slate-400">–</span>
                {/if}
              </td>
              <td class="text-slate-500">{formatDateTime(c.lastSeenAt)}</td>
              <td>
                {#if c.active}
                  <span class="rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700">Aktif</span>
                {:else}
                  <span class="rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-500">Non-aktif</span>
                {/if}
              </td>
              <td class="text-right whitespace-nowrap">
                {#if c.active && c.deviceId !== thisDevice}
                  <button class="btn btn-ghost px-3 py-1" onclick={() => (confirming = { kind: 'use', counter: c })} disabled={busy}>Pakai di komputer ini</button>
                {/if}
                {#if c.deviceId}
                  <button class="btn btn-ghost px-3 py-1" onclick={() => (confirming = { kind: 'release', counter: c })} disabled={busy}>Lepas</button>
                {/if}
                <button class="btn btn-ghost px-3 py-1" onclick={() => (confirming = { kind: c.active ? 'deactivate' : 'activate', counter: c })} disabled={busy}>
                  {c.active ? 'Non-aktifkan' : 'Aktifkan'}
                </button>
                <button class="btn btn-secondary px-3 py-1" onclick={() => openForm(c)}>Ubah</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <LoadingOverlay show={loading} />
  </div>
</div>

{#if editing}
  <Modal title={editing.id ? 'Ubah Loket' : 'Tambah Loket'} onclose={() => (editing = null)}>
    <form id="counter-form" class="space-y-4" onsubmit={save} novalidate>
      <FormAlert message={form.general} />
      <Field label="Nama loket" id="c-name" {form} name="name" hint="Tampil di nota, laporan, dan pilihan loket. Mengganti nama berlaku di semua laporan.">
        <input id="c-name" class="input" bind:value={editing.name} maxlength="30" />
      </Field>
      {#snippet codeHint()}
        Unik per loket, disisipkan setelah prefix. Contoh no. nota: <span class="font-mono font-semibold">{invoiceExample}</span>
      {/snippet}
      <Field label="Kode nota" id="c-code" {form} name="invoiceCode" hint={codeHint}>
        <input id="c-code" class="input w-32 uppercase" bind:value={editing.invoiceCode} maxlength="4" placeholder="1" />
      </Field>
    </form>
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (editing = null)}>Batal</button>
      <button class="btn btn-primary" form="counter-form" disabled={saving}>{saving ? 'Menyimpan…' : 'Simpan'}</button>
    {/snippet}
  </Modal>
{/if}

{#if confirming}
  {@const c = confirming.counter}
  <Modal
    title={confirming.kind === 'use' ? 'Pakai Loket' : confirming.kind === 'release' ? 'Lepas Loket' : confirming.kind === 'activate' ? 'Aktifkan Loket' : 'Non-aktifkan Loket'}
    onclose={() => (confirming = null)}
    width="max-w-md"
  >
    {#if confirming.kind === 'use'}
      <p>
        Pakai <b>{c.name}</b> di komputer ini? Loket lama komputer ini (kalau ada) dilepas.
        {#if c.deviceId}<br /><br /><b class="text-red-700">Loket ini sedang dipakai komputer {c.deviceName || 'lain'}</b>, yang tidak bisa bertransaksi lagi sampai dipilihkan loket lain.{/if}
      </p>
    {:else if confirming.kind === 'release'}
      <p>Lepas <b>{c.name}</b> dari komputer <b>{c.deviceName || 'lain'}</b>? Komputer itu tidak bisa bertransaksi sampai dipilihkan loket lain.</p>
    {:else if confirming.kind === 'deactivate'}
      <p>
        Non-aktifkan <b>{c.name}</b>? Loket tidak bisa dipakai bertransaksi{c.deviceId ? ` dan dilepas dari komputer ${c.deviceName}` : ''}. Riwayat transaksinya tetap ada di laporan.
      </p>
    {:else}
      <p>Aktifkan <b>{c.name}</b>? Loket bisa dipilih lagi untuk sebuah komputer.</p>
    {/if}
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (confirming = null)} disabled={busy}>Batal</button>
      <button class="btn {confirming?.kind === 'activate' || (confirming?.kind === 'use' && !c.deviceId) ? 'btn-primary' : 'btn-danger'}" onclick={run} disabled={busy}>
        {busy ? 'Menyimpan…' : 'Lanjutkan'}
      </button>
    {/snippet}
  </Modal>
{/if}
