export function formatMoney(sen: number): string {
  const negative = sen < 0
  const abs = Math.abs(Math.trunc(sen))
  const rupiah = Math.floor(abs / 100)
  const cents = abs % 100
  let text = rupiah.toString().replace(/\B(?=(\d{3})+(?!\d))/g, '.')
  if (cents !== 0) text += ',' + cents.toString().padStart(2, '0')
  return (negative ? '-' : '') + text
}

export function formatRp(sen: number): string {
  return 'Rp ' + formatMoney(sen)
}

export function parseMoney(input: string): number | null {
  let s = input.trim().replace(/^rp\.?\s*/i, '').replace(/\s/g, '')
  if (s === '') return null
  let whole: string
  let frac = ''
  if (s.includes(',')) {
    const parts = s.split(',')
    if (parts.length !== 2) return null
    whole = parts[0].replace(/\./g, '')
    frac = parts[1]
  } else if (s.includes('.')) {
    const groups = s.split('.')
    const thousands = groups.slice(1).every((g) => g.length === 3)
    if (thousands) {
      whole = groups.join('')
    } else if (groups.length === 2) {
      whole = groups[0]
      frac = groups[1]
    } else {
      return null
    }
  } else {
    whole = s
  }
  if (whole === '') whole = '0'
  if (!/^\d+$/.test(whole) || !/^\d{0,2}$/.test(frac)) return null
  const value = Number(whole) * 100 + Number(frac.padEnd(2, '0'))
  return Number.isSafeInteger(value) ? value : null
}

function isSignificant(ch: string): boolean {
  return (ch >= '0' && ch <= '9') || ch === ','
}

export function formatTyping(raw: string, caret: number): { text: string; caret: number } {
  let sig = 0
  for (const ch of raw.slice(0, caret)) if (isSignificant(ch)) sig++

  let whole = ''
  let frac = ''
  let hasComma = false
  let kept = 0
  let keptBeforeCaret = 0
  for (let i = 0; i < raw.length; i++) {
    const ch = raw[i]
    let keep = false
    if (ch === ',' && !hasComma) {
      hasComma = true
      keep = true
    } else if (ch >= '0' && ch <= '9') {
      if (!hasComma) {
        whole += ch
        keep = true
      } else if (frac.length < 2) {
        frac += ch
        keep = true
      }
    }
    if (keep) {
      kept++
      if (i < caret) keptBeforeCaret++
    }
  }
  sig = Math.min(sig, keptBeforeCaret)

  const trimmed = whole.replace(/^0+(?=\d)/, '')
  sig -= Math.min(whole.length - trimmed.length, sig)
  whole = trimmed
  if (whole === '' && hasComma) {
    whole = '0'
    sig++
  }

  const text = whole.replace(/\B(?=(\d{3})+(?!\d))/g, '.') + (hasComma ? ',' + frac : '')
  let pos = 0
  let seen = 0
  while (pos < text.length && seen < sig) {
    if (isSignificant(text[pos])) seen++
    pos++
  }
  return { text, caret: pos }
}
