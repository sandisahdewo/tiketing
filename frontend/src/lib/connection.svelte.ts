// A request that did not reach the server (client PCs only). It is shown as
// one alert over the app while a user is logged in (components/ServerAlert);
// before login the start and setup screens show the problem themselves.
export const connection = $state({ watching: false, error: '' })

// How service.Remote words it (internal/service/remote.go; keep the two the
// same). The check for an update says it differently, on purpose: it runs in
// the background and must not raise the alert.
const marker = 'Tidak bisa terhubung ke server '

// unreachable reports whether msg says that the server did not answer, also
// when the Go side put its own words in front of it.
export function unreachable(msg: string): boolean {
  return msg.includes(marker)
}

// alerted reports whether msg is shown by the alert, so it needs no toast.
export function alerted(msg: string): boolean {
  return connection.watching && unreachable(msg)
}

export function noteError(msg: string) {
  if (alerted(msg)) connection.error = msg
}
