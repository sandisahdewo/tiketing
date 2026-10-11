type Toast = { id: number; kind: 'success' | 'error' | 'info'; text: string }

let nextId = 1

export const toasts = $state<Toast[]>([])

export function toast(text: string, kind: Toast['kind'] = 'success') {
  const id = nextId++
  toasts.push({ id, kind, text })
  if (toasts.length > 3) toasts.splice(0, toasts.length - 3)
  setTimeout(() => {
    const i = toasts.findIndex((t) => t.id === id)
    if (i >= 0) toasts.splice(i, 1)
  }, kind === 'error' ? 6000 : 3000)
}
