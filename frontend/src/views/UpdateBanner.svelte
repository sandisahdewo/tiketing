<script lang="ts">
  import Modal from '../components/Modal.svelte'
  import { App, errorMessage } from '../lib/api'
  import { updateState } from '../lib/update.svelte'

  let error = $state('')

  async function install() {
    error = ''
    updateState.installing = true
    try {
      await App.ApplyUpdate()
    } catch (e) {
      error = errorMessage(e)
      updateState.installing = false
    }
  }
</script>

{#if updateState.info?.available && updateState.info.mandatory}
  <Modal title="Update Wajib" closable={false} width="max-w-md">
    <p>Versi <b>{updateState.info.latest}</b> wajib dipasang sebelum aplikasi bisa dipakai lagi.</p>
    {#if updateState.info.notes}<p class="mt-3 text-sm whitespace-pre-line text-slate-600">{updateState.info.notes}</p>{/if}
    {#if error}<p class="mt-3 rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>{/if}
    {#snippet footer()}
      <button class="btn btn-primary" onclick={install} disabled={updateState.installing}>{updateState.installing ? 'Mengunduh & memasang…' : 'Update Sekarang'}</button>
    {/snippet}
  </Modal>
{:else if updateState.info?.available && !updateState.dismissed}
  <div class="flex items-center gap-4 bg-amber-100 px-4 py-2 text-sm text-amber-900">
    <span class="flex-1">
      <b>Versi {updateState.info.latest} tersedia</b> (sekarang {updateState.info.current}).
      {#if updateState.info.notes}<span class="text-amber-800">{updateState.info.notes}</span>{/if}
      {#if error}<span class="ml-2 text-red-700">{error}</span>{/if}
    </span>
    <button class="btn btn-primary px-3 py-1" onclick={install} disabled={updateState.installing}>{updateState.installing ? 'Memasang…' : 'Update Sekarang'}</button>
    <button class="btn btn-ghost px-3 py-1" onclick={() => (updateState.dismissed = true)} disabled={updateState.installing}>Nanti</button>
  </div>
{/if}
