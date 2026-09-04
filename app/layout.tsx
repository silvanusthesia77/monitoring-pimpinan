import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Aplikasi Monitor Agenda Pimpinan Kabupaten Sorong Selatan",
  description:
    "Aplikasi monitoring agenda pimpinan untuk input jadwal, validasi kehadiran, delegasi, notifikasi, dan laporan dokumentasi kegiatan.",
  openGraph: {
    title: "Aplikasi Monitor Agenda Pimpinan Kabupaten Sorong Selatan",
    description:
      "Pantau agenda pimpinan, validasi kehadiran, delegasi, undangan, dan laporan dokumentasi kegiatan.",
    type: "website",
  },
  icons: {
    icon: "/favicon.svg",
    shortcut: "/favicon.svg",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="id">
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased`}
      >
        {children}
      </body>
    </html>
  );
}
