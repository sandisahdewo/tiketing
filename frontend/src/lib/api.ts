import { toast } from './toast.svelte'
import { session } from './session.svelte'
import { checkUpdate } from './update.svelte'

export * as App from '../../wailsjs/go/main/App'
export { model } from '../../wailsjs/go/models'

const sessionExpired = 'Sesi berakhir'
const versionMismatch = 'tidak cocok dengan server'

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
  // The server was updated (or this PC was): offer the matching version now.
  if (msg.includes(versionMismatch)) {
    checkUpdate().catch(() => {})
  }
  toast(msg, 'error')
  return msg
}
