<script lang="ts">
  import { App, errorMessage } from '../lib/api'
  import { session } from '../lib/session.svelte'
  import type { discover } from '../../wailsjs/go/models'
  import Field from '../components/Field.svelte'
  import FormAlert from '../components/FormAlert.svelte'
  import { FormState, required } from '../lib/form.svelte'

  let { ondone }: { ondone: () => void } = $props()

  let mode = $state<'server' | 'client'>('server')
  let port = $state(session.info?.port || 8787)
  let serverUrl = $state(session.info?.serverUrl || '')
  let busy = $state(false)
  let testResult = $state('')
  let testWarning = $state('')
  const noAdminWarning = 'Server ini belum punya akun admin. Pastikan alamatnya benar; akun admin dibuat di komputer server.'

  // The servers found on this PC's network (null until it has been searched).
  let servers = $state<discover.Server[] | null>(null)
  let searching = $state(false)
  // The user typed an address, so the search leaves the field alone.
  let typed = $state(false)
  // The address in the field without its scheme, as the list shows it.
  let chosen = $derived(serverUrl.trim().replace(/^https?:\/\//, '').replace(/\/+$/, ''))

  function pick(s: discover.Server) {
    serverUrl = s.address
    form.clear('serverUrl')
    testResult = ''
    testWarning = s.hasUsers ? '' : noAdminWarning
  }

  async function search() {
    if (searching) return
    searching = true
    try {
      // The address in the field is asked too: a server that still answers
      // there is in the list, also when it uses another port.
      servers = (await App.FindServers(chosen)) ?? []
      // There is only one server: no need to make the user pick it. It also
      // replaces a saved address that no server answers on any more (the
      // server got another address); not one the user typed here.
      const usable = servers.filter((s) => s.compatible)
      const answers = servers.some((s) => s.address === chosen)
      if (!typed && !answers && usable.length === 1) pick(usable[0])
    } catch {
      servers = []
    } finally {
      searching = false
    }
  }

  $effect(() => {
    if (mode === 'client' && servers === null) search()
  })

  const form = new FormState(() => ({
    port: mode === 'server' && !(Number.isInteger(port) && port >= 1024 && port <= 65535) ? 'Port harus angka antara 1024 dan 65535' : undefined,
    serverUrl: mode === 'client' ? required(serverUrl, 'Alamat server wajib diisi') : undefined
  }))
  // Only input problems go under a field; a server that cannot be reached is
  // shown above the fields.
  const serverFields = { port: /port/i, serverUrl: /alamat server/i }

  async function test() {
    testResult = ''
    testWarning = ''
    if (busy) return
    if (!serverUrl.trim()) {
      form.fail('Alamat server wajib diisi', { serverUrl: /./ })
      return
    }
    form.clear('serverUrl')
    busy = true
    try {
      const st = await App.TestServer(serverUrl)
      testResult = st.name ? `Terhubung ke server ${st.name}, versi ${st.version}.` : `Terhubung ke server versi ${st.version}.`
      if (!st.hasUsers) testWarning = noAdminWarning
    } catch (e) {
      form.fail(errorMessage(e), serverFields)
    } finally {
      busy = false
    }
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    testResult = ''
    testWarning = ''
    if (busy || !form.check()) return
    busy = true
    try {
      if (mode === 'server') {
        await App.SetupServer(Number(port))
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
      <FormAlert message={form.general} />

      {#if mode === 'server'}
        {#snippet addresses()}
          Komputer client nanti terhubung ke:
          {#each session.info?.addresses ?? [] as ip, i}<b>{ip}:{port}</b>{i < (session.info?.addresses?.length ?? 0) - 1 ? ', ' : ''}{/each}
        {/snippet}
        <Field label="Port" id="port" {form} name="port" hint={session.info?.addresses?.length ? addresses : undefined}>
          <input id="port" class="input w-40" type="number" bind:value={port} />
        </Field>
      {:else}
        <div>
          <div class="flex items-center justify-between">
            <span class="label">Server di jaringan ini</span>
            <button type="button" class="btn btn-ghost px-2 py-1 text-sm" onclick={search} disabled={searching}>Cari lagi</button>
          </div>
          {#if searching || servers === null}
            <p class="rounded-lg border border-slate-200 px-4 py-3 text-sm text-slate-500">Mencari server di jaringan…</p>
          {:else if servers.length}
            <div class="space-y-2">
              {#each servers as s (s.address)}
                <button
                  type="button"
                  class="flex w-full items-center justify-between gap-3 rounded-lg border-2 px-4 py-3 text-left transition disabled:opacity-60 {chosen === s.address ? 'border-brand-500 bg-brand-50' : 'border-slate-200 enabled:hover:border-slate-300'}"
                  disabled={!s.compatible}
                  onclick={() => pick(s)}
                >
                  <div>
                    <div class="font-semibold">{s.name || 'Server'}</div>
                    <div class="font-mono text-sm text-slate-500">{s.address}</div>
                  </div>
                  <div class="text-right text-sm">
                    <div class="text-slate-500">Versi {s.version}</div>
                    {#if !s.compatible}
                      <div class="text-red-700">Tidak cocok dengan aplikasi ini ({session.info?.version}). Update keduanya.</div>
                    {:else if !s.hasUsers}
                      <div class="text-amber-700">Belum punya akun admin</div>
                    {/if}
                  </div>
                </button>
              {/each}
            </div>
          {:else}
            <div class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
              <p class="font-semibold">Server tidak ditemukan di jaringan ini. Periksa:</p>
              <ol class="mt-2 list-decimal space-y-1 pl-5">
                <li>Aplikasi Tiket di komputer server sedang terbuka dan sudah diatur sebagai <b>Server</b>.</li>
                {#if session.info?.addresses?.length}
                  <li>
                    Komputer ini dan komputer server terhubung ke router atau WiFi yang sama. Alamat komputer ini
                    <b class="font-mono">{session.info.addresses.join(', ')}</b>; alamat server (lihat menu Pengaturan di komputer server) harus
                    sama di tiga angka pertamanya.
                  </li>
                {:else}
                  <li><b>Komputer ini belum terhubung ke jaringan.</b> Sambungkan kabel LAN atau WiFi ke router yang sama dengan komputer server.</li>
                {/if}
                <li>Windows Firewall di komputer server mengizinkan Tiket: saat Tiket pertama kali dibuka di sana, pilih <b>Allow</b> untuk jaringan Private.</li>
                <li>Kalau server memakai port selain 8787, server tidak muncul di daftar ini: isi alamatnya sendiri di bawah.</li>
              </ol>
              <p class="mt-2">Setelah itu klik <b>Cari lagi</b>, atau isi alamat server di bawah lalu klik <b>Tes koneksi</b>.</p>
            </div>
          {/if}
        </div>
        <Field label="Alamat server" id="server" {form} name="serverUrl" hint="Terisi saat server dipilih dari daftar. Bisa juga diisi sendiri: alamatnya ada di menu Pengaturan pada komputer Server.">
          <div class="flex gap-2">
            <input id="server" class="input" placeholder="192.168.1.10:8787" bind:value={serverUrl} oninput={() => (typed = serverUrl.trim() !== '')} />
            <button type="button" class="btn btn-secondary shrink-0" onclick={test} disabled={busy}>Tes koneksi</button>
          </div>
        </Field>
        {#if testResult}<p class="text-sm text-emerald-700">{testResult}</p>{/if}
        {#if testWarning}<p class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800" role="alert">{testWarning}</p>{/if}
      {/if}

    </div>

    <p class="mt-6 text-sm text-slate-500">Loket komputer ini dibuat dan dipilih oleh admin setelah login, di menu Loket dan Pengaturan.</p>

    <div class="mt-8 flex justify-end">
      <button class="btn btn-primary px-6" disabled={busy}>{busy ? 'Menyimpan…' : 'Simpan & Lanjut'}</button>
    </div>
  </form>
</div>
