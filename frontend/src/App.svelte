<script lang="ts">
  import Setup from './views/Setup.svelte'
  import FirstAdmin from './views/FirstAdmin.svelte'
  import Login from './views/Login.svelte'
  import Shell from './views/Shell.svelte'
  import Modal from './components/Modal.svelte'
  import Toasts from './components/Toasts.svelte'
  import { App, errorMessage } from './lib/api'
  import { session } from './lib/session.svelte'

  type Screen = 'loading' | 'setup' | 'error' | 'first-admin' | 'login' | 'main'

  let screen = $state<Screen>('loading')
  let error = $state('')
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
        screen = 'first-admin'
        return
      }
      session.user = await App.CurrentUser()
      screen = session.user ? 'main' : 'login'
    } catch (e) {
      error = errorMessage(e)
      screen = 'error'
    }
  }

  async function resetSetup() {
    confirmReset = false
    try {
      await App.ResetSetup()
      session.user = null
      boot()
    } catch (e) {
      error = errorMessage(e)
      screen = 'error'
    }
  }

  $effect(() => {
    boot()
  })

  $effect(() => {
    if (screen === 'main' && session.user === null) screen = 'login'
  })
</script>

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
  <FirstAdmin ondone={boot} />
{:else if screen === 'login'}
  <Login onlogin={() => (screen = 'main')} onreset={() => (confirmReset = true)} />
{:else}
  <Shell onlogout={() => (screen = 'login')} onreset={boot} />
{/if}

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
