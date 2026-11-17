import { toast } from './toast.svelte'
import { session } from './session.svelte'
import { checkUpdate } from './update.svelte'
import { alerted, noteError } from './connection.svelte'
import * as bindings from '../../wailsjs/go/main/App'

type Call = (...args: unknown[]) => Promise<unknown>

// Every call to the Go side goes through here, so a server that cannot be
// reached is noticed whichever screen asked, also where the error itself is
// not shown.
export const App = Object.fromEntries(
  Object.entries(bindings).map(([name, fn]) => [
    name,
    (...args: unknown[]) =>
      (fn as Call)(...args).catch((e: unknown) => {
        noteError(errorMessage(e))
        throw e
      })
  ])
) as typeof bindings
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
  if (!alerted(msg)) toast(msg, 'error')
  return msg
}
