import { CheckUpdate } from '../../wailsjs/go/main/App'
import type { updater } from '../../wailsjs/go/models'

export const updateState = $state<{
  info: updater.Info | null
  dismissed: boolean
  installing: boolean
}>({
  info: null,
  dismissed: false,
  installing: false
})

let checking: Promise<updater.Info> | null = null

// checkUpdate asks for the newest version (the server PC's, on a client) and
// shows the banner again when a version turns up that was not offered yet.
export function checkUpdate(): Promise<updater.Info> {
  checking ??= CheckUpdate()
    .then((u) => {
      if (u.latest !== updateState.info?.latest) updateState.dismissed = false
      updateState.info = u
      return u
    })
    .finally(() => (checking = null))
  return checking
}

// The app is often left open all day, so it keeps looking.
const every = 30 * 60 * 1000

export function watchUpdates(): () => void {
  const run = () => checkUpdate().catch(() => {})
  run()
  const timer = setInterval(run, every)
  return () => clearInterval(timer)
}
