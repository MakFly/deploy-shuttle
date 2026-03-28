import { NextResponse } from 'next/server'

const mockLicenses = [
  {
    id: 'lic_001',
    email: 'alice@example.com',
    plan: 'Pro',
    features: ['blue-green', 'rolling', 'secrets'],
    issued: '2026-01-15',
    expires: '2027-01-15',
    status: 'active',
  },
  {
    id: 'lic_002',
    email: 'bob@acme.io',
    plan: 'Team',
    features: ['blue-green', 'rolling', 'secrets', 'swarm'],
    issued: '2025-11-01',
    expires: '2026-11-01',
    status: 'active',
  },
  {
    id: 'lic_003',
    email: 'carol@startup.dev',
    plan: 'Solo',
    features: ['blue-green'],
    issued: '2024-03-10',
    expires: '2025-03-10',
    status: 'expired',
  },
]

export async function GET() {
  return NextResponse.json({ licenses: mockLicenses })
}

export async function POST() {
  return NextResponse.json({ success: true })
}
