<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import { App, model, handleError, errorMessage } from '../lib/api'
  import { formatDateTime } from '../lib/date'
  import { session } from '../lib/session.svelte'
  import { toast } from '../lib/toast.svelte'

  let users = $state<model.User[]>([])
  let loading = $state(true)
  let editing = $state<{ id: number; username: string; fullName: string; role: string; password: string } | null>(null)
  let formError = $state('')
  let saving = $state(false)
  let confirmDelete = $state<model.User | null>(null)

  async function load() {
    loading = true
    try {
      users = await App.ListUsers()
    } catch (e) {
      handleError(e)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    load()
  })

  function openForm(u?: model.User) {
    formError = ''
    editing = u
      ? { id: u.id, username: u.username, fullName: u.fullName, role: u.role, password: '' }
      : { id: 0, username: '', fullName: '', role: 'kasir', password: '' }
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    if (!editing || saving) return
    saving = true
    formError = ''
    try {
      const saved = await App.SaveUser(model.UserInput.createFrom(editing))
      if (session.user && saved.id === session.user.id) session.user = saved
      toast(editing.id ? 'Pengguna diperbarui' : 'Pengguna ditambahkan')
      editing = null
      load()
    } catch (e) {
      formError = errorMessage(e)
    } finally {
      saving = false
    }
  }

  let deleting = $state(false)

  async function remove() {
    if (!confirmDelete || deleting) return
    deleting = true
    try {
      await App.DeleteUser(confirmDelete.id)
      toast('Pengguna dihapus')
      confirmDelete = null
      load()
    } catch (e) {
      handleError(e)
    } finally {
      deleting = false
    }
  }
</script>

<div class="flex h-full flex-col p-6">
  <div class="flex items-center justify-between">
    <h1 class="text-2xl font-bold">Pengguna</h1>
    <button class="btn btn-primary" onclick={() => openForm()}>+ Tambah Pengguna</button>
  </div>

  <div class="card mt-4 min-h-0 flex-1 overflow-y-auto">
    <table class="table">
      <thead class="sticky top-0">
        <tr><th>Username</th><th>Nama</th><th>Role</th><th>Dibuat</th><th>Diubah</th><th class="w-48"></th></tr>
      </thead>
      <tbody>
        {#if loading && users.length === 0}
          <tr><td colspan="6" class="py-10 text-center text-slate-500">Memuat…</td></tr>
        {/if}
        {#each users as u (u.id)}
          <tr class="hover:bg-slate-50">
            <td class="font-mono">{u.username}</td>
            <td class="font-medium">{u.fullName}{#if u.id === session.user?.id} <span class="text-xs text-slate-400">(Anda)</span>{/if}</td>
            <td>
              <span class="rounded-full px-2 py-0.5 text-xs font-semibold {u.role === 'admin' ? 'bg-brand-100 text-brand-700' : 'bg-amber-100 text-amber-700'}">{u.role === 'admin' ? 'Admin' : 'Kasir'}</span>
            </td>
            <td class="text-slate-500">{formatDateTime(u.createdAt)}</td>
            <td class="text-slate-500">{formatDateTime(u.updatedAt)}</td>
            <td class="text-right">
              <button class="btn btn-secondary px-3 py-1" onclick={() => openForm(u)}>Ubah</button>
              {#if u.id !== session.user?.id}
                <button class="btn px-3 py-1 text-red-600 hover:bg-red-50" onclick={() => (confirmDelete = u)}>Hapus</button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>

{#if editing}
  <Modal title={editing.id ? 'Ubah Pengguna' : 'Tambah Pengguna'} onclose={() => (editing = null)}>
    <form id="user-form" class="space-y-4" onsubmit={save}>
      <div>
        <label class="label" for="username">Username</label>
        <input id="username" class="input" bind:value={editing.username} required autocomplete="off" />
        <p class="mt-1 text-xs text-slate-500">3-32 karakter: huruf kecil, angka, titik, minus, garis bawah</p>
      </div>
      <div>
        <label class="label" for="fullName">Nama lengkap</label>
        <input id="fullName" class="input" bind:value={editing.fullName} required />
      </div>
      <div>
        <label class="label" for="role">Role</label>
        <select id="role" class="input" bind:value={editing.role}>
          <option value="kasir">Kasir: hanya penjualan</option>
          <option value="admin">Admin: penjualan, tiket, pengguna, laporan</option>
        </select>
      </div>
      <div>
        <label class="label" for="password">{editing.id ? 'Password baru' : 'Password'}</label>
        <input id="password" class="input" type="password" bind:value={editing.password} required={!editing.id} autocomplete="new-password" />
        {#if editing.id}<p class="mt-1 text-xs text-slate-500">Kosongkan jika tidak ingin mengganti password</p>{/if}
      </div>
      {#if formError}<p class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{formError}</p>{/if}
    </form>
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (editing = null)}>Batal</button>
      <button class="btn btn-primary" form="user-form" disabled={saving}>{saving ? 'Menyimpan…' : 'Simpan'}</button>
    {/snippet}
  </Modal>
{/if}

{#if confirmDelete}
  <Modal title="Hapus Pengguna" onclose={() => (confirmDelete = null)} width="max-w-md">
    <p>Hapus pengguna <b>{confirmDelete.fullName}</b> ({confirmDelete.username})? Pengguna tidak bisa login lagi. Riwayat transaksinya tetap tersimpan.</p>
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (confirmDelete = null)}>Batal</button>
      <button class="btn btn-danger" onclick={remove} disabled={deleting}>{deleting ? 'Menghapus…' : 'Hapus'}</button>
    {/snippet}
  </Modal>
{/if}
