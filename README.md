# Aplikasi Monitor Agenda Pimpinan Kabupaten Sorong Selatan

Aplikasi full stack untuk monitoring agenda pimpinan Kabupaten Sorong Selatan.
Backend dibuat dengan Golang, database menggunakan MySQL, dan frontend memakai
HTML, CSS, serta Tailwind CSS.

## Fitur

- Login dua aktor: staf dan pimpinan
- Daftar akun baru dengan pilihan role staf atau pimpinan
- Staf menginput jadwal kunjungan pimpinan
- Staf upload undangan dan mengisi keterangan agenda
- Pimpinan menerima alert dan email saat agenda disubmit
- Pimpinan memilih hadir sendiri atau diwakili
- Pimpinan memilih pejabat yang mewakili dan mengisi keterangan validasi
- Pimpinan dapat menarik kembali validasi selama kegiatan masih lebih dari 24 jam
- Validasi terkunci jika kegiatan kurang dari 24 jam atau sudah berjalan
- Status agenda otomatis: menunggu validasi, tervalidasi, terkunci, sedang berlangsung, menunggu dokumentasi, selesai
- Staf upload dokumentasi kegiatan sebagai laporan setelah kegiatan selesai
- Pimpinan dan staf dapat download undangan serta dokumentasi

## Akun Demo

| Role | Email | Password |
| --- | --- | --- |
| Staf | `staf@sorsel.go.id` | `agenda123` |
| Pimpinan | `pimpinan@sorsel.go.id` | `agenda123` |

User baru juga bisa dibuat dari tab `Daftar` di halaman login.

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

## Email Notifikasi

Jika SMTP belum diatur, sistem tetap mencatat email sebagai simulasi di log backend.
Untuk mengirim email sungguhan, jalankan backend dengan environment berikut:

```bash
SMTP_HOST="smtp.gmail.com" \
SMTP_PORT="587" \
SMTP_USERNAME="email@example.com" \
SMTP_PASSWORD="app-password" \
SMTP_FROM="email@example.com" \
MYSQL_DSN="root:root@tcp(127.0.0.1:3306)/agenda_monitor?parseTime=true&multiStatements=true" \
go run .
```

## Catatan Implementasi

Sesi login saat ini disimpan di memori backend. Untuk produksi, sesi sebaiknya
dipindahkan ke database atau Redis agar tetap aktif saat server restart dan aman
untuk beberapa instance server.
