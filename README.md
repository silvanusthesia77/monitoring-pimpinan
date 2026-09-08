# Aplikasi Monitor Agenda Pimpinan Kabupaten Sorong Selatan

Aplikasi full stack untuk monitoring agenda pimpinan Kabupaten Sorong Selatan.
Backend dibuat dengan Golang, database menggunakan MySQL, dan frontend memakai
HTML, CSS, serta Tailwind CSS.

## Fitur

- Login tiga aktor: admin, staf, dan pimpinan
- Admin mengelola user dan dapat menghapus akun user
- Daftar staf/pimpinan memakai verifikasi kode OTP ke akun Gmail aktif
- Staf menginput jadwal kunjungan pimpinan
- Staf upload undangan dan mengisi keterangan agenda
- Pimpinan menerima alert dan email saat agenda disubmit
- Pimpinan memilih hadir sendiri atau diwakili
- Pimpinan memilih pejabat yang mewakili dan mengisi keterangan validasi
- Pimpinan dapat menarik kembali validasi selama kegiatan masih lebih dari 24 jam
- Validasi terkunci jika kegiatan kurang dari 24 jam atau sudah berjalan
- Status agenda otomatis: menunggu validasi, tervalidasi, terkunci, sedang berlangsung, menunggu dokumentasi, selesai
- Staf upload dokumentasi kegiatan dari menu Dokumentasi dengan memilih agenda
- Laporan/berita acara tampil di menu Laporan dan dapat dilihat oleh staf serta pimpinan
- Pimpinan dan staf dapat download undangan, dokumentasi, serta PDF berita acara

## Akun Demo

| Role | Email | Password |
| --- | --- | --- |
| Admin | `admin@sorsel.go.id` | `agenda123` |
| Staf | `staf@sorsel.go.id` | `agenda123` |
| Pimpinan | `sergiodyego45@gmail.com` | `agenda123` |

Tab `Daftar` dipakai untuk staf/pimpinan dengan akun Gmail aktif. Sistem akan
mengirim kode verifikasi ke Gmail tersebut, lalu akun dibuat setelah kode benar.
Admin memakai akun bawaan dan mengelola user dari menu `Kelola User`.

## Menjalankan Dengan Docker

Pastikan Docker sudah aktif, lalu jalankan:

```bash
docker compose up --build
```

Buka aplikasi:

```text
http://localhost:8080
```

MySQL tersedia di:

```text
localhost:3306
database: agenda_monitor
user: root
password: root
```

## Struktur Project

```text
backend/
  main.go
  go.mod
  Dockerfile
  .env.example

database/
  schema.sql

frontend/
  index.html
  styles.css
  app.js

docker-compose.yml
```

## Menjalankan Backend Tanpa Docker

Jika Go dan MySQL sudah terpasang di komputer:

```bash
cd backend
go mod download
MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/agenda_monitor?parseTime=true&multiStatements=true" go run .
```

Backend otomatis membaca `database/schema.sql`, membuat tabel, dan membuat akun
demo jika tabel pengguna masih kosong.

## Email Notifikasi Gmail

Email asli dikirim hanya untuk notifikasi yang memang ditujukan ke pimpinan, misalnya
saat staf submit agenda baru. Akun pimpinan default memakai `sergiodyego45@gmail.com`.

Password login aplikasi tetap password akun aplikasi, misalnya `agenda123` atau
password yang dibuat sendiri dari tab `Daftar`. Verifikasi daftar juga memakai
SMTP Gmail untuk mengirim kode OTP. Untuk SMTP Gmail, gunakan
**App Password** khusus pengiriman email, bukan password login Gmail biasa. Setelah
App Password dibuat, jalankan backend dengan environment berikut:

```bash
SMTP_HOST="smtp.gmail.com" \
SMTP_PORT="587" \
SMTP_USERNAME="sergiodyego45@gmail.com" \
SMTP_PASSWORD="app-password" \
SMTP_FROM="sergiodyego45@gmail.com" \
MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/agenda_monitor?parseTime=true&multiStatements=true" \
go run .
```

Contoh PowerShell Windows:

```powershell
$env:SMTP_HOST="smtp.gmail.com"
$env:SMTP_PORT="587"
$env:SMTP_USERNAME="sergiodyego45@gmail.com"
$env:SMTP_PASSWORD="app-password"
$env:SMTP_FROM="sergiodyego45@gmail.com"
$env:MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/agenda_monitor?parseTime=true&multiStatements=true"
go run .
```

Jika SMTP belum diatur, sistem hanya mencatat simulasi email di log backend dan
kolom notifikasi tidak ditandai sebagai email terkirim.

## Catatan Implementasi

Sesi login saat ini disimpan di memori backend. Untuk produksi, sesi sebaiknya
dipindahkan ke database atau Redis agar tetap aktif saat server restart dan aman
untuk beberapa instance server.
