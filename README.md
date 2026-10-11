# Tiket

Aplikasi desktop penjualan tiket untuk Windows: **Go + Wails v2 + SQLite + Svelte 5 + Tailwind v4**.

Fitur:
- **Kasir (POS):** pilih tiket dari grid atau cari, qty bisa lebih dari 1, bayar tunai, lalu nota tampil dan bisa dicetak ke printer thermal (ESC/POS, 58/80 mm).
- **Nota:** logo (bitmap) atau nama usaha, No/Date/Time In/Purpose/Cashier, label **Reprint N** untuk cetak ulang (setiap cetakan dicatat di `sale_prints`).
- **Tiket (admin):** tambah, ubah, hapus (soft delete), dan pulihkan.
- **Pengguna (admin):** role `admin` (semua menu) dan `kasir` (hanya penjualan).
- **Laporan (admin):** harian, mingguan (Senin–Minggu), bulanan, per tiket, dan daftar transaksi dengan cetak ulang nota.
- **Server/Client:** satu PC menyimpan database, PC lain terhubung lewat LAN.
- **Self-update:** cek update dari GitHub Releases, dengan file yang ditandatangani ed25519.

## Arsitektur

```
Svelte UI ──(binding Wails)──► app.go
                                 │
                       service.Service (interface)
                     ┌───────────┴────────────┐
             Local (mode Server)        Remote (mode Client)
             SQLite + HTTP API :8787 ◄── HTTP di LAN
```

| Folder | Isi |
|---|---|
| `internal/db` | Koneksi SQLite (modernc, tanpa CGO), migrasi `migrations/*.sql` + `PRAGMA user_version`, backup otomatis |
| `internal/store` | Query SQL (tanpa ORM) |
| `internal/service` | Aturan bisnis & hak akses (`local.go`), client HTTP (`remote.go`) |
| `internal/server` | API HTTP untuk PC client |
| `internal/receipt`, `internal/printer` | Susunan nota + ESC/POS, cetak RAW ke printer Windows |
| `internal/updater`, `cmd/tiket-release` | Self-update dan alat tanda tangan rilis |
| `frontend/src` | Tampilan Svelte |

### Nomor nota

Format: `{PREFIX}{KODE}{YYYYMMDD}{URUT 4 digit}`, contohnya `PNY25202610090003`.
- **Prefix** (misalnya `PNY`) diatur admin di Pengaturan dan berlaku untuk semua PC.
- **Kode** (misalnya `25`, maksimal 4 karakter) diatur per PC di Pengaturan, dan boleh kosong.
- Nomor urut mulai lagi dari 0001 setiap hari, dihitung per kombinasi prefix + kode.

### Purpose

Dihitung dari waktu transaksi, bukan disimpan sebagai kolom: Senin–Jumat → *Purpose hari kerja* (default `WEEKDAY REGULER`), Sabtu–Minggu → *Purpose akhir pekan* (default `WEEKEND REGULER`). Teksnya bisa diubah admin. Hari libur nasional **belum** dianggap weekend.

Uang disimpan sebagai **integer dalam sen** (Rp 12.500,50 → `1250050`) supaya tidak ada selisih pembulatan.

## Lokasi data (Windows)

- Database & konfigurasi: `%APPDATA%\Tiket\` (`tiket.db`, `config.json`)
- Backup otomatis: `%APPDATA%\Tiket\backups\` (harian + sebelum migrasi, 14 terakhir disimpan)
- Aplikasi terpasang per-user di `%LOCALAPPDATA%\Programs\Tiket\` (tanpa izin admin, sehingga update otomatis tidak memunculkan UAC)

Uninstall **tidak** menghapus database. Untuk pindah folder data saat pengujian, pakai env `TIKET_DATA_DIR`.

## Pengembangan (Linux)

Butuh Go, Node, dan Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0`).

```bash
wails dev -tags webkit2_41            # jalankan dengan hot reload
go test ./internal/...                # tes backend
cd frontend && npm run check          # cek tipe Svelte/TS
wails build -tags webkit2_41          # build Linux
wails build -platform windows/amd64   # build tiket.exe untuk Windows dari Linux
```

Di Linux, "printer" menyimpan hasil cetak sebagai file `.bin` ESC/POS di `<folder data>/print-out/`.

Uji mode client di satu PC: jalankan instance kedua dengan folder data lain, misalnya
`TIKET_DATA_DIR=/tmp/tiket-client ./build/bin/tiket`, lalu pilih Client → `127.0.0.1:8787`.

## Pemasangan di tempat usaha

1. **PC Server:** pasang, pilih **Server**, buat akun admin, isi tiket. Catat alamat yang muncul di Pengaturan (misalnya `192.168.1.10:8787`).
   - Beri **IP statis** pada PC server (lewat router atau Windows).
   - Saat pertama jalan, Windows Firewall akan bertanya. Pilih **Allow** untuk jaringan *Private*.
2. **PC Client:** pasang, pilih **Client**, isi alamat server, klik *Tes koneksi*.
3. **Printer:** di Pengaturan setiap PC, pilih printer thermal (driver Windows sudah terpasang), pilih lebar kertas, lalu klik *Cetak Tes*.

PC server harus menyala selama client dipakai. Sesi login tersimpan di memori server, jadi setelah server di-restart, semua kasir perlu login ulang.

## Rilis & update otomatis

Sekali saja:

```bash
go run ./cmd/tiket-release keygen
```

- Public key ditulis ke `internal/updater/publickey.txt`. **Commit** file ini.
- Kunci privat dicetak ke layar. Simpan sebagai GitHub secret **`TIKET_SIGNING_KEY`** dan **jangan** di-commit.
- Repo GitHub-nya harus **publik**, supaya aplikasi bisa mengunduh `latest.json` dan `tiket.exe` tanpa token. Kalau kode harus privat, terbitkan rilis di repo publik terpisah.

Setiap rilis:

```bash
git tag -a v0.2.0 -m "Perbaikan cetak nota"     # tambahkan [wajib] di pesan untuk update wajib
git push origin v0.2.0
```

GitHub Actions (`.github/workflows/release.yml`) akan menjalankan tes, membangun `tiket.exe` dan installer, menandatangani file, membuat `latest.json`, lalu menerbitkan rilis.

Aplikasi mengecek update saat dibuka dan lewat tombol **Cek Update** di Pengaturan. File update diverifikasi (sha256 + tanda tangan ed25519) sebelum mengganti exe. Client yang versi *major.minor*-nya berbeda dari server ditolak, jadi **update PC server lebih dulu**.

Build lokal (tanpa `-ldflags` dari workflow) tidak punya URL update, sehingga menampilkan "update belum dikonfigurasi".
