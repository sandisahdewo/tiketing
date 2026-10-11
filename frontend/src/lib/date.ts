const months = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']
const monthsLong = ['Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni', 'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember']
const days = ['Min', 'Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab']

export function toISODate(d: Date): string {
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

export function today(): string {
  return toISODate(new Date())
}

function parseDate(s: string): Date {
  const [y, m, d] = s.slice(0, 10).split('-').map(Number)
  return new Date(y, m - 1, d)
}

export function formatDate(s: string): string {
  const d = parseDate(s)
  return `${d.getDate().toString().padStart(2, '0')} ${months[d.getMonth()]} ${d.getFullYear()}`
}

export function formatDateTime(s: string | null | undefined): string {
  if (!s) return '-'
  return `${formatDate(s)} ${s.slice(11, 16)}`
}

export function formatTime(s: string): string {
  return s.slice(11, 16)
}

export function formatPeriod(period: string, kind: string): string {
  if (kind === 'monthly') {
    const [y, m] = period.split('-').map(Number)
    return `${monthsLong[m - 1]} ${y}`
  }
  const d = parseDate(period)
  if (kind === 'weekly') {
    const end = new Date(d)
    end.setDate(end.getDate() + 6)
    return `${formatDate(toISODate(d))} – ${formatDate(toISODate(end))}`
  }
  return `${days[d.getDay()]}, ${formatDate(period)}`
}

export function clockText(d: Date): string {
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${days[d.getDay()]}, ${pad(d.getDate())} ${months[d.getMonth()]} ${d.getFullYear()} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
