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
