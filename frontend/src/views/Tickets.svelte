<script lang="ts">
  import { untrack } from 'svelte'
  import Modal from '../components/Modal.svelte'
  import Pager from '../components/Pager.svelte'
  import LoadingOverlay from '../components/LoadingOverlay.svelte'
  import SortHeader from '../components/SortHeader.svelte'
  import type { Sort } from '../lib/sort'
  import MoneyInput from '../components/MoneyInput.svelte'
  import Field from '../components/Field.svelte'
  import FormAlert from '../components/FormAlert.svelte'
  import { FormState, required, maxLength } from '../lib/form.svelte'
  import { App, model, handleError, errorMessage } from '../lib/api'
  import { formatMoney, formatRp } from '../lib/money'
  import { categories, categoryLabel, colors, colorClass } from '../lib/colors'
  import { formatDateTime } from '../lib/date'
  import { toast } from '../lib/toast.svelte'

  const pageSize = 50

  let result = $state<model.TicketPage | null>(null)
  let tickets = $derived(result?.tickets ?? [])
  let page = $state(1)
  let search = $state('')
  let appliedSearch = $state('')
  let requestSeq = 0
  type Status = 'all' | 'active' | 'inactive' | 'deleted'
  const statuses: { value: Status; label: string }[] = [
    { value: 'all', label: 'Semua' },
    { value: 'active', label: 'Aktif' },
    { value: 'inactive', label: 'Non-aktif' },
    { value: 'deleted', label: 'Terhapus' }
  ]

  let status = $state<Status>('all')
  let showDeleted = $derived(status === 'deleted')
  let category = $state('')
  let loading = $state(true)


  let editing = $state<{ id: number; name: string; price: number | null; category: string; color: string } | null>(null)
  let priceInvalid = $state(false)
  const form = new FormState(() => ({
    name: required(editing?.name, 'Nama tiket wajib diisi') ?? maxLength(editing?.name ?? '', 100, 'Nama tiket maksimal 100 karakter'),
    price: priceInvalid ? 'Angka terlalu besar' : required(editing?.price, 'Harga wajib diisi')
  }))
  let saving = $state(false)
  let confirmDelete = $state<model.Ticket | null>(null)
  let confirmActive = $state<model.Ticket | null>(null)
  // A delete, (de)activation or restore is in progress; blocks repeat clicks.
  let busy = $state(false)

  // The ticket just added or edited: shown on this page (at the top when it
  // is not already here) and blinked green, if it fits the status/category filter.
  let saved: { ticket: model.Ticket; isNew: boolean } | null = null
  let flash = $state<number | null>(null)
  // No column means the cashier-screen order: tickets before topups, then by name.
  let sort = $state<Sort>({ key: '', dir: 'asc' })

  function fitsFilter(t: model.Ticket) {
    if (status === 'deleted') return false
    if (status === 'active' && !t.active) return false
    if (status === 'inactive' && t.active) return false
    return !category || t.category === category
  }

  // Filters, search and paging run on the server.
  async function load() {
    const seq = ++requestSeq
    loading = true
    try {
      const out = await App.SearchTickets(model.TicketQuery.createFrom({ search: appliedSearch, category, status, page, pageSize, sort: sort.key, dir: sort.dir }))
      if (seq !== requestSeq) return
      // The last page can empty out after a delete or status change.
      if (out.tickets.length === 0 && out.count > 0 && page > 1) {
        page = Math.ceil(out.count / pageSize)
        return load()
      }
      if (saved && fitsFilter(saved.ticket)) {
        const t = saved.ticket
        const onPage = out.tickets.some((x) => x.id === t.id)
        if (saved.isNew || !onPage) out.tickets = [t, ...out.tickets.filter((x) => x.id !== t.id)]
        flash = t.id
        setTimeout(() => (flash = null), 2000)
      }
      saved = null
      result = out
    } catch (e) {
      if (seq === requestSeq) handleError(e)
    } finally {
      if (seq === requestSeq) loading = false
    }
  }

  function goto(p: number) {
    page = p
    load()
  }

  $effect(() => {
    const value = search.trim()
    const timer = setTimeout(() => (appliedSearch = value), 250)
    return () => clearTimeout(timer)
  })

  // New filters start again from page 1.
  $effect(() => {
    appliedSearch
    category
    status
    sort
    untrack(() => {
      page = 1
      load()
    })
  })

  function openForm(t?: model.Ticket) {
    form.reset()
    editing = t
      ? { id: t.id, name: t.name, price: t.price, category: t.category, color: t.color }
      : { id: 0, name: '', price: null, category: category || 'ticket', color: 'orange' }
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    if (!editing || saving || !form.check()) return
    saving = true
    try {
      const ticket = await App.SaveTicket(model.TicketInput.createFrom({ id: editing.id, name: editing.name, price: editing.price, category: editing.category, color: editing.color }))
      toast(editing.id ? 'Tiket diperbarui' : 'Tiket ditambahkan')
      saved = { ticket, isNew: !editing.id }
      editing = null
      load()
    } catch (e) {
      form.fail(errorMessage(e), { name: /nama tiket/i, price: /harga/i, category: /kategori/i, color: /warna/i })
    } finally {
      saving = false
    }
  }

  async function remove() {
    if (!confirmDelete || busy) return
    busy = true
    try {
      await App.DeleteTicket(confirmDelete.id)
      toast('Tiket dihapus')
      confirmDelete = null
      load()
    } catch (e) {
      handleError(e)
    } finally {
      busy = false
    }
  }

  async function toggleActive() {
    if (!confirmActive || busy) return
    const t = confirmActive
    busy = true
    try {
      await App.SetTicketActive(t.id, !t.active)
      toast(`${t.name} ${t.active ? 'dinon-aktifkan' : 'diaktifkan'}`)
      confirmActive = null
      load()
    } catch (e) {
      handleError(e)
    } finally {
      busy = false
    }
  }

  async function restore(t: model.Ticket) {
    if (busy) return
    busy = true
    try {
      await App.RestoreTicket(t.id)
      toast('Tiket dipulihkan')
      load()
    } catch (e) {
      handleError(e)
    } finally {
      busy = false
    }
  }
</script>

<div class="flex h-full flex-col p-6">
  <div class="flex items-center justify-between">
    <h1 class="text-2xl font-bold">Tiket</h1>
    <button class="btn btn-primary" onclick={() => openForm()}>+ Tambah Tiket</button>
  </div>

  <div class="mt-4 flex items-center gap-4">
    <input class="input max-w-sm" placeholder="Cari nama tiket…" bind:value={search} />
    <div class="flex rounded-md border border-slate-300 bg-white p-0.5 text-sm">
      {#each statuses as st}
        <button class="rounded px-3 py-1.5 {status === st.value ? 'bg-brand-600 text-white' : 'text-slate-600'}" onclick={() => (status = st.value)}>{st.label}</button>
      {/each}
    </div>
    <div class="flex rounded-md border border-slate-300 bg-white p-0.5 text-sm">
      {#each [{ value: '', label: 'Semua' }, ...categories] as c}
        <button class="rounded px-3 py-1.5 {category === c.value ? 'bg-brand-600 text-white' : 'text-slate-600'}" onclick={() => (category = c.value)}>{c.label}</button>
      {/each}
    </div>
  </div>

  <div class="relative mt-4 flex min-h-0 flex-1 flex-col">
  <div class="card min-h-40 flex-1 overflow-y-auto">
    <table class="table">
      <thead class="sticky top-0">
        <tr>
          <SortHeader label="Nama" key="name" {sort} onsort={(s) => (sort = s)} />
          <SortHeader label="Kategori" key="category" {sort} onsort={(s) => (sort = s)} />
          <SortHeader label="Harga" key="price" {sort} onsort={(s) => (sort = s)} align="right" firstDir="desc" />
          {#if !showDeleted}<SortHeader label="Status" key="active" {sort} onsort={(s) => (sort = s)} firstDir="desc" />{/if}
          <SortHeader label="Dibuat" key="created" {sort} onsort={(s) => (sort = s)} firstDir="desc" />
          <SortHeader label="Diubah" key="updated" {sort} onsort={(s) => (sort = s)} firstDir="desc" />
          {#if showDeleted}<SortHeader label="Dihapus" key="deleted" {sort} onsort={(s) => (sort = s)} firstDir="desc" />{/if}
          <th class="w-72"></th>
        </tr>
      </thead>
      <tbody>
        {#if !loading && tickets.length === 0}
          <tr><td colspan="7" class="py-10 text-center text-slate-500">{appliedSearch || category ? 'Tiket tidak ditemukan.' : showDeleted ? 'Tidak ada tiket terhapus.' : status === 'inactive' ? 'Tidak ada tiket non-aktif.' : status === 'active' ? 'Tidak ada tiket aktif.' : 'Belum ada tiket.'}</td></tr>
        {/if}
        {#each tickets as t (t.id)}
          <tr class="hover:bg-slate-50 {!showDeleted && !t.active ? 'text-slate-400' : ''} {t.id === flash ? 'row-flash' : ''}">
            <td class="font-medium">
              <span class="inline-flex items-center gap-2"><span class="h-3.5 w-3.5 shrink-0 rounded-sm {colorClass(t.color)} {!showDeleted && !t.active ? 'opacity-40' : ''}"></span>{t.name}</span>
            </td>
            <td>{categoryLabel(t.category)}</td>
            <td class="text-right">{formatRp(t.price)}</td>
            {#if !showDeleted}
              <td>
                {#if t.active}
                  <span class="rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700">Aktif</span>
                {:else}
                  <span class="rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-500">Non-aktif</span>
                {/if}
              </td>
            {/if}
            <td class="text-slate-500">{formatDateTime(t.createdAt)}</td>
            <td class="text-slate-500">{formatDateTime(t.updatedAt)}</td>
            {#if showDeleted}<td class="text-slate-500">{formatDateTime(t.deletedAt)}</td>{/if}
            <td class="text-right">
              {#if showDeleted}
                <button class="btn btn-secondary px-3 py-1" onclick={() => restore(t)} disabled={busy}>Pulihkan</button>
              {:else}
                <button class="btn btn-ghost px-3 py-1" onclick={() => (confirmActive = t)}>{t.active ? 'Non-aktifkan' : 'Aktifkan'}</button>
                <button class="btn btn-secondary px-3 py-1" onclick={() => openForm(t)}>Ubah</button>
                <button class="btn px-3 py-1 text-red-600 hover:bg-red-50" onclick={() => (confirmDelete = t)}>Hapus</button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
  <LoadingOverlay show={loading} />
  </div>
  {#if result}
    <Pager {page} {pageSize} count={result.count} {loading} ongoto={goto} />
  {/if}
</div>

{#if editing}
  <Modal title={editing.id ? 'Ubah Tiket' : 'Tambah Tiket'} onclose={() => (editing = null)}>
    <form id="ticket-form" class="space-y-4" onsubmit={save} novalidate>
      <FormAlert message={form.general} />
      <Field label="Nama tiket" id="name" {form} name="name">
        <input id="name" class="input" bind:value={editing.name} maxlength="100" />
      </Field>
      <Field label="Harga" id="price" {form} name="price" hint="Boleh pecahan, contoh 12.500,50">
        <MoneyInput id="price" bind:value={editing.price} bind:invalid={priceInvalid} />
      </Field>
      <Field label="Kategori" {form} name="category">
        <div class="flex w-fit rounded-md border border-slate-300 bg-white p-0.5 text-sm">
          {#each categories as c}
            <button type="button" class="rounded px-4 py-1.5 {editing.category === c.value ? 'bg-brand-600 text-white' : 'text-slate-600'}" onclick={() => editing && (editing.category = c.value)}>{c.label}</button>
          {/each}
        </div>
      </Field>
      <Field label="Warna" {form} name="color">
        <div class="flex flex-wrap gap-2">
          {#each colors as c}
            <button
              type="button"
              class="h-8 w-8 rounded-md {c.bg} {editing.color === c.value ? 'ring-2 ring-slate-800 ring-offset-2' : ''}"
              title={c.label}
              aria-label={c.label}
              onclick={() => editing && (editing.color = c.value)}
            ></button>
          {/each}
        </div>
        <div class="mt-3 flex min-h-24 w-40 flex-col items-center justify-center rounded-md px-3 py-4 text-center text-white shadow-sm {colorClass(editing.color)}">
          <span class="font-semibold break-words uppercase">{editing.name || 'Nama tiket'}</span>
          <span class="mt-1 text-sm opacity-90">({formatMoney(editing.price ?? 0)})</span>
        </div>
      </Field>
    </form>
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (editing = null)}>Batal</button>
      <button class="btn btn-primary" form="ticket-form" disabled={saving}>{saving ? 'Menyimpan…' : 'Simpan'}</button>
    {/snippet}
  </Modal>
{/if}

{#if confirmDelete}
  <Modal title="Hapus Tiket" onclose={() => (confirmDelete = null)} width="max-w-md">
    <p>Hapus tiket <b>{confirmDelete.name}</b>? Tiket tidak akan muncul di kasir, tapi riwayat penjualan tetap tersimpan. Tiket bisa dipulihkan dari tab Terhapus. Kalau hanya ingin berhenti dijual sementara, pakai <b>Non-aktifkan</b>.</p>
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (confirmDelete = null)}>Batal</button>
      <button class="btn btn-danger" onclick={remove} disabled={busy}>{busy ? 'Menghapus…' : 'Hapus'}</button>
    {/snippet}
  </Modal>
{/if}

{#if confirmActive}
  <Modal title={confirmActive.active ? 'Non-aktifkan Tiket' : 'Aktifkan Tiket'} onclose={() => (confirmActive = null)} width="max-w-md">
    {#if confirmActive.active}
      <p>Non-aktifkan <b>{confirmActive.name}</b>? Tiket tidak muncul di kasir dan tidak bisa dijual sampai diaktifkan lagi. Riwayat penjualan tetap tersimpan.</p>
    {:else}
      <p>Aktifkan <b>{confirmActive.name}</b>? Tiket akan muncul lagi di kasir dan bisa dijual.</p>
    {/if}
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (confirmActive = null)}>Batal</button>
      <button class="btn {confirmActive?.active ? 'btn-danger' : 'btn-primary'}" onclick={toggleActive} disabled={busy}>{busy ? 'Menyimpan…' : confirmActive?.active ? 'Non-aktifkan' : 'Aktifkan'}</button>
    {/snippet}
  </Modal>
{/if}
