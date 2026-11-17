// End-to-end tests of paying at the cashier screen when things go wrong
// between the client PC and the server: the money part of the app.
//
// A real server app, a real client app (driven through its page), and a
// proxy in between that loses a request, loses an answer, holds an answer
// back, or ends the server abruptly. After every scenario the sales on the
// server are compared with what should have been recorded: a sale must never
// be recorded twice without the cashier having decided so.
//
// Started by run.sh, which builds the app, makes the data folders and runs
// the client in dev mode. Environment:
//   E2E_DIR     folder with server/ (seeded, see seed/main.go), client/, shots/
//   E2E_BIN     the built app, run as the server
//   E2E_CHROME  a Chrome or Chromium to drive (default: Playwright's own)
//   E2E_ONLY    run only the scenarios whose name starts with this, e.g. "07"
const { chromium } = require('playwright-core')
const { spawn, execFileSync } = require('child_process')
const http = require('http')

const E = process.env.E2E_DIR
const BIN = process.env.E2E_BIN
if (!E || !BIN) {
  console.error('Jalankan lewat e2e/run.sh')
  process.exit(2)
}
const PAGE = 'http://localhost:34115/' // the client app in dev mode
const SERVER = 18790
const PROXY = 18791 // what the client is set up to connect to
const DEVICE = 'e2e-client'
const ENTRY = 1000000 // Rp 10.000
const PARKIR = 500000 // Rp 5.000
const OPENING = 50000000 // Rp 500.000
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

// ---- server ----------------------------------------------------------------
let server
const up = () => fetch(`http://127.0.0.1:${SERVER}/api/status`).then(() => true, () => false)
async function startServer() {
  server = spawn(BIN, [], { env: { ...process.env, TIKET_DATA_DIR: `${E}/server` }, detached: true, stdio: 'ignore' })
  for (let i = 0; i < 80 && !(await up()); i++) await sleep(500)
  if (!(await up())) throw new Error('server did not start')
}
async function stopServer(signal) {
  server.kill(signal)
  for (let i = 0; i < 80 && (await up()); i++) await sleep(250)
}
async function api(name, token, body) {
  const res = await fetch(`http://127.0.0.1:${SERVER}/api/${name}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-App-Version': '0.1.0', ...(token ? { Authorization: 'Bearer ' + token } : {}) },
    body: JSON.stringify(body || {})
  })
  return { status: res.status, body: await res.json() }
}
async function ok(name, token, body) {
  const r = await api(name, token, body)
  if (r.status !== 200) throw new Error(`${name}: ${r.status} ${r.body.error}`)
  return r.body
}
const loginApi = async (user) => (await ok('login', '', { username: user, password: passwords[user] })).token
const sql = (q) => execFileSync('sqlite3', [`${E}/server/tiket.db`, q]).toString().trim()
// The sales on the server that count (not deleted): how many, and their sum.
const sales = () => {
  const [n, sum] = sql('SELECT COUNT(*), COALESCE(SUM(total), 0) FROM sales WHERE deleted_at IS NULL').split('|').map(Number)
  return { n, sum }
}

// ---- proxy -----------------------------------------------------------------
// fault applies to the next sale that passes:
//   'request'  the request is lost before the server sees it
//   'answer'   the server handles the sale, its answer is lost
//   'slow'     the answer is held back longer than the client waits (15 s)
//   'kill'     the server app is ended abruptly right after it handled the
//              sale, as when it crashes; the answer is lost
let fault = ''
const proxy = http.createServer((req, res) => {
  const chunks = []
  req.on('data', (c) => chunks.push(c))
  req.on('end', () => {
    const mine = req.url === '/api/createSale' ? fault : ''
    if (mine) fault = ''
    if (mine === 'request') return req.socket.destroy()
    const upstream = http.request({ host: '127.0.0.1', port: SERVER, path: req.url, method: req.method, headers: req.headers }, (r) => {
      const body = []
      r.on('data', (c) => body.push(c))
      r.on('end', async () => {
        if (mine === 'kill') server.kill('SIGKILL')
        if (mine === 'answer' || mine === 'kill') return req.socket.destroy()
        if (mine === 'slow') await sleep(17000)
        if (res.destroyed) return
        res.writeHead(r.statusCode, r.headers)
        res.end(Buffer.concat(body))
      })
    })
    upstream.on('error', () => req.socket.destroy())
    upstream.end(Buffer.concat(chunks))
  })
})

// ---- the cashier -----------------------------------------------------------
const passwords = { budi: '1234', admin: 'rahasia' }
let browser, context, page
let payButton, alertHead, sameHead, otherHead
const tile = (name) => page.getByRole('button', { name: new RegExp('^' + name, 'i') }).first()
const logoutButton = () => page.getByRole('button', { name: 'Keluar', exact: true }).first()

async function login(user) {
  await page.locator('#u').fill(user)
  await page.locator('#p').fill(passwords[user])
  await page.getByRole('button', { name: 'Masuk' }).click()
}
async function logout() {
  await logoutButton().click()
  // A cashier with an open shift is asked first.
  const anyway = page.getByRole('button', { name: 'Keluar saja' })
  await Promise.race([page.locator('#u').waitFor(), anyway.waitFor()])
  if (await anyway.isVisible()) await anyway.click()
  await page.locator('#u').waitFor()
}
// A fresh screen for user: a new browser profile (nothing kept on "this
// PC"), an empty cart, the server running.
async function fresh(user = 'budi') {
  fault = ''
  if (!(await up())) await startServer()
  if (context) await context.close().catch(() => {})
  context = await browser.newContext({ viewport: { width: 1280, height: 900 } })
  page = await context.newPage()
  page.setDefaultTimeout(25000)
  payButton = page.getByRole('button', { name: 'Bayar (Enter)' })
  alertHead = page.getByRole('heading', { name: 'Server tidak terhubung' })
  sameHead = page.getByRole('heading', { name: 'Transaksi ini sudah tercatat' })
  otherHead = page.getByRole('heading', { name: 'Pembayaran sebelumnya sudah tercatat' })
  await page.goto(PAGE)
  await Promise.race([page.locator('#u').waitFor(), logoutButton().waitFor()])
  // The app itself remembers who is logged in; make it this scenario's user.
  if (!(await page.locator('#u').isVisible())) {
    if (new RegExp(user, 'i').test(await page.locator('header').innerText())) return
    await logout()
  }
  await login(user)
  await logoutButton().waitFor()
}
const add = (name) => tile(name).click()
async function openPay() {
  await page.getByRole('button', { name: 'Bayar (F8)' }).click()
  await page.getByRole('button', { name: 'Uang pas' }).click()
}
async function pay() {
  await openPay()
  await payButton.click()
}
// The sale went through: the receipt is shown.
async function receipt() {
  const done = page.getByRole('button', { name: 'Transaksi Baru' })
  await done.waitFor()
  await done.click()
}
async function closeAlert() {
  await alertHead.waitFor({ timeout: 40000 })
  await page.getByText('Tutup', { exact: true }).click()
  await alertHead.waitFor({ state: 'detached' })
}
// The warning that the sale is recorded already; the cashier looks at it.
async function view(head = sameHead) {
  await head.waitFor()
  await page.getByRole('button', { name: 'Lihat di riwayat' }).click()
  await head.waitFor({ state: 'detached' })
}
const noWarning = async () => {
  if ((await sameHead.count()) + (await otherHead.count())) throw new Error('a warning was shown')
}
// A first payment whose outcome the client does not learn.
async function lost(kind, ...names) {
  for (const n of names) await add(n)
  await openPay()
  fault = kind
  await payButton.click()
  await closeAlert()
}
// "Batal" of the payment dialog (the last one on screen) and of the cart.
const dialogCancel = () => page.getByRole('button', { name: 'Batal' }).last().click()
const cartCancel = () => page.getByRole('button', { name: 'Batal' }).first().click()
// The payment numbers kept on "this PC".
const kept = () => page.evaluate(() => Object.keys(localStorage).filter((k) => k.startsWith('tiket:sale-request')).length)

// ---- report ----------------------------------------------------------------
const results = []
// want: how the sales that count must have changed: n sales, sum of money.
// opts.user: who is at the screen; opts.page false for a scenario that talks
// to the server directly.
async function scenario(name, want, fn, opts = {}) {
  if (process.env.E2E_ONLY && !name.startsWith(process.env.E2E_ONLY)) return
  let before
  let note = ''
  let error = ''
  try {
    if (opts.page !== false) await fresh(opts.user)
    before = sales()
    note = (await fn()) || ''
  } catch (e) {
    error = String(e.message || e).split('\n')[0].slice(0, 170)
    if (opts.page !== false && page) {
      await page.screenshot({ path: `${E}/shots/gagal-${name.slice(0, 2)}.png` }).catch(() => {})
      note = 'layar: ' + (await page.locator('body').innerText().catch(() => '')).replace(/\n+/g, ' | ').slice(-240)
    }
  }
  const after = sales()
  const got = before ? { n: after.n - before.n, sum: after.sum - before.sum } : { n: NaN, sum: NaN }
  const pass = !error && got.n === want.n && got.sum === want.sum
  results.push(pass)
  console.log(
    `${pass ? 'LOLOS' : 'GAGAL'}  ${name}\n       nota bertambah ${got.n} (harus ${want.n}), nilai ${got.sum / 100} (harus ${want.sum / 100})` +
      (error ? '\n       error: ' + error : '') +
      (note ? '\n       ' + note : '')
  )
}
function check(name, pass, note) {
  results.push(pass)
  console.log(`${pass ? 'LOLOS' : 'GAGAL'}  ${name}\n       ${note}`)
}

;(async () => {
  await startServer()
  await new Promise((r) => proxy.listen(PROXY, '127.0.0.1', r))

  // Two tickets, one counter for the client PC, cashier budi on a shift.
  let admin = await loginApi('admin')
  const entry = await ok('saveTicket', admin, { name: 'Entry', price: ENTRY, category: 'ticket', color: 'orange' })
  const parkir = await ok('saveTicket', admin, { name: 'Parkir', price: PARKIR, category: 'ticket', color: 'blue' })
  const counter = await ok('saveCounter', admin, { name: 'LOKET 1', invoiceCode: '1' })
  await ok('assignCounter', admin, { counterId: counter.id, deviceId: DEVICE, deviceName: 'PC-KASIR', force: true })
  await ok('saveUser', admin, { username: 'budi', fullName: 'Budi', role: 'kasir', password: passwords.budi })
  let budi = await loginApi('budi')
  const st = await ok('shiftStatus', budi, { deviceId: DEVICE })
  const shift = await ok('openShift', budi, { deviceId: DEVICE, typeId: st.suggestedTypeId || st.types[0].id, openingCash: OPENING })

  browser = await chromium.launch(process.env.E2E_CHROME ? { executablePath: process.env.E2E_CHROME } : {})

  // ---- at the cashier screen ------------------------------------------------
  await scenario('01 Penjualan biasa', { n: 1, sum: ENTRY }, async () => {
    await add('ENTRY')
    await pay()
    await receipt()
    return `nomor yang masih tersimpan di PC: ${await kept()} (harus 0)`
  })

  await scenario('02 Tombol bayar ditekan lima kali sekaligus', { n: 1, sum: 2 * ENTRY }, async () => {
    await add('ENTRY')
    await add('ENTRY')
    await openPay()
    await payButton.evaluate((b) => {
      for (let i = 0; i < 5; i++) b.click()
    })
    await receipt()
  })

  await scenario('03 Jawaban server hilang, bayar lagi: peringatan; Batal, lalu Lihat di riwayat', { n: 1, sum: ENTRY }, async () => {
    await lost('answer', 'ENTRY')
    await payButton.click()
    await sameHead.waitFor()
    await dialogCancel()
    await sameHead.waitFor({ state: 'detached' })
    await payButton.click()
    await view()
    return `nomor yang masih tersimpan di PC: ${await kept()} (harus 0)`
  })

  await scenario('04 Jawaban hilang, pilih "Proses sebagai transaksi baru": dua nota, disengaja', { n: 2, sum: 2 * ENTRY }, async () => {
    await lost('answer', 'ENTRY')
    await payButton.click()
    await sameHead.waitFor()
    await page.getByRole('button', { name: 'Proses sebagai transaksi baru' }).click()
    await receipt()
  })

  await scenario('05 Permintaan tidak sampai ke server, bayar lagi: sekali, tanpa peringatan', { n: 1, sum: ENTRY }, async () => {
    await lost('request', 'ENTRY')
    await payButton.click()
    await receipt()
    await noWarning()
  })

  await scenario('06 Jawaban hilang, keranjang dikosongkan lalu diisi ulang yang sama', { n: 1, sum: ENTRY }, async () => {
    await lost('answer', 'ENTRY')
    await dialogCancel()
    await cartCancel()
    await add('ENTRY')
    await pay()
    await view()
  })

  await scenario('07 Jawaban hilang, pindah ke menu lain lalu kembali', { n: 1, sum: ENTRY }, async () => {
    await lost('answer', 'ENTRY')
    await dialogCancel()
    await page.getByRole('button', { name: 'Laporan', exact: true }).click()
    await page.getByRole('button', { name: 'Kasir', exact: true }).click()
    await add('ENTRY')
    await pay()
    await view()
  })

  await scenario('08 Jawaban hilang, logout lalu login lagi', { n: 1, sum: ENTRY }, async () => {
    await lost('answer', 'ENTRY')
    await dialogCancel()
    await logout()
    await login('budi')
    await add('ENTRY')
    await pay()
    await view()
  })

  await scenario('09 Jawaban hilang, halaman dimuat ulang', { n: 1, sum: ENTRY }, async () => {
    await lost('answer', 'ENTRY')
    await page.reload()
    await tile('ENTRY').waitFor()
    await add('ENTRY')
    await pay()
    await view()
  })

  await scenario('10 Jawaban hilang, isi keranjang diubah: peringatan "isi berbeda", Lihat di riwayat', { n: 1, sum: ENTRY }, async () => {
    await lost('answer', 'ENTRY')
    await dialogCancel()
    await add('PARKIR')
    await pay()
    await view(otherHead)
  })

  await scenario('11 Pembeli berikutnya setelah jawaban hilang: peringatan "isi berbeda", Lanjutkan (dua pembelian sah)', { n: 2, sum: ENTRY + PARKIR }, async () => {
    await lost('answer', 'ENTRY')
    await dialogCancel()
    await cartCancel()
    await add('PARKIR')
    await pay()
    await otherHead.waitFor()
    await page.getByRole('button', { name: 'Lanjutkan transaksi ini' }).click()
    await receipt()
  })

  await scenario('12 Server menjawab setelah 17 detik (client menunggu 15), bayar lagi', { n: 1, sum: ENTRY }, async () => {
    await lost('slow', 'ENTRY')
    await payButton.click()
    await view()
  })

  await scenario('13 Jawaban hilang, aplikasi server ditutup dan dibuka lagi, bayar lagi', { n: 1, sum: ENTRY }, async () => {
    await lost('answer', 'ENTRY')
    await stopServer()
    await startServer()
    await payButton.click()
    await view()
    return 'kasir tidak diminta login ulang'
  })

  await scenario('14 Jawaban hilang, sesi berakhir, login lagi, keranjang diisi ulang', { n: 1, sum: ENTRY }, async () => {
    await lost('answer', 'ENTRY')
    sql('DELETE FROM sessions')
    await payButton.click()
    await page.locator('#u').waitFor()
    await login('budi')
    await add('ENTRY')
    await pay()
    await view()
  })
  // The scenario above ended every login, also the ones of this script.
  admin = await loginApi('admin')
  budi = await loginApi('budi')

  await scenario('15 Server menolak (tiket nonaktif), tiket diaktifkan, bayar lagi: sekali, tanpa peringatan', { n: 1, sum: PARKIR }, async () => {
    await add('PARKIR')
    await openPay()
    await ok('setTicketActive', admin, { id: parkir.id, active: false })
    await payButton.click()
    await page.getByText(/non-aktif/).first().waitFor()
    const refused = sales()
    await ok('setTicketActive', admin, { id: parkir.id, active: true })
    await page.reload()
    await tile('PARKIR').waitFor()
    await add('PARKIR')
    await pay()
    await receipt()
    await noWarning()
    return `nomor yang masih tersimpan di PC: ${await kept()} (harus 0); saat ditolak tidak ada nota: ${refused.n}`
  })

  await scenario('16 Setelah "Proses sebagai transaksi baru", jawaban hilang lagi, bayar lagi', { n: 2, sum: 2 * ENTRY }, async () => {
    await lost('answer', 'ENTRY')
    await payButton.click()
    await sameHead.waitFor()
    fault = 'answer'
    await page.getByRole('button', { name: 'Proses sebagai transaksi baru' }).click()
    await closeAlert()
    await payButton.click()
    await view()
  })

  await scenario('17 Permintaan hilang, lalu jawaban hilang, lalu bayar', { n: 1, sum: ENTRY }, async () => {
    await lost('request', 'ENTRY')
    fault = 'answer'
    await payButton.click()
    await closeAlert()
    await payButton.click()
    await view()
  })

  await scenario('18 Permintaan tidak sampai, keranjang dikosongkan dan diisi ulang: sekali, tanpa peringatan', { n: 1, sum: ENTRY }, async () => {
    await lost('request', 'ENTRY')
    await dialogCancel()
    await cartCancel()
    await add('ENTRY')
    await pay()
    await receipt()
    await noWarning()
  })

  await scenario('19 Jawaban hilang, bayar lagi dengan uang diterima yang berbeda', { n: 1, sum: ENTRY }, async () => {
    await lost('answer', 'ENTRY')
    await page.locator('#paid').fill('50000')
    await payButton.click()
    await view()
    return `uang diterima di nota: ${Number(sql('SELECT paid FROM sales ORDER BY id DESC LIMIT 1')) / 100} (pembayaran pertama: 10000)`
  })

  await scenario('20 Server menolak, kasir batal, pembeli berikutnya membeli tiket lain: sekali, tanpa peringatan', { n: 1, sum: ENTRY }, async () => {
    await add('PARKIR')
    await openPay()
    await ok('setTicketActive', admin, { id: parkir.id, active: false })
    await payButton.click()
    await page.getByText(/non-aktif/).first().waitFor()
    await ok('setTicketActive', admin, { id: parkir.id, active: true })
    if (await payButton.isVisible()) await dialogCancel()
    await cartCancel()
    await add('ENTRY')
    await pay()
    await receipt()
    await noWarning()
  })

  await scenario('21 Aplikasi server mati mendadak tepat setelah menyimpan, dinyalakan lagi, bayar lagi', { n: 1, sum: ENTRY }, async () => {
    await lost('kill', 'ENTRY')
    for (let i = 0; i < 40 && (await up()); i++) await sleep(250)
    if (await up()) throw new Error('server was not stopped')
    await startServer()
    await payButton.click()
    await view()
    return 'nota yang tersimpan sebelum server mati tetap ada'
  })

  await scenario('22 Kasir lain login di PC yang sama setelah jawaban hilang, lalu kasir pertama kembali', { n: 1, sum: ENTRY }, async () => {
    await lost('answer', 'ENTRY')
    await dialogCancel()
    await logout()
    await login('admin')
    await logoutButton().waitFor()
    const during = await kept()
    await logout()
    await login('budi')
    await add('ENTRY')
    await pay()
    await view()
    return `nomor tersimpan selama kasir lain login: ${during} (milik kasir pertama saja)`
  })

  // The admin corrects a sale and the answer is lost: the old sale must be
  // deleted once and replaced once. Paying again is not asked about; a
  // correction cannot be made twice.
  await scenario(
    '23 Koreksi nota oleh admin, jawaban hilang, bayar lagi',
    { n: 0, sum: PARKIR },
    async () => {
      const [id, invoice] = sql("SELECT id || '|' || invoice_no FROM sales WHERE deleted_at IS NULL ORDER BY id DESC LIMIT 1").split('|')
      await page.getByRole('button', { name: 'Laporan', exact: true }).click()
      await page.getByRole('button', { name: 'Daftar transaksi' }).click()
      await page.getByRole('button', { name: 'Koreksi', exact: true }).first().click()
      await page.locator('#correct-reason').fill('tes e2e')
      await page.getByRole('button', { name: 'Koreksi di Kasir' }).click()
      await tile('PARKIR').waitFor()
      await lost('answer', 'PARKIR')
      await payButton.click()
      await page.getByText(/diganti/).first().waitFor()
      await noWarning()
      const replacements = sql(`SELECT COUNT(*) FROM sales WHERE replaces_sale_id = ${id}`)
      const deleted = sql(`SELECT deleted_at IS NOT NULL FROM sales WHERE id = ${id}`)
      if (replacements !== '1' || deleted !== '1') throw new Error(`pengganti ${replacements}, nota lama dihapus ${deleted}`)
      return `nota ${invoice} dihapus sekali dan diganti tepat satu nota`
    },
    { user: 'admin' }
  )

  // ---- straight to the server ------------------------------------------------
  const sale = (request) => api('createSale', budi, { deviceId: DEVICE, items: [{ ticketId: entry.id, qty: 1 }], paid: 5000000, requestId: request })
  const direct = { page: false }

  await scenario(
    '24 Nomor yang sama dikirim 20 kali bersamaan',
    { n: 1, sum: ENTRY },
    async () => {
      const all = await Promise.all(Array.from({ length: 20 }, () => sale('bersamaan')))
      const failed = all.filter((r) => r.status !== 200).length
      const made = all.filter((r) => r.status === 200 && !r.body.repeated).length
      const ids = new Set(all.filter((r) => r.status === 200).map((r) => r.body.id)).size
      if (failed || made !== 1 || ids !== 1) throw new Error(`gagal ${failed}, nota baru ${made}, nota berbeda ${ids}`)
      return '20 jawaban: 1 nota baru, 19 "sudah tercatat", tanpa error'
    },
    direct
  )

  await scenario(
    '25 20 penjualan berbeda dikirim bersamaan',
    { n: 20, sum: 20 * ENTRY },
    async () => {
      const all = await Promise.all(Array.from({ length: 20 }, (_, i) => sale('paralel-' + i)))
      const invoices = new Set(all.map((r) => r.body.invoiceNo)).size
      if (all.some((r) => r.status !== 200 || r.body.repeated) || invoices !== 20) throw new Error(`nomor nota berbeda: ${invoices}`)
      return '20 nomor nota berbeda'
    },
    direct
  )

  await scenario(
    '26 Dua penjualan tanpa nomor: dua nota, tidak dibandingkan',
    { n: 2, sum: 2 * ENTRY },
    async () => {
      const a = await sale('')
      const b = await sale('')
      if (a.body.repeated || b.body.repeated || a.body.id === b.body.id) throw new Error('dianggap satu penjualan')
    },
    direct
  )

  await scenario(
    '27 Nomor terlalu panjang ditolak',
    { n: 0, sum: 0 },
    async () => {
      const r = await sale('x'.repeat(65))
      if (r.status !== 400) throw new Error('status ' + r.status)
      return r.body.error
    },
    direct
  )

  await scenario(
    '28 Nomor yang sama setelah tiketnya dinonaktifkan: dilaporkan sudah tercatat, bukan error',
    { n: 1, sum: ENTRY },
    async () => {
      const first = await sale('setelah-nonaktif')
      await ok('setTicketActive', admin, { id: entry.id, active: false })
      const again = await sale('setelah-nonaktif')
      const other = await sale('baru-saat-nonaktif')
      await ok('setTicketActive', admin, { id: entry.id, active: true })
      if (again.status !== 200 || !again.body.repeated || again.body.id !== first.body.id) throw new Error(`kiriman ulang: ${again.status} ${again.body.error || ''}`)
      if (other.status !== 400) throw new Error('penjualan baru tiket nonaktif diterima')
      return 'penjualan baru tiket nonaktif tetap ditolak: ' + other.body.error
    },
    direct
  )

  await scenario(
    '29 Nomor yang sama setelah notanya dihapus admin: tidak dibuat ulang',
    { n: 0, sum: 0 },
    async () => {
      const first = await sale('setelah-dihapus')
      await ok('deleteSale', admin, { id: first.body.id, reason: 'tes e2e' })
      const again = await sale('setelah-dihapus')
      if (again.status !== 200 || !again.body.repeated || !again.body.deletedAt || again.body.id !== first.body.id) throw new Error(`${again.status} ${again.body.error || ''}`)
      return 'dilaporkan sudah tercatat dan sudah dihapus'
    },
    direct
  )

  // ---- the money -------------------------------------------------------------
  if (!process.env.E2E_ONLY) {
    const report = await ok('getShift', budi, { id: shift.id })
    const inShift = Number(sql(`SELECT COALESCE(SUM(total), 0) FROM sales WHERE deleted_at IS NULL AND shift_id = ${shift.id}`))
    check(
      '30 Uang yang seharusnya ada di laci menurut shift = modal awal + semua nota',
      report.expectedCash === OPENING + inShift,
      `shift: ${report.expectedCash / 100}; modal ${OPENING / 100} + nota ${inShift / 100}`
    )
    const twice = sql('SELECT COUNT(*) FROM (SELECT 1 FROM sales WHERE request_id IS NOT NULL GROUP BY request_id HAVING COUNT(*) > 1)')
    check('31 Tidak ada nomor transaksi yang terpakai dua kali di database', twice === '0', `nomor yang terpakai dua kali: ${twice}`)
  }

  await scenario(
    '32 Nomor yang sama setelah shift ditutup: dilaporkan sudah tercatat; penjualan baru tetap ditolak',
    { n: 1, sum: ENTRY },
    async () => {
      const first = await sale('setelah-tutup-shift')
      const r = await ok('getShift', budi, { id: shift.id })
      await ok('closeShift', budi, { shiftId: shift.id, actualCash: r.expectedCash })
      const again = await sale('setelah-tutup-shift')
      if (again.status !== 200 || !again.body.repeated || again.body.id !== first.body.id) throw new Error(`kiriman ulang: ${again.status} ${again.body.error || ''}`)
      const other = await sale('baru-setelah-tutup')
      if (other.status === 200) throw new Error('penjualan baru diterima tanpa shift')
      return 'penjualan baru: ' + other.body.error
    },
    direct
  )

  const passed = results.filter(Boolean).length
  console.log(`\n${passed} dari ${results.length} lolos.`)
  if (passed !== results.length) process.exitCode = 1
})()
  .catch((e) => {
    console.error(e)
    process.exitCode = 1
  })
  .finally(async () => {
    if (browser) await browser.close().catch(() => {})
    proxy.close()
    if (server) server.kill()
  })
