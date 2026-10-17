export const categories = [
  { value: 'ticket', label: 'Tiket' },
  { value: 'topup', label: 'Topup Card' }
]

export function categoryLabel(value: string): string {
  return categories.find((c) => c.value === value)?.label ?? value
}

// Must match model.TicketColors. Classes are written out in full so Tailwind picks them up.
export const colors: { value: string; label: string; bg: string }[] = [
  { value: 'orange', label: 'Oranye', bg: 'bg-amber-500 hover:bg-amber-600' },
  { value: 'yellow', label: 'Kuning', bg: 'bg-yellow-500 hover:bg-yellow-600' },
  { value: 'red', label: 'Merah', bg: 'bg-red-600 hover:bg-red-700' },
  { value: 'rose', label: 'Mawar', bg: 'bg-rose-500 hover:bg-rose-600' },
  { value: 'pink', label: 'Merah muda', bg: 'bg-pink-500 hover:bg-pink-600' },
  { value: 'purple', label: 'Ungu', bg: 'bg-purple-600 hover:bg-purple-700' },
  { value: 'indigo', label: 'Nila', bg: 'bg-indigo-600 hover:bg-indigo-700' },
  { value: 'blue', label: 'Biru', bg: 'bg-blue-600 hover:bg-blue-700' },
  { value: 'sky', label: 'Biru langit', bg: 'bg-sky-500 hover:bg-sky-600' },
  { value: 'teal', label: 'Hijau toska', bg: 'bg-teal-600 hover:bg-teal-700' },
  { value: 'emerald', label: 'Hijau', bg: 'bg-emerald-600 hover:bg-emerald-700' },
  { value: 'lime', label: 'Hijau muda', bg: 'bg-lime-600 hover:bg-lime-700' },
  { value: 'slate', label: 'Abu-abu', bg: 'bg-slate-600 hover:bg-slate-700' }
]

export function colorClass(value: string): string {
  return (colors.find((c) => c.value === value) ?? colors[0]).bg
}
