<script lang="ts">
  import { App, errorMessage } from '../lib/api'
  import { session } from '../lib/session.svelte'
  import Field from '../components/Field.svelte'
  import { FormState, required } from '../lib/form.svelte'

  let { onlogin, onreset }: { onlogin: () => void; onreset: () => void } = $props()

  let username = $state('')
  let password = $state('')
  let busy = $state(false)
  const form = new FormState(() => ({
    username: required(username, 'Username wajib diisi'),
    password: required(password, 'Password wajib diisi')
  }))
  let userEl: HTMLInputElement

  $effect(() => userEl?.focus())

  async function login(e: SubmitEvent) {
    e.preventDefault()
    if (busy || !form.check()) return
    busy = true
    try {
      session.user = await App.Login(username, password)
      onlogin()
    } catch (e) {
      password = ''
      // A wrong login is reported under the password field.
      form.fail(errorMessage(e), { password: /username atau password|password/i, username: /username/i })
    } finally {
      busy = false
    }
  }
</script>

<div class="flex min-h-full flex-col items-center justify-center bg-gradient-to-br from-brand-700 to-brand-500 p-6">
  <form class="card w-full max-w-sm p-8" onsubmit={login} novalidate>
    <h1 class="text-center text-3xl font-bold tracking-wide text-brand-700">TIKET</h1>
    <p class="mt-1 text-center text-sm text-slate-500">{session.info?.terminal}</p>
    <div class="mt-6 space-y-4">
      <Field label="Username" id="u" {form} name="username">
        <input id="u" class="input" bind:this={userEl} bind:value={username} autocomplete="off" />
      </Field>
      <Field label="Password" id="p" {form} name="password">
        <input id="p" class="input" type="password" bind:value={password} />
      </Field>
      {#if form.general}<p class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{form.general}</p>{/if}
    </div>
    <button class="btn btn-primary mt-6 w-full py-2.5" disabled={busy}>{busy ? 'Masuk…' : 'Masuk'}</button>
  </form>
  <div class="mt-4 text-center text-xs text-white/80">
    Versi {session.info?.version} · Mode {session.info?.mode === 'server' ? 'Server' : `Client → ${session.info?.serverUrl}`}
    · <button class="underline" onclick={onreset}>Ubah mode</button>
  </div>
</div>
