<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import { App, errorMessage } from '../lib/api'
  import { updateState } from '../lib/update.svelte'

  let error = $state('')
  let info = $derived(updateState.info)

  async function install() {
    error = ''
    if (updateState.installing) return
    updateState.installing = true
    try {
      await App.ApplyUpdate()
    } catch (e) {
      error = errorMessage(e)
      updateState.installing = false
    }
  }
</script>

{#if info?.available && info.mandatory}
  <Modal title="Update Wajib" closable={false} width="max-w-md">
    {#if info.fromServer}
      <p>Komputer server sudah memakai versi <b>{info.latest}</b> (komputer ini {info.current}). Pasang versi yang sama agar bisa bertransaksi lagi.</p>
      <p class="mt-2 text-sm text-slate-500">File diunduh dari komputer server, tidak perlu internet. Data tidak berubah.</p>
    {:else}
      <p>Versi <b>{info.latest}</b> wajib dipasang sebelum aplikasi bisa dipakai lagi.</p>
    {/if}
    {#if info.notes}<p class="mt-3 text-sm whitespace-pre-line text-slate-600">{info.notes}</p>{/if}
    {#if error}<p class="mt-3 rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>{/if}
    {#snippet footer()}
      <button class="btn btn-primary" onclick={install} disabled={updateState.installing}>{updateState.installing ? 'Mengunduh & memasang…' : 'Update Sekarang'}</button>
    {/snippet}
  </Modal>
{:else if info?.serverBehind}
  <div class="bg-red-100 px-4 py-2 text-sm text-red-900">
    <b>Komputer server masih versi {info.latest}</b>, komputer ini sudah {info.current}. Keduanya tidak bisa tersambung sampai komputer server diupdate.
  </div>
{:else if info?.manual}
  <div class="bg-red-100 px-4 py-2 text-sm text-red-900">
    <b>Komputer server sudah versi {info.latest}</b> (komputer ini {info.current}), tetapi file update tidak bisa diambil dari server. Pasang installer versi {info.latest} di komputer ini.
  </div>
{:else if info?.available && !updateState.dismissed}
  <div class="flex items-center gap-4 bg-amber-100 px-4 py-2 text-sm text-amber-900">
    <span class="flex-1">
      <b>Versi {info.latest} tersedia</b> (sekarang {info.current}){info.fromServer ? ', dari komputer server' : ''}.
      {#if info.notes}<span class="text-amber-800">{info.notes}</span>{/if}
      {#if error}<span class="ml-2 text-red-700">{error}</span>{/if}
    </span>
    <button class="btn btn-primary px-3 py-1" onclick={install} disabled={updateState.installing}>{updateState.installing ? 'Memasang…' : 'Update Sekarang'}</button>
    <button class="btn btn-ghost px-3 py-1" onclick={() => (updateState.dismissed = true)} disabled={updateState.installing}>Nanti</button>
  </div>
{/if}
