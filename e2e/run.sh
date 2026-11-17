#!/usr/bin/env bash
# Runs the end-to-end tests of paying at the cashier screen: builds the app,
# starts a server and a client in a temporary folder, and drives the client's
# page while a proxy between the two loses requests and answers.
# See README.md in this folder.
set -euo pipefail
cd "$(dirname "$0")/.."

WAILS="${WAILS:-wails}"
TAGS="${WAILS_TAGS:-webkit2_41}"
dir="$(mktemp -d -t tiket-e2e.XXXXXX)"
dev=""

cleanup() {
  status=$?
  # wails dev starts the app and the frontend watcher; stop the whole group.
  if [ -n "$dev" ]; then kill -- "-$dev" 2>/dev/null || true; fi
  if [ $status -eq 0 ]; then rm -rf "$dir"; else echo "Gagal. Log dan tangkapan layar: $dir" >&2; fi
}
trap cleanup EXIT

[ -d e2e/node_modules ] || (cd e2e && npm install --no-audit --no-fund)

"$WAILS" build -tags "$TAGS" >/dev/null
mkdir -p "$dir/server" "$dir/client" "$dir/shots"
echo '{"mode":"server","port":18790}' > "$dir/server/config.json"
echo '{"mode":"client","serverUrl":"http://127.0.0.1:18791","deviceId":"e2e-client"}' > "$dir/client/config.json"
go run ./e2e/seed "$dir/server"

# The client runs in dev mode: only then can a browser load its page and
# reach the Go side of the app.
TIKET_DATA_DIR="$dir/client" setsid "$WAILS" dev -tags "$TAGS" > "$dir/dev.log" 2>&1 &
dev=$!
for _ in $(seq 1 180); do
  curl -fs -o /dev/null http://localhost:34115/ && break
  sleep 1
done
curl -fs -o /dev/null http://localhost:34115/ || { echo "wails dev tidak menyala; lihat $dir/dev.log" >&2; exit 1; }

E2E_DIR="$dir" E2E_BIN="$PWD/build/bin/tiket" node e2e/pay.test.cjs
