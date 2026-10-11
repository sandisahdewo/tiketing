import { toast } from './toast.svelte'
import { session } from './session.svelte'

export * as App from '../../wailsjs/go/main/App'
export { model } from '../../wailsjs/go/models'

const sessionExpired = 'Sesi berakhir'

export function errorMessage(err: unknown): string {
  if (typeof err === 'string') return err
  if (err instanceof Error) return err.message
  return String(err)
}

export function handleError(err: unknown): string {
  const msg = errorMessage(err)
  if (msg.includes(sessionExpired)) {
    session.user = null
  }
  toast(msg, 'error')
  return msg
}
