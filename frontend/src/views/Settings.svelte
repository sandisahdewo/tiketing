<script lang="ts">
  import { App, model, handleError, errorMessage } from '../lib/api'
  import { session, isAdmin } from '../lib/session.svelte'
  import { updateState } from '../lib/update.svelte'
  import { toast } from '../lib/toast.svelte'
  import Modal from '../components/Modal.svelte'
  import Field from '../components/Field.svelte'
  import Select from '../components/Select.svelte'
  import { FormState, required, maxLength } from '../lib/form.svelte'
  import { today } from '../lib/date'

  let { onreset }: { onreset: () => void } = $props()

  const info = session.info!
  let terminal = $state(info.terminal)
  let invoiceCode = $state(info.invoiceCode)
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

  const localForm = new FormState(() => ({
    terminal: required(terminal, 'Nama loket wajib diisi') ?? maxLength(terminal, 30, 'Nama loket maksimal 30 karakter'),
    invoiceCode: /^[A-Za-z0-9]{0,4}$/.test(invoiceCode.trim()) ? undefined : 'Kode nota maksimal 4 karakter huruf/angka'
  }))
  // Printing problems belong to the printer field.
  const localFields = { terminal: /terminal|loket/i, invoiceCode: /kode nota/i, paperWidth: /kertas/i, printerName: /printer|cetak/i }

  const businessForm = new FormState(() => ({
    businessName: maxLength(business.businessName, 100, 'Nama usaha maksimal 100 karakter'),
    invoicePrefix: /^[A-Za-z0-9]{1,6}$/.test(business.invoicePrefix.trim()) ? undefined : 'Prefix nota 1-6 karakter huruf/angka, contoh PNY',
    purposeWeekday: required(business.purposeWeekday, 'Purpose hari kerja wajib diisi') ?? maxLength(business.purposeWeekday, 40, 'Purpose maksimal 40 karakter'),
    purposeWeekend: required(business.purposeWeekend, 'Purpose akhir pekan wajib diisi') ?? maxLength(business.purposeWeekend, 40, 'Purpose maksimal 40 karakter')
  }))
  let logoError = $state('')

  let invoiceExample = $derived(
    `${business.invoicePrefix.toUpperCase()}${invoiceCode.toUpperCase()}${today().replaceAll('-', '')}0001`
  )

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
      await App.SaveLocalSettings(printerName, Number(paperWidth), terminal, invoiceCode)
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
      await App.SaveLocalSettings(printerName, Number(paperWidth), terminal, invoiceCode)
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

  async function checkUpdate() {
    if (checking) return
    checking = true
    updateMessage = ''
    try {
      const u = await App.CheckUpdate()
      updateState.info = u
      updateState.dismissed = false
      if (!u.configured) updateMessage = 'Update otomatis belum dikonfigurasi pada build ini.'
      else if (u.available) updateMessage = `Versi ${u.latest} tersedia. Lihat banner di atas.`
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

  <div class="mt-4 grid max-w-5xl grid-cols-1 gap-6 xl:grid-cols-2">
    <section class="card p-5">
      <h2 class="font-semibold">Komputer ini</h2>
      <p class="text-sm text-slate-500">Hanya berlaku untuk komputer ini.</p>
      <div class="mt-4 space-y-4">
        <Field label="Nama loket" id="terminal" form={localForm} name="terminal">
          <input id="terminal" class="input" bind:value={terminal} maxlength="30" />
        </Field>
        {#snippet invoiceHint()}
          Disisipkan setelah prefix. Contoh no. nota: <span class="font-mono font-semibold">{invoiceExample}</span>
        {/snippet}
        <Field label="Kode nota" id="code" form={localForm} name="invoiceCode" hint={invoiceHint}>
          <input id="code" class="input w-32 uppercase" bind:value={invoiceCode} maxlength="4" placeholder="25" />
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
        {#if localForm.general}<p class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{localForm.general}</p>{/if}
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
          {#if businessForm.general}<p class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">{businessForm.general}</p>{/if}
          <button class="btn btn-primary" disabled={savingBusiness}>{savingBusiness ? 'Menyimpan…' : 'Simpan'}</button>
        </form>
      </section>
    {/if}

    <section class="card p-5">
      <h2 class="font-semibold">Update aplikasi</h2>
      <p class="mt-2 text-sm">Versi terpasang: <b>{info.version}</b></p>
      <button class="btn btn-secondary mt-4" onclick={checkUpdate} disabled={checking}>{checking ? 'Memeriksa…' : 'Cek Update'}</button>
      {#if updateMessage}<p class="mt-3 text-sm text-slate-600">{updateMessage}</p>{/if}
      {#if info.mode === 'server'}
        <p class="mt-3 text-xs text-slate-500">Update komputer server lebih dulu, baru komputer client.</p>
      {/if}
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
