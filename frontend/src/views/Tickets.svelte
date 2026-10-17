<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import MoneyInput from '../components/MoneyInput.svelte'
  import { App, model, handleError, errorMessage } from '../lib/api'
  import { formatMoney, formatRp } from '../lib/money'
  import { categories, categoryLabel, colors, colorClass } from '../lib/colors'
  import { formatDateTime } from '../lib/date'
  import { toast } from '../lib/toast.svelte'

  let tickets = $state<model.Ticket[]>([])
  let search = $state('')
  let showDeleted = $state(false)
  let category = $state('')
  let loading = $state(true)

  let visible = $derived(category ? tickets.filter((t) => t.category === category) : tickets)

  let editing = $state<{ id: number; name: string; price: number | null; category: string; color: string } | null>(null)
  let formError = $state('')
  let saving = $state(false)
  let confirmDelete = $state<model.Ticket | null>(null)

  async function load() {
    loading = true
    try {
      tickets = await App.ListTickets(search, showDeleted)
    } catch (e) {
      handleError(e)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    showDeleted
    const timer = setTimeout(load, 200)
    search
    return () => clearTimeout(timer)
  })

  function openForm(t?: model.Ticket) {
    formError = ''
    editing = t
      ? { id: t.id, name: t.name, price: t.price, category: t.category, color: t.color }
      : { id: 0, name: '', price: null, category: category || 'ticket', color: 'orange' }
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    if (!editing) return
    if (editing.price == null) {
      formError = 'Harga wajib diisi dengan angka yang valid'
      return
    }
    saving = true
    formError = ''
    try {
      await App.SaveTicket(model.TicketInput.createFrom({ id: editing.id, name: editing.name, price: editing.price, category: editing.category, color: editing.color }))
      toast(editing.id ? 'Tiket diperbarui' : 'Tiket ditambahkan')
      editing = null
      load()
    } catch (e) {
      formError = errorMessage(e)
    } finally {
      saving = false
    }
  }

  async function remove() {
    if (!confirmDelete) return
    try {
      await App.DeleteTicket(confirmDelete.id)
      toast('Tiket dihapus')
      confirmDelete = null
      load()
    } catch (e) {
      handleError(e)
    }
  }

  async function restore(t: model.Ticket) {
    try {
      await App.RestoreTicket(t.id)
      toast('Tiket dipulihkan')
      load()
    } catch (e) {
      handleError(e)
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
      <button class="rounded px-3 py-1.5 {!showDeleted ? 'bg-brand-600 text-white' : 'text-slate-600'}" onclick={() => (showDeleted = false)}>Aktif</button>
      <button class="rounded px-3 py-1.5 {showDeleted ? 'bg-brand-600 text-white' : 'text-slate-600'}" onclick={() => (showDeleted = true)}>Terhapus</button>
    </div>
    <div class="flex rounded-md border border-slate-300 bg-white p-0.5 text-sm">
      {#each [{ value: '', label: 'Semua' }, ...categories] as c}
        <button class="rounded px-3 py-1.5 {category === c.value ? 'bg-brand-600 text-white' : 'text-slate-600'}" onclick={() => (category = c.value)}>{c.label}</button>
      {/each}
    </div>
  </div>

  <div class="card mt-4 min-h-0 flex-1 overflow-y-auto">
    <table class="table">
      <thead class="sticky top-0">
        <tr>
          <th>Nama</th>
          <th>Kategori</th>
          <th class="text-right">Harga</th>
          <th>Dibuat</th>
          <th>Diubah</th>
          {#if showDeleted}<th>Dihapus</th>{/if}
          <th class="w-48"></th>
        </tr>
      </thead>
      <tbody>
        {#if loading && tickets.length === 0}
          <tr><td colspan="7" class="py-10 text-center text-slate-500">Memuat…</td></tr>
        {:else if visible.length === 0}
          <tr><td colspan="7" class="py-10 text-center text-slate-500">{showDeleted ? 'Tidak ada tiket terhapus.' : 'Belum ada tiket.'}</td></tr>
        {/if}
        {#each visible as t (t.id)}
          <tr class="hover:bg-slate-50">
            <td class="font-medium">
              <span class="inline-flex items-center gap-2"><span class="h-3.5 w-3.5 shrink-0 rounded-sm {colorClass(t.color)}"></span>{t.name}</span>
            </td>
            <td>{categoryLabel(t.category)}</td>
            <td class="text-right">{formatRp(t.price)}</td>
            <td class="text-slate-500">{formatDateTime(t.createdAt)}</td>
            <td class="text-slate-500">{formatDateTime(t.updatedAt)}</td>
            {#if showDeleted}<td class="text-slate-500">{formatDateTime(t.deletedAt)}</td>{/if}
            <td class="text-right">
              {#if showDeleted}
                <button class="btn btn-secondary px-3 py-1" onclick={() => restore(t)}>Pulihkan</button>
              {:else}
                <button class="btn btn-secondary px-3 py-1" onclick={() => openForm(t)}>Ubah</button>
                <button class="btn px-3 py-1 text-red-600 hover:bg-red-50" onclick={() => (confirmDelete = t)}>Hapus</button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>

{#if editing}
  <Modal title={editing.id ? 'Ubah Tiket' : 'Tambah Tiket'} onclose={() => (editing = null)}>
    <form id="ticket-form" class="space-y-4" onsubmit={save}>
      <div>
        <label class="label" for="name">Nama tiket</label>
        <input id="name" class="input" bind:value={editing.name} required maxlength="100" />
      </div>
      <div>
        <label class="label" for="price">Harga</label>
        <MoneyInput id="price" bind:value={editing.price} />
        <p class="mt-1 text-xs text-slate-500">Boleh pecahan, contoh 12.500,50</p>
      </div>
      <div>
        <span class="label">Kategori</span>
        <div class="flex w-fit rounded-md border border-slate-300 bg-white p-0.5 text-sm">
          {#each categories as c}
            <button type="button" class="rounded px-4 py-1.5 {editing.category === c.value ? 'bg-brand-600 text-white' : 'text-slate-600'}" onclick={() => editing && (editing.category = c.value)}>{c.label}</button>
          {/each}
        </div>
      </div>
      <div>
        <span class="label">Warna</span>
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
      </div>
      {#if formError}<p class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{formError}</p>{/if}
    </form>
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (editing = null)}>Batal</button>
      <button class="btn btn-primary" form="ticket-form" disabled={saving}>{saving ? 'Menyimpan…' : 'Simpan'}</button>
    {/snippet}
  </Modal>
{/if}

{#if confirmDelete}
  <Modal title="Hapus Tiket" onclose={() => (confirmDelete = null)} width="max-w-md">
    <p>Hapus tiket <b>{confirmDelete.name}</b>? Tiket tidak akan muncul di kasir, tapi riwayat penjualan tetap tersimpan. Tiket bisa dipulihkan dari tab Terhapus.</p>
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (confirmDelete = null)}>Batal</button>
      <button class="btn btn-danger" onclick={remove}>Hapus</button>
    {/snippet}
  </Modal>
{/if}
