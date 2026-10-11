<script lang="ts">
  import { App, errorMessage } from '../lib/api'
  import { session } from '../lib/session.svelte'

  let { ondone }: { ondone: () => void } = $props()

  let mode = $state<'server' | 'client'>('server')
  let port = $state(session.info?.port || 8787)
  let terminal = $state(session.info?.terminal || 'LOKET 1')
  let serverUrl = $state(session.info?.serverUrl || '')
  let busy = $state(false)
  let error = $state('')
  let testResult = $state('')

  async function test() {
    error = ''
    testResult = ''
    busy = true
    try {
      const st = await App.TestServer(serverUrl)
      testResult = `Terhubung ke server versi ${st.version}.`
    } catch (e) {
      error = errorMessage(e)
    } finally {
      busy = false
    }
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    busy = true
    try {
      if (mode === 'server') {
        await App.SetupServer(Number(port), terminal)
      } else {
        await App.SetupClient(serverUrl, terminal)
      }
      ondone()
    } catch (e) {
      error = errorMessage(e)
    } finally {
      busy = false
    }
  }
</script>

<div class="flex min-h-full items-center justify-center p-6">
  <form class="card w-full max-w-2xl p-8" onsubmit={save}>
    <h1 class="text-2xl font-bold text-brand-700">Pengaturan Awal</h1>
    <p class="mt-1 text-sm text-slate-500">Pilih peran komputer ini. Pengaturan ini bisa diubah nanti oleh admin.</p>

    <div class="mt-6 grid grid-cols-2 gap-4">
      <button
        type="button"
        class="rounded-lg border-2 p-4 text-left transition {mode === 'server' ? 'border-brand-500 bg-brand-50' : 'border-slate-200 hover:border-slate-300'}"
        onclick={() => (mode = 'server')}
      >
        <div class="font-semibold">Server</div>
        <div class="mt-1 text-sm text-slate-500">Komputer ini menyimpan database. Pakai ini jika hanya 1 komputer, atau sebagai pusat untuk komputer lain.</div>
      </button>
      <button
        type="button"
        class="rounded-lg border-2 p-4 text-left transition {mode === 'client' ? 'border-brand-500 bg-brand-50' : 'border-slate-200 hover:border-slate-300'}"
        onclick={() => (mode = 'client')}
      >
        <div class="font-semibold">Client</div>
        <div class="mt-1 text-sm text-slate-500">Komputer ini terhubung ke komputer Server di jaringan yang sama (LAN/WiFi).</div>
      </button>
    </div>

    <div class="mt-6 space-y-4">
      <div>
        <label class="label" for="terminal">Nama loket / komputer</label>
        <input id="terminal" class="input" bind:value={terminal} maxlength="30" required />
        <p class="mt-1 text-xs text-slate-500">Dicetak di nota, contoh: LOKET 1</p>
      </div>

      {#if mode === 'server'}
        <div>
          <label class="label" for="port">Port</label>
          <input id="port" class="input w-40" type="number" min="1024" max="65535" bind:value={port} required />
          {#if session.info?.addresses?.length}
            <p class="mt-1 text-xs text-slate-500">
              Komputer client nanti terhubung ke:
              {#each session.info.addresses as ip, i}<b>{ip}:{port}</b>{i < session.info.addresses.length - 1 ? ', ' : ''}{/each}
            </p>
          {/if}
        </div>
      {:else}
        <div>
          <label class="label" for="server">Alamat server</label>
          <div class="flex gap-2">
            <input id="server" class="input" placeholder="192.168.1.10:8787" bind:value={serverUrl} required />
            <button type="button" class="btn btn-secondary shrink-0" onclick={test} disabled={busy || !serverUrl}>Tes koneksi</button>
          </div>
          <p class="mt-1 text-xs text-slate-500">Lihat alamatnya di menu Pengaturan pada komputer Server.</p>
          {#if testResult}<p class="mt-2 text-sm text-emerald-700">{testResult}</p>{/if}
        </div>
      {/if}

      {#if error}<p class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>{/if}
    </div>

    <div class="mt-8 flex justify-end">
      <button class="btn btn-primary px-6" disabled={busy}>{busy ? 'Menyimpan…' : 'Simpan & Lanjut'}</button>
    </div>
  </form>
</div>
