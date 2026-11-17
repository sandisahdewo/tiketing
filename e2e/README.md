# Tes e2e pembayaran di kasir

Menguji bagian yang menyangkut uang: apa yang tercatat di server kalau koneksi antara PC client dan server bermasalah tepat saat kasir membayar. Aturannya satu: **satu pembelian tidak boleh tercatat dua kali tanpa keputusan kasir.**

Yang dijalankan sungguhan: aplikasi server, aplikasi client (halamannya diklik lewat browser otomatis), dan sebuah perantara di antara keduanya yang bisa membuang permintaan, membuang jawaban, menahan jawaban lebih lama dari batas tunggu client, atau mematikan server secara mendadak. Setelah tiap skenario, jumlah nota dan nilai uang di database server dibandingkan dengan yang seharusnya.

## Menjalankan

Butuh yang sama dengan pengembangan (Go, Node, Wails CLI), ditambah `sqlite3`, `curl`, dan Chrome/Chromium.

```bash
cd e2e && npm install                 # sekali saja
npx playwright-core install chromium  # sekali saja; atau pakai E2E_CHROME di bawah
./run.sh                              # sekitar 6 menit
```

- `E2E_CHROME=/usr/bin/google-chrome-stable ./run.sh` memakai Chrome yang sudah terpasang.
- `E2E_ONLY=07 ./run.sh` hanya menjalankan skenario yang namanya diawali `07`.
- Selama tes berjalan, jendela aplikasi server dan client muncul di layar; biarkan saja.
- Port 18790, 18791, dan 34115 harus bebas. Tes memakai folder sementara, bukan data asli.
- Kalau ada yang gagal, folder sementaranya tidak dihapus: di dalamnya ada log dan tangkapan layar skenario yang gagal.

Tes ini tidak dijalankan di CI (butuh layar dan Linux); jalankan sendiri sebelum rilis yang menyentuh kasir, penjualan, sesi login, atau koneksi client-server.

## Isinya

| File | Isi |
|---|---|
| `run.sh` | Build, menyiapkan folder server dan client, menyalakan client dalam mode dev, menjalankan tes |
| `pay.test.cjs` | Perantara, langkah-langkah kasir, dan semua skenario |
| `seed/main.go` | Membuat database server dengan admin pertama (admin pertama tidak bisa dibuat lewat jaringan) |

Skenarionya: penjualan biasa dan tombol bayar ditekan berkali-kali; jawaban atau permintaan hilang lalu kasir membayar lagi, dengan keranjang dikosongkan, pindah menu, logout, halaman dimuat ulang, isi keranjang diubah, pembeli berikutnya, uang diterima berbeda, sesi berakhir, server ditutup-buka, server mati mendadak, server lambat, kasir lain di PC yang sama; koreksi nota oleh admin; kiriman bersamaan langsung ke server; kiriman ulang setelah tiket dinonaktifkan, nota dihapus, atau shift ditutup; dan kecocokan uang shift dengan nota.

## Yang tidak tercakup

- Windows, dan dua komputer dengan WiFi sungguhan. Coba di tempat: cabut WiFi tepat setelah menekan Bayar, sambungkan lagi, lalu bayar lagi.
- Menutup dan membuka aplikasi client yang sebenarnya (yang diuji: memuat ulang halaman).
- Listrik padam di PC server (yang diuji: aplikasinya dimatikan paksa).
- Kasir pindah ke PC lain setelah error: nomor transaksinya tersimpan di PC pertama, jadi di PC kedua tidak ada peringatan.
