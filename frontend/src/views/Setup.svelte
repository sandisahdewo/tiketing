<script lang="ts">
  import { App, errorMessage } from '../lib/api'
  import { session } from '../lib/session.svelte'
  import Field from '../components/Field.svelte'
  import { FormState, required, maxLength } from '../lib/form.svelte'

  let { ondone }: { ondone: () => void } = $props()

  let mode = $state<'server' | 'client'>('server')
  let port = $state(session.info?.port || 8787)
  let terminal = $state(session.info?.terminal || 'LOKET 1')
  let serverUrl = $state(session.info?.serverUrl || '')
  let busy = $state(false)
  let testResult = $state('')

  const form = new FormState(() => ({
    terminal: mode === 'server' ? (required(terminal, 'Nama loket wajib diisi') ?? maxLength(terminal, 30, 'Nama loket maksimal 30 karakter')) : undefined,
    port: mode === 'server' && !(Number.isInteger(port) && port >= 1024 && port <= 65535) ? 'Port harus angka antara 1024 dan 65535' : undefined,
    serverUrl: mode === 'client' ? required(serverUrl, 'Alamat server wajib diisi') : undefined
  }))
  // Connection problems are about the server address; the rest name their field.
  const serverFields = { port: /port/i, terminal: /terminal|loket/i, serverUrl: /server|terhubung|versi/i }

  async function test() {
    testResult = ''
    if (busy) return
    if (!serverUrl.trim()) {
      form.fail('Alamat server wajib diisi', { serverUrl: /./ })
      return
    }
    form.clear('serverUrl')
    busy = true
    try {
      const st = await App.TestServer(serverUrl)
      testResult = `Terhubung ke server versi ${st.version}.`
    } catch (e) {
      form.fail(errorMessage(e), serverFields)
    } finally {
      busy = false
    }
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    testResult = ''
    if (busy || !form.check()) return
    busy = true
    try {
      if (mode === 'server') {
        await App.SetupServer(Number(port), terminal)
      } else {
        await App.SetupClient(serverUrl)
      }
      ondone()
    } catch (e) {
      form.fail(errorMessage(e), serverFields)
    } finally {
      busy = false
    }
  }
</script>

<div class="flex min-h-full items-center justify-center p-6">
  <form class="card w-full max-w-2xl p-8" onsubmit={save} novalidate>
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
      {#if mode === 'server'}
        <Field label="Nama loket komputer ini" id="terminal" {form} name="terminal" hint="Loket pertama, dicetak di nota, contoh: LOKET 1. Loket lain dibuat admin di menu Loket.">
          <input id="terminal" class="input" bind:value={terminal} maxlength="30" />
        </Field>
      {/if}

      {#if mode === 'server'}
        {#snippet addresses()}
          Komputer client nanti terhubung ke:
          {#each session.info?.addresses ?? [] as ip, i}<b>{ip}:{port}</b>{i < (session.info?.addresses?.length ?? 0) - 1 ? ', ' : ''}{/each}
        {/snippet}
        <Field label="Port" id="port" {form} name="port" hint={session.info?.addresses?.length ? addresses : undefined}>
          <input id="port" class="input w-40" type="number" bind:value={port} />
        </Field>
      {:else}
        <Field label="Alamat server" id="server" {form} name="serverUrl" hint="Lihat alamatnya di menu Pengaturan pada komputer Server. Loket komputer ini dipilih admin setelah login, di menu Pengaturan.">
          <div class="flex gap-2">
            <input id="server" class="input" placeholder="192.168.1.10:8787" bind:value={serverUrl} />
            <button type="button" class="btn btn-secondary shrink-0" onclick={test} disabled={busy}>Tes koneksi</button>
          </div>
        </Field>
        {#if testResult}<p class="text-sm text-emerald-700">{testResult}</p>{/if}
      {/if}

      {#if form.general}<p class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{form.general}</p>{/if}
    </div>

    <div class="mt-8 flex justify-end">
      <button class="btn btn-primary px-6" disabled={busy}>{busy ? 'Menyimpan…' : 'Simpan & Lanjut'}</button>
    </div>
  </form>
</div>
