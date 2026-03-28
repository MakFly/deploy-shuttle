import type { Metadata } from 'next'
import { Geist, Geist_Mono } from 'next/font/google'
import Link from 'next/link'
import './globals.css'

const geistSans = Geist({
  variable: '--font-geist-sans',
  subsets: ['latin'],
})

const geistMono = Geist_Mono({
  variable: '--font-geist-mono',
  subsets: ['latin'],
})

export const metadata: Metadata = {
  title: 'Shuttle Dashboard',
  description: 'Shuttle deployment management dashboard',
}

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html lang="en" className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}>
      <body className="min-h-full flex">
        <nav className="w-56 shrink-0 border-r bg-zinc-50 flex flex-col gap-1 p-4 min-h-screen">
          <p className="text-xs font-semibold text-zinc-400 uppercase tracking-wider px-2 mb-2">
            Admin
          </p>
          <Link
            href="/admin"
            className="px-2 py-1.5 rounded text-sm text-zinc-700 hover:bg-zinc-200 transition-colors"
          >
            Overview
          </Link>
          <Link
            href="/admin/licenses"
            className="px-2 py-1.5 rounded text-sm text-zinc-700 hover:bg-zinc-200 transition-colors"
          >
            Licenses
          </Link>
          <Link
            href="/admin/orgs"
            className="px-2 py-1.5 rounded text-sm text-zinc-700 hover:bg-zinc-200 transition-colors"
          >
            Organizations
          </Link>

          <p className="text-xs font-semibold text-zinc-400 uppercase tracking-wider px-2 mt-4 mb-2">
            Account
          </p>
          <Link
            href="/account"
            className="px-2 py-1.5 rounded text-sm text-zinc-700 hover:bg-zinc-200 transition-colors"
          >
            My Account
          </Link>
          <Link
            href="/account/team"
            className="px-2 py-1.5 rounded text-sm text-zinc-700 hover:bg-zinc-200 transition-colors"
          >
            Team
          </Link>
          <Link
            href="/account/billing"
            className="px-2 py-1.5 rounded text-sm text-zinc-700 hover:bg-zinc-200 transition-colors"
          >
            Billing
          </Link>
        </nav>
        <main className="flex-1 overflow-auto">{children}</main>
      </body>
    </html>
  )
}
