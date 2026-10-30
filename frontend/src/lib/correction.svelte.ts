import type { model } from './api'

// A sale being corrected at the cashier screen: its items fill the cart, and
// paying saves the new sale in its place (the old one is deleted with reason).
// shift is the corrected sale's shift while it is open: the new sale goes into
// it, so the screen sells without a shift of its own; otherwise it needs one.
export const correction = $state<{ sale: model.Sale | null; reason: string; shift: model.Shift | null }>({
  sale: null,
  reason: '',
  shift: null
})

export function startCorrection(sale: model.Sale, reason: string, shift: model.Shift | null) {
  correction.sale = sale
  correction.reason = reason
  correction.shift = shift
}

export function endCorrection() {
  correction.sale = null
  correction.reason = ''
  correction.shift = null
}
