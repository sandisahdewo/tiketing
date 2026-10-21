<script lang="ts">
  import { App, model, errorMessage } from '../lib/api'

  let { ondone }: { ondone: () => void } = $props()

  let username = $state('admin')
  let fullName = $state('')
  let password = $state('')
  let confirm = $state('')
  let busy = $state(false)
  let error = $state('')

  async function save(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    if (password !== confirm) {
      error = 'Konfirmasi password tidak sama'
      return
    }
    if (busy) return
    busy = true
    try {
      await App.CreateFirstAdmin(model.UserInput.createFrom({ id: 0, username, fullName, role: 'admin', password }))
      ondone()
    } catch (e) {
      error = errorMessage(e)
    } finally {
      busy = false
    }
  }
</script>

<div class="flex min-h-full items-center justify-center p-6">
  <form class="card w-full max-w-md p-8" onsubmit={save}>
    <h1 class="text-2xl font-bold text-brand-700">Buat Akun Admin</h1>
    <p class="mt-1 text-sm text-slate-500">Belum ada pengguna. Buat akun admin pertama untuk mengelola tiket, pengguna, dan laporan.</p>
    <div class="mt-6 space-y-4">
      <div>
        <label class="label" for="u">Username</label>
        <input id="u" class="input" bind:value={username} required autocomplete="off" />
      </div>
      <div>
        <label class="label" for="n">Nama lengkap</label>
        <input id="n" class="input" bind:value={fullName} required />
      </div>
      <div>
        <label class="label" for="p">Password</label>
        <input id="p" class="input" type="password" bind:value={password} required minlength="4" />
      </div>
      <div>
        <label class="label" for="c">Ulangi password</label>
        <input id="c" class="input" type="password" bind:value={confirm} required />
      </div>
      {#if error}<p class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>{/if}
    </div>
    <button class="btn btn-primary mt-6 w-full py-2.5" disabled={busy}>{busy ? 'Menyimpan…' : 'Buat Admin'}</button>
  </form>
</div>
