<script lang="ts">
  import { App, errorMessage } from '../lib/api'
  import { session } from '../lib/session.svelte'

  let { onlogin, onreset }: { onlogin: () => void; onreset: () => void } = $props()

  let username = $state('')
  let password = $state('')
  let busy = $state(false)
  let error = $state('')
  let userEl: HTMLInputElement

  $effect(() => userEl?.focus())

  async function login(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    busy = true
    try {
      session.user = await App.Login(username, password)
      onlogin()
    } catch (e) {
      error = errorMessage(e)
      password = ''
    } finally {
      busy = false
    }
  }
</script>

<div class="flex min-h-full flex-col items-center justify-center bg-gradient-to-br from-brand-700 to-brand-500 p-6">
  <form class="card w-full max-w-sm p-8" onsubmit={login}>
    <h1 class="text-center text-3xl font-bold tracking-wide text-brand-700">TIKET</h1>
    <p class="mt-1 text-center text-sm text-slate-500">{session.info?.terminal}</p>
    <div class="mt-6 space-y-4">
      <div>
        <label class="label" for="u">Username</label>
        <input id="u" class="input" bind:this={userEl} bind:value={username} required autocomplete="off" />
      </div>
      <div>
        <label class="label" for="p">Password</label>
        <input id="p" class="input" type="password" bind:value={password} required />
      </div>
      {#if error}<p class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>{/if}
    </div>
    <button class="btn btn-primary mt-6 w-full py-2.5" disabled={busy}>{busy ? 'Masuk…' : 'Masuk'}</button>
  </form>
  <div class="mt-4 text-center text-xs text-white/80">
    Versi {session.info?.version} · Mode {session.info?.mode === 'server' ? 'Server' : `Client → ${session.info?.serverUrl}`}
    · <button class="underline" onclick={onreset}>Ubah mode</button>
  </div>
</div>
