<script lang="ts">
  // Tells the user that the server could not be reached, whichever request
  // found out (see lib/connection). It stays until closed or connected again.
  import Modal from './Modal.svelte'
  import { App } from '../lib/api'
  import { connection } from '../lib/connection.svelte'
  import { toast } from '../lib/toast.svelte'

  let checking = $state(false)

  $effect(() => {
    connection.watching = true
    return () => {
      connection.watching = false
      connection.error = ''
    }
  })

  async function retry() {
    if (checking) return
    checking = true
    try {
      await App.Status()
      connection.error = ''
      toast('Terhubung kembali ke server')
    } catch {
      // Still not reachable: the alert stays.
    } finally {
      checking = false
    }
  }
</script>

{#if connection.error}
  <Modal title="Server tidak terhubung" closable={false} width="max-w-md">
    <p role="alert">{connection.error}</p>
    <p class="mt-2 text-sm text-slate-600">
      Yang baru saja Anda lakukan mungkin belum tersimpan. Setelah terhubung kembali, ulangi langkah itu. Untuk pembayaran, aplikasi
      memberi tahu kalau transaksinya ternyata sudah tercatat, supaya nota tidak dobel.
    </p>
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (connection.error = '')}>Tutup</button>
      <button class="btn btn-primary" onclick={retry} disabled={checking}>{checking ? 'Mencoba…' : 'Coba lagi'}</button>
    {/snippet}
  </Modal>
{/if}
