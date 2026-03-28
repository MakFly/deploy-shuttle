import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

const licenses = [
  {
    email: 'alice@example.com',
    plan: 'Pro',
    features: ['blue-green', 'rolling', 'secrets'],
    issued: '2026-01-15',
    expires: '2027-01-15',
    status: 'active',
  },
  {
    email: 'bob@acme.io',
    plan: 'Team',
    features: ['blue-green', 'rolling', 'secrets', 'swarm'],
    issued: '2025-11-01',
    expires: '2026-11-01',
    status: 'active',
  },
  {
    email: 'carol@startup.dev',
    plan: 'Solo',
    features: ['blue-green'],
    issued: '2024-03-10',
    expires: '2025-03-10',
    status: 'expired',
  },
]

export default function LicensesPage() {
  return (
    <div className="p-8 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Licenses</h1>
        <Button>Issue License</Button>
      </div>

      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Email</TableHead>
              <TableHead>Plan</TableHead>
              <TableHead>Features</TableHead>
              <TableHead>Issued</TableHead>
              <TableHead>Expires</TableHead>
              <TableHead>Status</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {licenses.map((row) => (
              <TableRow key={row.email}>
                <TableCell>{row.email}</TableCell>
                <TableCell>{row.plan}</TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1">
                    {row.features.map((f) => (
                      <Badge key={f} variant="outline" className="text-xs">
                        {f}
                      </Badge>
                    ))}
                  </div>
                </TableCell>
                <TableCell className="text-zinc-500">{row.issued}</TableCell>
                <TableCell className="text-zinc-500">{row.expires}</TableCell>
                <TableCell>
                  <Badge variant={row.status === 'active' ? 'default' : 'destructive'}>
                    {row.status}
                  </Badge>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}
