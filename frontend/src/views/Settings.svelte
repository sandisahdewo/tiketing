<script lang="ts">
  import { App, model, handleError, errorMessage } from '../lib/api'
  import type { main } from '../../wailsjs/go/models'
  import { session, isAdmin } from '../lib/session.svelte'
  import { updateState, checkUpdate } from '../lib/update.svelte'
  import { toast } from '../lib/toast.svelte'
  import Modal from '../components/Modal.svelte'
  import Field from '../components/Field.svelte'
  import FormAlert from '../components/FormAlert.svelte'
  import Select from '../components/Select.svelte'
  import { FormState, required, maxLength } from '../lib/form.svelte'
  import { formatDateTime } from '../lib/date'

  let { onreset }: { onreset: () => void } = $props()

  const info = session.info!
  let printerName = $state(info.printerName)
  let paperWidth = $state(info.paperWidth || 58)
  let printers = $state<string[]>([])
  let savingLocal = $state(false)

  let business = $state<model.Settings>(
    model.Settings.createFrom({ businessName: '', address: '', footer: '', invoicePrefix: '', purposeWeekday: '', purposeWeekend: '', hasLogo: false })
  )
  let savingBusiness = $state(false)
  let logo = $state('')
  let logoBusy = $state(false)

  const localForm = new FormState(() => ({}))
  // Printing problems belong to the printer field.
  // "Printer belum dipilih" is about the field; a print that fails is not.
  const localFields = { paperWidth: /kertas/i, printerName: /printer belum dipilih/i }

  // This PC's counter. Only an admin picks it, from the counters in the Loket menu.
  let myCounter = $state<model.Counter | null>(null)
  let counters = $state<model.Counter[]>([])
  let chosenCounter = $state(0)
  let counterError = $state('')
  let assigning = $state(false)
  let takeOver = $state<model.Counter | null>(null)

  async function loadCounter() {
    try {
      const c = await App.MyCounter()
      myCounter = c.id ? c : null
      chosenCounter = c.id
      session.info = await App.GetAppInfo()
      if (isAdmin()) counters = await App.ListCounters()
    } catch (e) {
      counterError = errorMessage(e)
    }
  }

  $effect(() => {
    loadCounter()
  })

  let counterOptions = $derived(
    counters
      .filter((c) => c.active)
      .map((c) => ({
        value: c.id,
        label: c.name + (c.deviceId && c.deviceId !== info.deviceId ? ` (dipakai ${c.deviceName || 'komputer lain'})` : '')
      }))
  )

  async function useCounter(force = false) {
    if (assigning || !chosenCounter) return
    const c = counters.find((x) => x.id === chosenCounter)
    if (!force && c?.deviceId && c.deviceId !== info.deviceId) {
      takeOver = c
      return
    }
    assigning = true
    counterError = ''
    try {
      await App.UseCounter(chosenCounter, force)
      takeOver = null
      await loadCounter()
      toast('Loket komputer ini: ' + (myCounter?.name ?? ''))
    } catch (e) {
      counterError = errorMessage(e)
      takeOver = null
    } finally {
      assigning = false
    }
  }

  const businessForm = new FormState(() => ({
    businessName: maxLength(business.businessName, 100, 'Nama usaha maksimal 100 karakter'),
    invoicePrefix: /^[A-Za-z0-9]{1,6}$/.test(business.invoicePrefix.trim()) ? undefined : 'Prefix nota 1-6 karakter huruf/angka, contoh PNY',
    purposeWeekday: required(business.purposeWeekday, 'Purpose hari kerja wajib diisi') ?? maxLength(business.purposeWeekday, 40, 'Purpose maksimal 40 karakter'),
    purposeWeekend: required(business.purposeWeekend, 'Purpose akhir pekan wajib diisi') ?? maxLength(business.purposeWeekend, 40, 'Purpose maksimal 40 karakter')
  }))
  let logoError = $state('')


  async function loadLogo() {
    try {
      logo = await App.LogoPreview()
    } catch (e) {
      handleError(e)
    }
  }

  async function chooseLogo() {
    if (logoBusy) return
    logoBusy = true
    logoError = ''
    try {
      if (await App.ChooseLogo()) {
        business.hasLogo = true
        await loadLogo()
        toast('Logo disimpan')
      }
    } catch (e) {
      logoError = errorMessage(e)
    } finally {
      logoBusy = false
    }
  }

  async function removeLogo() {
    if (logoBusy) return
    logoBusy = true
    logoError = ''
    try {
      await App.RemoveLogo()
      business.hasLogo = false
      logo = ''
      toast('Logo dihapus')
    } catch (e) {
      logoError = errorMessage(e)
    } finally {
      logoBusy = false
    }
  }

  let backingUp = $state(false)
  let backupMessage = $state('')
  let backupError = $state('')

  async function backupDatabase() {
    if (backingUp) return
    backingUp = true
    backupMessage = backupError = ''
    try {
      const path = await App.BackupDatabase()
      if (path) {
        backupMessage = 'Backup disimpan: ' + path
        toast('Backup database disimpan')
      }
    } catch (e) {
      backupError = errorMessage(e)
    } finally {
      backingUp = false
    }
  }

  // Restore: pick and check a backup, confirm, then replace the database.
  let restoreInfo = $state<main.RestoreInfo | null>(null)
  let checkingBackup = $state(false)
  let restoring = $state(false)

  async function chooseRestore() {
    if (backingUp || checkingBackup || restoring) return
    backupMessage = backupError = ''
    checkingBackup = true
    try {
      const info = await App.ChooseRestoreFile()
      if (info.path) restoreInfo = info
    } catch (e) {
      backupError = errorMessage(e)
    } finally {
      checkingBackup = false
    }
  }

  function cancelRestore() {
    if (restoring) return
    restoreInfo = null
    App.CancelRestore()
  }

  async function restoreDatabase() {
    if (!restoreInfo || restoring) return
    restoring = true
    try {
      await App.RestoreDatabase()
      toast('Database dipulihkan. Silakan login kembali.')
      restoreInfo = null
      onreset()
    } catch (e) {
      backupError = errorMessage(e)
      restoreInfo = null
    } finally {
      restoring = false
    }
  }

  let savingLogs = $state(false)
  let logsMessage = $state('')
  let logsError = $state('')

  async function downloadLogs() {
    if (savingLogs) return
    savingLogs = true
    logsMessage = logsError = ''
    try {
      const path = await App.DownloadLogs()
      if (path) {
        logsMessage = 'Log disimpan: ' + path
        toast('Log aplikasi disimpan')
      }
    } catch (e) {
      logsError = errorMessage(e)
    } finally {
      savingLogs = false
    }
  }

  let checking = $state(false)
  let updateMessage = $state('')
  let confirmReset = $state(false)

  $effect(() => {
    App.ListPrinters()
      .then((p) => (printers = p))
      .catch(handleError)
    App.GetSettings()
      .then((s) => {
        business = s
        if (s.hasLogo) loadLogo()
      })
      .catch(handleError)
  })

  // Saving and test printing both write the local settings, so they share
  // savingLocal and each blocks the other.
  async function saveLocal() {
    if (savingLocal || !localForm.check()) return
    savingLocal = true
    try {
      await App.SaveLocalSettings(printerName, Number(paperWidth))
      session.info = await App.GetAppInfo()
      toast('Pengaturan komputer disimpan')
    } catch (e) {
      localForm.fail(errorMessage(e), localFields)
    } finally {
      savingLocal = false
    }
  }

  let testPrinting = $state(false)

  async function testPrint() {
    if (savingLocal || !localForm.check()) return
    savingLocal = testPrinting = true
    try {
      await App.SaveLocalSettings(printerName, Number(paperWidth))
      toast(await App.PrintTest())
    } catch (e) {
      localForm.fail(errorMessage(e), localFields)
    } finally {
      savingLocal = testPrinting = false
    }
  }

  async function saveBusiness(e: SubmitEvent) {
    e.preventDefault()
    if (savingBusiness || !businessForm.check()) return
    savingBusiness = true
    try {
      await App.SaveSettings(business)
      toast('Info usaha disimpan')
    } catch (e) {
      businessForm.fail(errorMessage(e), { invoicePrefix: /prefix/i, purposeWeekday: /purpose/i, businessName: /nama usaha/i })
    } finally {
      savingBusiness = false
    }
  }

  async function checkNow() {
    if (checking) return
    checking = true
    updateMessage = ''
    try {
      const u = await checkUpdate()
      updateState.dismissed = false
      if (u.serverBehind) updateMessage = `Server masih versi ${u.latest}. Update komputer server dulu.`
      else if (u.manual) updateMessage = `Server sudah versi ${u.latest}, tapi filenya tidak bisa dibagikan. Pasang installer versi ${u.latest} di komputer ini.`
      else if (!u.configured) updateMessage = 'Update otomatis belum dikonfigurasi pada build ini.'
      else if (u.available) updateMessage = `Versi ${u.latest} tersedia. Lihat banner di atas.`
      else if (u.fromServer) updateMessage = 'Aplikasi sudah sama dengan versi server.'
      else updateMessage = 'Aplikasi sudah versi terbaru.'
    } catch (e) {
      updateMessage = errorMessage(e)
    } finally {
      checking = false
    }
  }

  let resetting = $state(false)

  async function resetMode() {
    if (resetting) return
    resetting = true
    try {
      await App.ResetSetup()
      onreset()
    } catch (e) {
      resetting = false
      handleError(e)
    }
  }
</script>

<div class="h-full overflow-y-auto p-6">
  <h1 class="text-2xl font-bold">Pengaturan</h1>

  <div class="mt-4 grid max-w-5xl grid-cols-1 items-start gap-6 xl:grid-cols-2">
    <section class="card p-5">
      <h2 class="font-semibold">Komputer ini</h2>
      <p class="text-sm text-slate-500">Hanya berlaku untuk komputer ini.</p>
      <div class="mt-4 space-y-4">
        <FormAlert message={localForm.general} />
        <Field label="Loket" id="counter" error={counterError}>
          {#if myCounter}
            <p class="text-sm"><b>{myCounter.name}</b> <span class="text-slate-500">· kode nota {myCounter.invoiceCode}</span></p>
          {:else}
            <p class="rounded-md bg-amber-50 px-3 py-2 text-sm text-amber-800">Komputer ini belum punya loket dan belum bisa dipakai transaksi.{isAdmin() ? '' : ' Minta admin memilih loket.'}</p>
          {/if}
          {#if isAdmin()}
            <div class="mt-2 flex gap-2">
              <div class="flex-1"><Select id="counter" bind:value={chosenCounter} options={[{ value: 0, label: 'Pilih loket…' }, ...counterOptions]} /></div>
              <button class="btn btn-secondary" onclick={() => useCounter()} disabled={assigning || !chosenCounter || chosenCounter === myCounter?.id}>{assigning ? 'Menyimpan…' : 'Pakai'}</button>
            </div>
            <p class="field-hint">Buat atau ubah loket di menu Loket. Satu loket hanya untuk satu komputer.</p>
          {/if}
        </Field>
        <Field label="Printer thermal" id="printer" form={localForm} name="printerName">
          <Select
            id="printer"
            bind:value={printerName}
            options={[...(printerName && !printers.includes(printerName) ? [printerName] : []), ...printers].map((p) => ({ value: p, label: p }))}
          />
        </Field>
        <Field label="Lebar kertas" form={localForm} name="paperWidth">
          <div class="flex gap-4 text-sm">
            <label class="flex items-center gap-2"><input type="radio" bind:group={paperWidth} value={58} /> 58 mm</label>
            <label class="flex items-center gap-2"><input type="radio" bind:group={paperWidth} value={80} /> 80 mm</label>
          </div>
        </Field>
        <div class="flex gap-2">
          <button class="btn btn-primary" onclick={saveLocal} disabled={savingLocal}>{savingLocal && !testPrinting ? 'Menyimpan…' : 'Simpan'}</button>
          <button class="btn btn-secondary" onclick={testPrint} disabled={savingLocal}>{testPrinting ? 'Mencetak…' : 'Cetak Tes'}</button>
        </div>
      </div>
    </section>

    <section class="card p-5">
      <h2 class="font-semibold">Koneksi</h2>
      <dl class="mt-4 space-y-2 text-sm">
        <div class="flex gap-2"><dt class="w-32 text-slate-500">Mode</dt><dd class="font-medium">{info.mode === 'server' ? 'Server' : 'Client'}</dd></div>
        {#if info.mode === 'server'}
          <div class="flex gap-2">
            <dt class="w-32 text-slate-500">Alamat untuk client</dt>
            <dd class="font-mono">
              {#each info.addresses as ip}<div>{ip}:{info.port}</div>{:else}<div>Tidak ada jaringan</div>{/each}
            </dd>
          </div>
          <div class="flex gap-2"><dt class="w-32 text-slate-500">Folder data</dt><dd class="font-mono break-all">{info.dataDir}</dd></div>
        {:else}
          <div class="flex gap-2"><dt class="w-32 text-slate-500">Server</dt><dd class="font-mono">{info.serverUrl}</dd></div>
        {/if}
      </dl>
      {#if isAdmin()}
        <button class="btn btn-secondary mt-4" onclick={() => (confirmReset = true)}>Ubah mode server/client</button>
      {/if}
    </section>

    {#if isAdmin()}
      <section class="card p-5">
        <h2 class="font-semibold">Info usaha di nota</h2>
        <p class="text-sm text-slate-500">Berlaku untuk semua komputer.</p>
        <form class="mt-4 space-y-4" onsubmit={saveBusiness} novalidate>
          <FormAlert message={businessForm.general} />
          <Field label="Nama usaha" id="bname" form={businessForm} name="businessName">
            <input id="bname" class="input" bind:value={business.businessName} maxlength="100" />
          </Field>
          <Field
            label="Logo nota"
            error={logoError}
            hint="PNG/JPG maks. 1 MB. Logo dicetak hitam-putih menggantikan nama usaha. Gambar bergaris tegas di latar putih/transparan hasilnya paling bagus."
          >
            <div class="flex items-center gap-4">
              <div class="flex h-24 w-40 items-center justify-center rounded-md border border-dashed border-slate-300 bg-slate-50">
                {#if logo}
                  <img src={logo} alt="Logo" class="max-h-20 max-w-36 [image-rendering:pixelated]" />
                {:else}
                  <span class="text-xs text-slate-400">Tanpa logo</span>
                {/if}
              </div>
              <div class="flex flex-col gap-2">
                <button type="button" class="btn btn-secondary" onclick={chooseLogo} disabled={logoBusy}>Pilih Gambar…</button>
                {#if business.hasLogo}
                  <button type="button" class="btn px-3 text-red-600 hover:bg-red-50" onclick={removeLogo} disabled={logoBusy}>Hapus Logo</button>
                {/if}
              </div>
            </div>
          </Field>
          <Field label="Alamat / telepon" id="addr" form={businessForm} name="address">
            <textarea id="addr" class="input" rows="3" bind:value={business.address}></textarea>
          </Field>
          <Field label="Catatan kaki nota" id="footer" form={businessForm} name="footer">
            <textarea id="footer" class="input" rows="2" bind:value={business.footer} placeholder="Opsional"></textarea>
          </Field>
          <Field label="Prefix no. nota" id="prefix" form={businessForm} name="invoicePrefix" hint="1-6 huruf/angka">
            <input id="prefix" class="input w-32 uppercase" bind:value={business.invoicePrefix} maxlength="6" placeholder="PNY" />
          </Field>
          <div class="grid grid-cols-2 gap-3">
            <Field label="Purpose hari kerja" id="pwd" form={businessForm} name="purposeWeekday" hint="Senin–Jumat">
              <input id="pwd" class="input" bind:value={business.purposeWeekday} maxlength="40" />
            </Field>
            <Field label="Purpose akhir pekan" id="pwe" form={businessForm} name="purposeWeekend" hint="Sabtu–Minggu">
              <input id="pwe" class="input" bind:value={business.purposeWeekend} maxlength="40" />
            </Field>
          </div>
          <button class="btn btn-primary" disabled={savingBusiness}>{savingBusiness ? 'Menyimpan…' : 'Simpan'}</button>
        </form>
      </section>
    {/if}

    {#if isAdmin()}
      <section class="card p-5">
        <h2 class="font-semibold">Backup database</h2>
        {#if info.mode === 'server'}
          <ul class="mt-2 space-y-1.5 text-sm text-slate-600">
            <li>
              <b class="text-slate-700">Backup</b> menyimpan salinan lengkap semua data (transaksi, tiket, pengguna, pengaturan) ke satu file .db.
              Simpan di flashdisk atau folder Google Drive/OneDrive, jangan hanya di komputer ini. Backup harian otomatis tetap dibuat di folder data.
            </li>
            <li>
              <b class="text-slate-700">Restore</b> mengganti semua data dengan isi file backup. Transaksi setelah waktu backup akan hilang. Backup diperiksa dan diuji
              dulu sebelum dipakai, dan data sekarang otomatis dibackup sebelum diganti.
            </li>
          </ul>
          <div class="mt-4 flex gap-2">
            <button class="btn btn-secondary" onclick={backupDatabase} disabled={backingUp || checkingBackup || restoring}>{backingUp ? 'Membuat backup…' : 'Backup Database'}</button>
            <button class="btn btn-secondary" onclick={chooseRestore} disabled={backingUp || checkingBackup || restoring}>{checkingBackup ? 'Memeriksa backup…' : 'Restore Database…'}</button>
          </div>
          {#if backupMessage}<p class="mt-3 text-sm break-all text-emerald-700">{backupMessage}</p>{/if}
          {#if backupError}<p class="mt-3 rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{backupError}</p>{/if}
        {:else}
          <p class="mt-2 text-sm text-slate-600">Database ada di komputer server ({info.serverUrl}). Backup dan restore dilakukan dari sana.</p>
        {/if}
      </section>
    {/if}

    <section class="card p-5">
      <h2 class="font-semibold">Update aplikasi</h2>
      <p class="mt-2 text-sm">Versi terpasang: <b>{info.version}</b></p>
      <button class="btn btn-secondary mt-4" onclick={checkNow} disabled={checking}>{checking ? 'Memeriksa…' : 'Cek Update'}</button>
      {#if updateMessage}<p class="mt-3 text-sm text-slate-600">{updateMessage}</p>{/if}
      <p class="mt-3 text-xs text-slate-500">
        {#if info.mode === 'server'}
          Komputer server mengambil update dari internet. Setelah server diupdate, komputer client mendapat versi yang sama dari server, tanpa internet.
        {:else}
          Komputer ini mengikuti versi komputer server. Update komputer server lebih dulu.
        {/if}
      </p>
    </section>

    <section class="card p-5">
      <h2 class="font-semibold">Log aplikasi</h2>
      <p class="mt-2 text-sm text-slate-600">
        Simpan catatan aktivitas dan error komputer ini (30 hari terakhir) sebagai satu file .zip untuk dikirim ke pengembang saat ada masalah.
        {#if info.mode === 'client'}Kalau masalahnya terkait server, ambil juga log dari komputer server.{/if}
      </p>
      <button class="btn btn-secondary mt-4" onclick={downloadLogs} disabled={savingLogs}>{savingLogs ? 'Menyiapkan…' : 'Download Log'}</button>
      {#if logsMessage}<p class="mt-3 text-sm break-all text-emerald-700">{logsMessage}</p>{/if}
      {#if logsError}<p class="mt-3 rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{logsError}</p>{/if}
    </section>
  </div>
</div>

{#if confirmReset}
  <Modal title="Ubah Mode" onclose={() => (confirmReset = false)} width="max-w-md">
    <p>Aplikasi akan kembali ke Pengaturan Awal. Data di komputer ini <b>tidak dihapus</b>.</p>
    {#if info.mode === 'server'}<p class="mt-2 text-sm text-red-700">Komputer client tidak bisa terhubung selama server dimatikan.</p>{/if}
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (confirmReset = false)}>Batal</button>
      <button class="btn btn-danger" onclick={resetMode} disabled={resetting}>{resetting ? 'Memproses…' : 'Lanjutkan'}</button>
    {/snippet}
  </Modal>
{/if}

{#if restoreInfo}
  <Modal title="Restore Database" onclose={cancelRestore} closable={!restoring} width="max-w-md">
    <dl class="space-y-1 rounded-md bg-slate-50 p-3 text-sm">
      <div class="flex gap-2"><dt class="w-36 text-slate-500">File</dt><dd class="font-medium break-all">{restoreInfo.name}</dd></div>
      <div class="flex gap-2"><dt class="w-36 text-slate-500">Transaksi</dt><dd class="font-medium">{restoreInfo.sales.toLocaleString('id-ID')}</dd></div>
      <div class="flex gap-2"><dt class="w-36 text-slate-500">Transaksi terakhir</dt><dd class="font-medium">{formatDateTime(restoreInfo.lastSale)}</dd></div>
      <div class="flex gap-2"><dt class="w-36 text-slate-500">Pengguna</dt><dd class="font-medium">{restoreInfo.users}</dd></div>
    </dl>
    <ul class="mt-3 space-y-0.5 text-sm text-emerald-700">
      <li>✓ File backup utuh</li>
      <li>✓ Struktur data cocok dengan aplikasi ini</li>
      <li>✓ Ada akun admin aktif</li>
      <li>✓ Uji baca data oleh aplikasi berhasil</li>
    </ul>
    <p class="mt-3 text-sm">
      <b class="text-red-700">Semua data sekarang akan diganti</b> dengan isi backup ini: transaksi, tiket, pengguna, dan pengaturan. Transaksi setelah
      {formatDateTime(restoreInfo.lastSale)} akan hilang.
    </p>
    <p class="mt-2 text-sm text-slate-600">
      Data sekarang otomatis dibackup dulu ke folder backups. Semua pengguna, termasuk di komputer kasir, harus login ulang.
    </p>
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={cancelRestore} disabled={restoring}>Batal</button>
      <button class="btn btn-danger" onclick={restoreDatabase} disabled={restoring}>{restoring ? 'Memulihkan…' : 'Restore Database'}</button>
    {/snippet}
  </Modal>
{/if}

{#if takeOver}
  <Modal title="Ambil Alih Loket" onclose={() => (takeOver = null)} width="max-w-md">
    <p>
      Loket <b>{takeOver.name}</b> sedang dipakai komputer <b>{takeOver.deviceName || 'lain'}</b>. Kalau diambil alih, komputer itu tidak punya loket lagi
      dan tidak bisa bertransaksi sampai admin memilihkan loket lain.
    </p>
    {#snippet footer()}
      <button class="btn btn-secondary" onclick={() => (takeOver = null)} disabled={assigning}>Batal</button>
      <button class="btn btn-danger" onclick={() => useCounter(true)} disabled={assigning}>{assigning ? 'Menyimpan…' : 'Ambil Alih'}</button>
    {/snippet}
  </Modal>
{/if}
