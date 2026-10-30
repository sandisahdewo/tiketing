<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import Field from '../components/Field.svelte'
  import FormAlert from '../components/FormAlert.svelte'
  import LoadingOverlay from '../components/LoadingOverlay.svelte'
  import { App, model, handleError, errorMessage } from '../lib/api'
  import { FormState, required, maxLength } from '../lib/form.svelte'
  import { shiftHours } from '../lib/shift.svelte'
  import { toast } from '../lib/toast.svelte'

  // Shift types (Siang, Sore, Malam…): a cashier picks one when opening a
  // shift. Types that were used are turned off, never deleted; a change only
  // applies to shifts opened after it.
  let types = $state<model.ShiftType[]>([])
  let loading = $state(true)
  let busy = $state(false)
  let flash = $state<number | null>(null)

  async function load() {
    loading = true
    try {
      types = await App.ListShiftTypes()
    } catch (e) {
      handleError(e)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    load()
  })

  function blink(id: number) {
    flash = id
    setTimeout(() => (flash = null), 2000)
  }

  // --- add / edit
  let editing = $state<{ id: number; name: string; startTime: string; endTime: string } | null>(null)
  let saving = $state(false)
  // Same rule as the server: H:MM or HH:MM, 00:00 to 23:59.
  const clockPattern = /^([01]?\d|2[0-3]):[0-5]\d$/
  const form = new FormState(() => ({
    name: required(editing?.name, 'Nama shift wajib diisi') ?? maxLength(editing?.name ?? '', 30, 'Nama shift maksimal 30 karakter'),
    startTime: clockPattern.test((editing?.startTime ?? '').trim()) ? undefined : 'Jam mulai tidak valid, contoh 06:00',
    endTime: !clockPattern.test((editing?.endTime ?? '').trim())
      ? 'Jam selesai tidak valid, contoh 14:00'
      : pad(editing?.endTime) === pad(editing?.startTime)
        ? 'Jam selesai tidak boleh sama dengan jam mulai'
        : undefined
  }))

  function pad(clock: string | undefined): string {
    return (clock ?? '').trim().padStart(5, '0')
  }

  let overnight = $derived(
    !!editing && clockPattern.test(editing.startTime.trim()) && clockPattern.test(editing.endTime.trim()) && pad(editing.endTime) < pad(editing.startTime)
  )

  function openForm(t?: model.ShiftType) {
    form.reset()
    editing = t ? { id: t.id, name: t.name, startTime: t.startTime, endTime: t.endTime } : { id: 0, name: '', startTime: '', endTime: '' }
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    if (!editing || saving || !form.check()) return
    saving = true
    try {
      const saved = await App.SaveShiftType(model.ShiftTypeInput.createFrom(editing))
      toast(editing.id ? 'Shift diperbarui' : 'Shift ditambahkan')
      editing = null
      await load()
      blink(saved.id)
    } catch (err) {
      form.fail(errorMessage(err), { name: /nama/i, startTime: /jam mulai/i, endTime: /jam selesai/i })
    } finally {
      saving = false
    }
  }

  // --- turning on and off
  let confirming = $state<model.ShiftType | null>(null)

  async function toggle() {
    if (!confirming || busy) return
    const t = confirming
    busy = true
    try {
      await App.SetShiftTypeActive(t.id, !t.active)
      toast(`Shift ${t.name} ${t.active ? 'dinon-aktifkan' : 'diaktifkan'}`)
      confirming = null
      await load()
      blink(t.id)
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
      <h1 class="text-2xl font-bold">Shift</h1>
      <p class="text-sm text-slate-500">Kasir memilih salah satu shift aktif saat buka shift. Setelah jam selesai lewat, aplikasi menyarankan tutup shift.</p>
    </div>
    <button class="btn btn-primary" onclick={() => openForm()}>+ Tambah Shift</button>
  </div>

  <div class="relative mt-4 flex min-h-0 flex-1 flex-col">
    <div class="card min-h-40 flex-1 overflow-y-auto">
      <table class="table">
        <thead class="sticky top-0">
          <tr>
            <th>Nama</th>
            <th>Jam</th>
            <th>Status</th>
            <th class="w-64"></th>
          </tr>
        </thead>
        <tbody>
          {#if !loading && types.length === 0}
            <tr><td colspan="4" class="py-10 text-center text-slate-500">Belum ada shift.</td></tr>
          {/if}
          {#each types as t (t.id)}
            <tr class="hover:bg-slate-50 {t.active ? '' : 'text-slate-400'} {t.id === flash ? 'row-flash' : ''}">
              <td class="font-medium">{t.name}</td>
              <td class="tabular-nums">{shiftHours(t)}</td>
              <td>
                {#if t.active}
                  <span class="rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700">Aktif</span>
                {:else}
                  <span class="rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-500">Non-aktif</span>
                {/if}
              </td>
              <td class="text-right whitespace-nowrap">
                <button class="btn btn-ghost px-3 py-1" onclick={() => (confirming = t)} disabled={busy}>{t.active ? 'Non-aktifkan' : 'Aktifkan'}</button>
                <button class="btn btn-secondary px-3 py-1" onclick={() => openForm(t)}>Ubah</button>
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
  <Modal title={editing.id ? 'Ubah Shift' : 'Tambah Shift'} onclose={() => (editing = null)}>
    <form id="shift-type-form" class="space-y-4" onsubmit={save} novalidate>
      <FormAlert message={form.general} />
      <Field label="Nama shift" id="st-name" {form} name="name" hint="Misalnya Siang, Sore, atau Malam.">
        <input id="st-name" class="input" bind:value={editing.name} maxlength="30" />
      </Field>
      <div class="grid grid-cols-2 gap-4">
        <Field label="Jam mulai" id="st-start" {form} name="startTime">
          <input id="st-start" class="input tabular-nums" bind:value={editing.startTime} maxlength="5" placeholder="06:00" />
        </Field>
        <Field label="Jam selesai" id="st-end" {form} name="endTime">
          <input id="st-end" class="input tabular-nums" bind:value={editing.endTime} maxlength="5" placeholder="14:00" />
        </Field>
      </div>
      <p class="text-sm text-slate-500">
        {#if overnight}
          Shift ini melewati tengah malam dan selesai keesokan harinya.
        {:else}
          Kalau jam selesai lebih kecil dari jam mulai (misalnya 22:00–06:00), shift selesai keesokan harinya.
        {/if}
        Perubahan hanya berlaku untuk shift yang dibuka setelah ini.
      </p>
    </form>
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (editing = null)}>Batal</button>
      <button class="btn btn-primary" form="shift-type-form" disabled={saving}>{saving ? 'Menyimpan…' : 'Simpan'}</button>
    {/snippet}
  </Modal>
{/if}

{#if confirming}
  {@const t = confirming}
  <Modal title={t.active ? 'Non-aktifkan Shift' : 'Aktifkan Shift'} onclose={() => (confirming = null)} width="max-w-md">
    {#if t.active}
      <p>Non-aktifkan shift <b>{t.name}</b>? Shift ini tidak bisa dipilih saat buka shift. Shift {t.name} yang sedang berjalan tetap bisa dipakai sampai ditutup, dan laporannya tetap ada.</p>
    {:else}
      <p>Aktifkan shift <b>{t.name}</b>? Kasir bisa memilihnya lagi saat buka shift.</p>
    {/if}
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (confirming = null)} disabled={busy}>Batal</button>
      <button class="btn {t.active ? 'btn-danger' : 'btn-primary'}" onclick={toggle} disabled={busy}>{busy ? 'Menyimpan…' : 'Lanjutkan'}</button>
    {/snippet}
  </Modal>
{/if}
