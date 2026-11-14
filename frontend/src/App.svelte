<script lang="ts">
  import Setup from './views/Setup.svelte'
  import FirstAdmin from './views/FirstAdmin.svelte'
  import Login from './views/Login.svelte'
  import Shell from './views/Shell.svelte'
  import Modal from './components/Modal.svelte'
  import Toasts from './components/Toasts.svelte'
  import UpdateBanner from './views/UpdateBanner.svelte'
  import { App, errorMessage } from './lib/api'
  import { session } from './lib/session.svelte'
  import { watchUpdates } from './lib/update.svelte'

  type Screen = 'loading' | 'setup' | 'error' | 'first-admin' | 'no-admin' | 'login' | 'main'

  let screen = $state<Screen>('loading')
  let error = $state('')
  let serverName = $state('')
  let confirmReset = $state(false)

  async function boot() {
    screen = 'loading'
    error = ''
    try {
      session.info = await App.GetAppInfo()
      if (!session.info.mode) {
        screen = 'setup'
        return
      }
      if (!session.info.ready) {
        error = session.info.startError
        screen = 'error'
        return
      }
      const status = await App.Status()
      if (!status.hasUsers) {
        // The first admin is made on the server PC. A client only says so:
        // it may be pointing at the wrong PC.
        serverName = status.name || ''
        screen = session.info.mode === 'client' ? 'no-admin' : 'first-admin'
        return
      }
      session.user = await App.CurrentUser()
      screen = session.user ? 'main' : 'login'
    } catch (e) {
      error = errorMessage(e)
      screen = 'error'
    }
  }

  let resetting = false

  async function resetSetup() {
    if (resetting) return
    resetting = true
    confirmReset = false
    // A client asks its server first, which takes a while when it is off.
    screen = 'loading'
    try {
      await App.ResetSetup()
      session.user = null
      boot()
    } catch (e) {
      error = errorMessage(e)
      screen = 'error'
    } finally {
      resetting = false
    }
  }

  $effect(() => {
    boot()
  })

  $effect(() => {
    if (screen === 'main' && session.user === null) screen = 'login'
  })

  // Looked for from the start, so a client that the server no longer accepts
  // is offered the server's version on the login screen already.
  let canUpdate = $derived(!!session.info?.mode && !!session.info?.ready)
  $effect(() => {
    if (canUpdate) return watchUpdates()
  })
</script>

<div class="flex h-full flex-col">
<UpdateBanner />
<div class="min-h-0 flex-1">
{#if screen === 'loading'}
  <div class="flex h-full items-center justify-center text-slate-500">Memuat…</div>
{:else if screen === 'setup'}
  <Setup ondone={boot} />
{:else if screen === 'error'}
  <div class="flex h-full items-center justify-center p-6">
    <div class="card w-full max-w-lg p-8">
      <h1 class="text-xl font-bold text-red-700">Tidak bisa memulai aplikasi</h1>
      <p class="mt-3 text-slate-700">{error}</p>
      <div class="mt-6 flex gap-2">
        <button class="btn btn-primary" onclick={boot}>Coba lagi</button>
        <button class="btn btn-secondary" onclick={() => (confirmReset = true)}>Ubah mode server/client</button>
      </div>
    </div>
  </div>
{:else if screen === 'first-admin'}
  <FirstAdmin ondone={boot} onreset={() => (confirmReset = true)} />
{:else if screen === 'no-admin'}
  <div class="flex h-full items-center justify-center p-6">
    <div class="card w-full max-w-lg p-8">
      <h1 class="text-xl font-bold text-amber-700">Server belum punya akun admin</h1>
      <p class="mt-3 text-slate-700">
        Komputer ini Client dari server <b class="font-mono">{session.info?.serverUrl}</b>{serverName ? ` (${serverName})` : ''}.
        Server itu belum punya pengguna.
      </p>
      <p class="mt-2 text-slate-700">Buat akun admin di komputer server, atau periksa apakah alamatnya benar.</p>
      <div class="mt-6 flex gap-2">
        <button class="btn btn-primary" onclick={boot}>Coba lagi</button>
        <button class="btn btn-secondary" onclick={() => (confirmReset = true)}>Ubah mode server/client</button>
      </div>
    </div>
  </div>
{:else if screen === 'login'}
  <Login onlogin={() => (screen = 'main')} onlost={boot} />
{:else}
  <Shell onlogout={() => (screen = 'login')} onreset={boot} />
{/if}
</div>
</div>

{#if confirmReset}
  <Modal title="Ubah Mode" onclose={() => (confirmReset = false)} width="max-w-md">
    <p>Kembali ke Pengaturan Awal untuk memilih mode Server atau Client. Data di komputer ini <b>tidak dihapus</b>.</p>
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (confirmReset = false)}>Batal</button>
      <button class="btn btn-primary" onclick={resetSetup}>Lanjutkan</button>
    {/snippet}
  </Modal>
{/if}

<Toasts />
