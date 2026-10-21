<script lang="ts">
  import { App, model, errorMessage } from '../lib/api'
  import Field from '../components/Field.svelte'
  import { FormState, required, maxLength, username as usernameRule } from '../lib/form.svelte'

  let { ondone }: { ondone: () => void } = $props()

  let username = $state('admin')
  let fullName = $state('')
  let password = $state('')
  let confirm = $state('')
  let busy = $state(false)
  const form = new FormState(() => ({
    username: usernameRule(username),
    fullName: required(fullName, 'Nama lengkap wajib diisi') ?? maxLength(fullName, 100, 'Nama lengkap maksimal 100 karakter'),
    password: password === '' ? 'Password wajib diisi' : [...password].length < 4 ? 'Password minimal 4 karakter' : undefined,
    confirm: required(confirm, 'Ulangi password') ?? (confirm !== password ? 'Konfirmasi password tidak sama' : undefined)
  }))

  async function save(e: SubmitEvent) {
    e.preventDefault()
    if (busy || !form.check()) return
    busy = true
    try {
      await App.CreateFirstAdmin(model.UserInput.createFrom({ id: 0, username, fullName, role: 'admin', password }))
      ondone()
    } catch (e) {
      form.fail(errorMessage(e), { username: /username/i, fullName: /nama lengkap/i, password: /password/i })
    } finally {
      busy = false
    }
  }
</script>

<div class="flex min-h-full items-center justify-center p-6">
  <form class="card w-full max-w-md p-8" onsubmit={save} novalidate>
    <h1 class="text-2xl font-bold text-brand-700">Buat Akun Admin</h1>
    <p class="mt-1 text-sm text-slate-500">Belum ada pengguna. Buat akun admin pertama untuk mengelola tiket, pengguna, dan laporan.</p>
    <div class="mt-6 space-y-4">
      <Field label="Username" id="u" {form} name="username" hint="3-32 karakter: huruf kecil, angka, titik, minus, garis bawah">
        <input id="u" class="input" bind:value={username} autocomplete="off" />
      </Field>
      <Field label="Nama lengkap" id="n" {form} name="fullName">
        <input id="n" class="input" bind:value={fullName} maxlength="100" />
      </Field>
      <Field label="Password" id="p" {form} name="password" hint="Minimal 4 karakter">
        <input id="p" class="input" type="password" bind:value={password} />
      </Field>
      <Field label="Ulangi password" id="c" {form} name="confirm">
        <input id="c" class="input" type="password" bind:value={confirm} />
      </Field>
      {#if form.general}<p class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{form.general}</p>{/if}
    </div>
    <button class="btn btn-primary mt-6 w-full py-2.5" disabled={busy}>{busy ? 'Menyimpan…' : 'Buat Admin'}</button>
  </form>
</div>
