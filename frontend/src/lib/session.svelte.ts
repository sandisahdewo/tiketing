import type { main, model } from '../../wailsjs/go/models'

export const session = $state<{
  info: main.AppInfo | null
  user: model.User | null
}>({
  info: null,
  user: null
})

export function isAdmin(): boolean {
  return session.user?.role === 'admin'
}
