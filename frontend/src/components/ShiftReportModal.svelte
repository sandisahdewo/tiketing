<script lang="ts">
  import { untrack } from 'svelte'
  import Modal from './Modal.svelte'
  import Field from './Field.svelte'
  import FormAlert from './FormAlert.svelte'
  import MoneyInput from './MoneyInput.svelte'
  import ReceiptPaper from './ReceiptPaper.svelte'
  import LoadingOverlay from './LoadingOverlay.svelte'
  import { App, model, handleError, errorMessage } from '../lib/api'
  import { FormState, maxLength } from '../lib/form.svelte'
  import { formatRp } from '../lib/money'
  import { formatDate, formatDateTime, formatTime, toISODate } from '../lib/date'
  import { session, isAdmin } from '../lib/session.svelte'
  import { shift as shiftState, shiftHours, isOverdue } from '../lib/shift.svelte'
  import { toast } from '../lib/toast.svelte'
  import type { main } from '../../wailsjs/go/models'

  // The shift report. With shiftId it shows that one shift: its sales, the
  // cash it should have, and closing or correcting it. Without, it lists the
  // shifts of date (admin, with the ones still open from other dates) or the
  // cashier's own shifts of today. onchange runs after a shift was closed or
  // corrected.
  let {
    date = '',
    shiftId = 0,
    onclose,
    onchange
  }: { date?: string; shiftId?: number; onclose: () => void; onchange?: () => void } = $props()

  const admin = isAdmin()
  // Opened on one shift (from the cashier screen): there is no list to go back to.
  const single = untrack(() => shiftId > 0)
  let day = $state<model.ShiftDay | null>(null)
  let report = $state<model.ShiftReport | null>(null)
  let selected = $state<number | null>(untrack(() => shiftId || null))
  let loading = $state(true)
  let listMode = $derived(selected === null)

  // Shifts past their schedule are marked; the mark follows the clock.
  let now = $state(new Date())
  $effect(() => {
    const timer = setInterval(() => (now = new Date()), 30000)
    return () => clearInterval(timer)
  })

  async function load() {
    loading = true
    try {
      if (selected !== null) report = await App.GetShift(selected)
      else day = await App.ShiftDay(date)
    } catch (e) {
      handleError(e)
    } finally {
      loading = false
    }
  }

  $effect(() => {
    untrack(load)
  })

  function open(id: number | null) {
    selected = id
    report = null
    preview = null
    correcting = false
    actual = null
    note = ''
    form.reset()
    load()
  }

  // --- closing and correcting
  let correcting = $state(false)
  let editing = $derived(!!report && (!report.closing || correcting))
  let actual = $state<number | null>(null)
  let actualInvalid = $state(false)
  let note = $state('')
  let saving = $state(false)
  let printing = $state(false)
  // What the shift shows: its sales now while it is being closed, else as
  // they were when it was closed.
  let shown = $derived(report && !editing && report.closing ? report.closing.sales : (report?.sales ?? null))
  let expected = $derived(report && !editing && report.closing ? report.closing.expectedCash : (report?.expectedCash ?? 0))
  let difference = $derived(editing && actual != null ? actual - expected : null)
  const form = new FormState(() => ({
    actual: actualInvalid ? 'Kas aktual tidak valid' : actual == null ? 'Kas aktual wajib diisi' : undefined,
    note:
      report?.closing && !note.trim()
        ? 'Alasan koreksi wajib diisi'
        : difference && !note.trim()
          ? 'Keterangan selisih wajib diisi'
          : maxLength(note, 200, 'Keterangan maksimal 200 karakter')
  }))
  const fields = { actual: /kas aktual/i, note: /keterangan|alasan/i }

  function startCorrection() {
    form.reset()
    actual = null
    note = ''
    correcting = true
  }

  // The server's time now, for a closing not saved yet.
  function serverNow(): string {
    const d = new Date(Date.now() + shiftState.offset)
    const pad = (n: number) => n.toString().padStart(2, '0')
    return `${toISODate(d)} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  }

  // What will print: the closing being typed, or the one that counts.
  let draft = $derived.by((): model.ShiftClosing | null => {
    if (!report) return null
    if (!editing) return report.closing ?? null
    const prev = report.closing
    const cash = actual ?? 0
    return model.ShiftClosing.createFrom({
      id: 0,
      shift: report.shift,
      revision: prev ? prev.revision + 1 : 0,
      sales: report.sales,
      expectedCash: report.expectedCash,
      actualCash: cash,
      difference: cash - report.expectedCash,
      note: note.trim(),
      closedByName: session.user?.fullName ?? '',
      closedAt: serverNow(),
      printCount: 0,
      previousActual: prev ? prev.actualCash : null,
      deleted: report.deleted
    })
  })

  let preview = $state<main.ReceiptView | null>(null)
  $effect(() => {
    const c = draft
    const summary = listMode && admin && day
    if (!c && !summary) {
      preview = null
      return
    }
    const timer = setTimeout(() => {
      const req = c ? App.ShiftClosingPreview(c) : App.ShiftDayPreview(date)
      req.then((v) => (preview = v)).catch(() => {})
    }, 150)
    return () => clearTimeout(timer)
  })

  async function print() {
    if (!report) return
    printing = true
    try {
      toast(await App.PrintShiftClosing(report.shift.id))
    } catch (e) {
      handleError(e)
    } finally {
      printing = false
    }
  }

  async function save(e: SubmitEvent) {
    e.preventDefault()
    if (!report || saving || !form.check()) return
    saving = true
    try {
      const saved = await App.CloseShift(
        model.CloseShiftInput.createFrom({
          shiftId: report.shift.id,
          revision: report.closing ? report.closing.revision + 1 : 0,
          actualCash: actual,
          note
        })
      )
      toast(saved.revision > 0 ? `Koreksi ke-${saved.revision} disimpan` : `Shift ${saved.shift.name} ditutup`)
      onchange?.()
      await print()
      await load()
      correcting = false
      actual = null
      note = ''
      form.reset()
    } catch (err) {
      form.fail(errorMessage(err), fields)
    } finally {
      saving = false
    }
  }

  async function reprint() {
    if (!report?.closing || printing) return
    await print()
    await load()
  }

  async function printSummary() {
    if (printing) return
    printing = true
    try {
      toast(await App.PrintShiftDay(date))
    } catch (e) {
      handleError(e)
    } finally {
      printing = false
    }
  }

  function differenceClass(n: number) {
    return n < 0 ? 'text-red-700' : n > 0 ? 'text-amber-700' : 'text-emerald-700'
  }

  function signed(n: number) {
    return (n > 0 ? '+' : '') + formatRp(n)
  }

  let title = $derived(
    report && !listMode
      ? `Laporan Shift ${report.shift.name} · ${formatDate(report.shift.businessDate)}`
      : admin
        ? `Laporan Shift · ${formatDate(date)}`
        : 'Laporan Shift Saya · Hari Ini'
  )
</script>

{#snippet badge(r: model.ShiftReport)}
  {#if !r.closing}
    {#if isOverdue(r.shift, now)}
      <span class="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800">Lewat jadwal</span>
    {:else}
      <span class="rounded-full bg-sky-50 px-2 py-0.5 text-xs font-medium text-sky-700">Terbuka</span>
    {/if}
  {:else}
    <span class="rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700">Ditutup</span>
    {#if r.closing.revision > 0}<span class="ml-1 rounded-full bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-800">Dikoreksi {r.closing.revision}×</span>{/if}
    {#if r.changed}<span class="ml-1 rounded-full bg-red-50 px-2 py-0.5 text-xs font-medium text-red-700">Berubah</span>{/if}
  {/if}
{/snippet}

<Modal {title} {onclose} width="max-w-7xl">
  <div class="relative flex min-h-80 gap-5">
    <div class="min-w-0 flex-1">
      {#if listMode && day}
        {#if day.unclosed.length > 0}
          <div class="mb-4 rounded-md bg-amber-50 px-3 py-2 text-sm text-amber-900">
            <b>Masih terbuka dari tanggal lain:</b>
            <ul class="mt-1 space-y-1">
              {#each day.unclosed as u (u.id)}
                <li class="flex items-center justify-between gap-3">
                  <span>Shift {u.name} {formatDate(u.businessDate)} · {u.cashierName} · {u.counterName} · dibuka {formatDateTime(u.openedAt)}</span>
                  <button class="btn btn-secondary px-3 py-1" onclick={() => open(u.id)}>Lihat</button>
                </li>
              {/each}
            </ul>
          </div>
        {/if}
        <table class="table text-sm whitespace-nowrap">
          <thead>
            <tr>
              <th>Shift</th>
              <th>Kasir</th>
              <th class="text-right">Modal awal</th>
              <th class="text-right">Penjualan</th>
              <th class="text-right">Kas aktual</th>
              <th class="text-right">Selisih</th>
              <th>Status</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {#each day.shifts as r (r.shift.id)}
              <tr class="hover:bg-slate-50">
                <td>
                  <div class="font-medium">{r.shift.name}</div>
                  <div class="text-xs text-slate-500">
                    {r.shift.closedAt ? `${formatTime(r.shift.openedAt)}–${formatTime(r.shift.closedAt)}` : `buka ${formatTime(r.shift.openedAt)}`}
                  </div>
                </td>
                <td>
                  <div>{r.shift.cashierName}</div>
                  <div class="text-xs text-slate-500">{r.shift.counterName}</div>
                </td>
                <td class="text-right">{formatRp(r.shift.openingCash)}</td>
                <td class="text-right">{formatRp(r.closing ? r.closing.sales.total : r.sales.total)}</td>
                <td class="text-right">{r.closing ? formatRp(r.closing.actualCash) : '–'}</td>
                <td class="text-right {r.closing ? differenceClass(r.closing.difference) : ''}">{r.closing ? signed(r.closing.difference) : '–'}</td>
                <td>{@render badge(r)}</td>
                <td class="text-right"><button class="btn btn-secondary px-3 py-1" onclick={() => open(r.shift.id)}>Lihat</button></td>
              </tr>
            {:else}
              <tr>
                <td colspan="8" class="py-10 text-center text-slate-500">
                  {admin ? 'Tidak ada shift pada tanggal ini.' : 'Belum ada shift Anda hari ini.'}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {:else if report && shown}
        {@const h = report.shift}
        {#if !single}
          <button class="mb-3 text-sm text-brand-700 hover:underline" onclick={() => open(null)}>← Semua shift</button>
        {/if}
        <div class="flex items-center gap-3">
          <div class="text-lg font-semibold">Shift {h.name} · {shiftHours(h)}</div>
          {@render badge(report)}
        </div>
        <p class="text-sm text-slate-500">
          {h.cashierName} · {h.counterName} · dibuka {formatDateTime(h.openedAt)} ·
          {h.closedAt ? `ditutup ${formatDateTime(h.closedAt)}` : `jadwal selesai ${formatDateTime(h.scheduledEndAt)}`}
        </p>

        <dl class="mt-3 grid grid-cols-2 gap-x-6 gap-y-1 text-sm">
          <dt class="text-slate-500">Transaksi</dt><dd class="text-right">{shown.transactions}</dd>
          <dt class="text-slate-500">Tiket ({shown.ticketQty})</dt><dd class="text-right">{formatRp(shown.ticketTotal)}</dd>
          <dt class="text-slate-500">Topup ({shown.topupQty})</dt><dd class="text-right">{formatRp(shown.topupTotal)}</dd>
          <dt class="font-semibold">Total penjualan</dt><dd class="text-right font-semibold">{formatRp(shown.total)}</dd>
          {#if shown.deletedCount > 0}
            <dt class="text-slate-500">Nota dihapus ({shown.deletedCount})</dt><dd class="text-right text-slate-500">{formatRp(shown.deletedTotal)}</dd>
          {/if}
        </dl>

        <div class="mt-3 rounded-md bg-slate-50 p-3 text-sm">
          <div class="flex justify-between"><span class="text-slate-500">Modal awal</span><span>{formatRp(h.openingCash)}</span></div>
          <div class="flex justify-between"><span class="text-slate-500">Penjualan</span><span>{formatRp(shown.total)}</span></div>
          <div class="mt-1 flex justify-between border-t border-slate-200 pt-1 font-semibold"><span>Kas seharusnya</span><span>{formatRp(expected)}</span></div>
        </div>

        {#if report.changed && report.closing}
          <p class="mt-3 rounded-md bg-amber-50 px-3 py-2 text-sm text-amber-800">
            Ada perubahan sejak shift ditutup: penjualan sekarang <b>{formatRp(report.sales.total)}</b>, saat ditutup {formatRp(report.closing.sales.total)}. Laporan tetap mencetak angka saat ditutup; buat koreksi bila perlu.
          </p>
        {/if}

        {#if report.deleted.length > 0}
          {@const counted = report.closing?.sales.deletedCount ?? report.deleted.length}
          <div class="mt-4 text-sm font-semibold">Nota dihapus ({report.deleted.length})</div>
          <table class="table mt-1 text-sm">
            <thead><tr><th>Nota</th><th class="text-right">Total</th><th>Dihapus</th><th>Alasan</th></tr></thead>
            <tbody>
              {#each report.deleted as d, i (d.id)}
                <tr>
                  <td class="font-mono whitespace-nowrap">{d.invoiceNo}</td>
                  <td class="text-right whitespace-nowrap">{formatRp(d.total)}</td>
                  <td class="whitespace-nowrap">
                    {formatTime(d.deletedAt)} oleh {d.deletedByName}
                    {#if i >= counted}<span class="ml-1 rounded-full bg-red-50 px-2 py-0.5 text-xs font-medium text-red-700">setelah ditutup</span>{/if}
                  </td>
                  <td>
                    {d.reason}
                    {#if d.replacedByInvoiceNo}<div class="text-xs text-slate-500">diganti nota <span class="font-mono">{d.replacedByInvoiceNo}</span></div>{/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}

        {#if editing}
          <form id="shift-close-form" class="mt-4 space-y-3" onsubmit={save} novalidate>
            <FormAlert message={form.general} />
            {#if report.closing}
              <p class="text-sm text-slate-600">Koreksi ke-{report.closing.revision + 1}. Kas aktual sebelumnya {formatRp(report.closing.actualCash)}.</p>
            {/if}
            <Field label="Kas aktual (uang di laci)" id="shift-actual" {form} name="actual">
              <MoneyInput id="shift-actual" bind:value={actual} bind:invalid={actualInvalid} autofocus large />
            </Field>
            {#if difference !== null}
              <p class="text-sm">Selisih: <b class={differenceClass(difference)}>{signed(difference)}</b></p>
            {/if}
            {#if report.closing || difference}
              <Field label={report.closing ? 'Alasan koreksi' : 'Keterangan selisih'} id="shift-note" {form} name="note">
                <input
                  id="shift-note"
                  class="input"
                  bind:value={note}
                  maxlength="200"
                  placeholder={report.closing ? 'contoh: salah hitung uang 50.000' : 'contoh: uang kembalian kurang'}
                />
              </Field>
            {/if}
          </form>
        {:else if report.closing}
          {@const k = report.closing}
          <div class="mt-4 rounded-md border border-slate-200 p-3 text-sm">
            <div class="flex justify-between"><span class="text-slate-500">Kas aktual</span><b>{formatRp(k.actualCash)}</b></div>
            <div class="flex justify-between"><span class="text-slate-500">Selisih</span><b class={differenceClass(k.difference)}>{signed(k.difference)}</b></div>
            {#if k.note}<div class="mt-1"><span class="text-slate-500">Keterangan:</span> {k.note}</div>{/if}
            <div class="mt-1 text-slate-500">Ditutup oleh {k.closedByName}, {formatDateTime(k.closedAt)}{k.revision > 0 ? ` · koreksi ke-${k.revision}` : ''}</div>
          </div>
        {/if}

        {#if report.history.length > 1}
          <div class="mt-4 text-sm font-semibold">Riwayat</div>
          <table class="table mt-1 text-sm whitespace-nowrap">
            <thead><tr><th>#</th><th class="text-right">Kas aktual</th><th class="text-right">Selisih</th><th>Oleh</th><th>Keterangan</th></tr></thead>
            <tbody>
              {#each report.history as c (c.id)}
                <tr class={c.id === report.closing?.id ? 'font-medium' : 'text-slate-500'}>
                  <td>{c.revision === 0 ? 'Awal' : `Koreksi ${c.revision}`}</td>
                  <td class="text-right">{formatRp(c.actualCash)}</td>
                  <td class="text-right">{signed(c.difference)}</td>
                  <td>{c.closedByName} {formatDateTime(c.closedAt)}</td>
                  <td class="whitespace-normal">{c.note || '–'}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      {/if}
    </div>

    {#if !listMode || admin}
      <!-- items-start: a stretched paper stops at the box height while its lines run on past it; min-w lets 80mm paper widen the box -->
      <div class="flex max-h-[65vh] min-w-[22rem] shrink-0 items-start justify-center overflow-y-auto bg-slate-100 p-3">
        {#if preview}
          <ReceiptPaper lines={preview.lines} logo={preview.logo} />
        {/if}
      </div>
    {/if}
    <LoadingOverlay show={loading} />
  </div>

  {#snippet footer()}
    {#if listMode}
      {#if admin}
        <button class="btn btn-primary" onclick={printSummary} disabled={printing || !day}>{printing ? 'Mencetak…' : 'Cetak Ringkasan'}</button>
      {/if}
    {:else if editing}
      {#if correcting}
        <button class="btn btn-secondary" onclick={() => (correcting = false)} disabled={saving}>Batal koreksi</button>
      {/if}
      <button class="btn btn-primary" form="shift-close-form" disabled={saving || printing}>
        {saving || printing ? 'Menyimpan…' : correcting ? 'Simpan Koreksi & Cetak' : 'Tutup Shift & Cetak'}
      </button>
    {:else if report?.closing}
      <button class="btn btn-secondary" onclick={startCorrection} disabled={printing}>Koreksi</button>
      <button class="btn btn-primary" onclick={reprint} disabled={printing}>{printing ? 'Mencetak…' : 'Cetak Ulang'}</button>
    {/if}
    <button class="btn btn-secondary" onclick={onclose}>Tutup</button>
  {/snippet}
</Modal>
