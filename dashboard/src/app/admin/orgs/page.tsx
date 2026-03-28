import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

const orgs = [
  { name: 'Acme Corp', plan: 'Team', seatsUsed: 8, seatsTotal: 10, created: '2025-09-01' },
  { name: 'Startup Dev', plan: 'Pro', seatsUsed: 2, seatsTotal: 5, created: '2025-11-15' },
  { name: 'Example Inc', plan: 'Solo', seatsUsed: 1, seatsTotal: 1, created: '2026-01-20' },
  { name: 'Big Corp', plan: 'Enterprise', seatsUsed: 42, seatsTotal: 50, created: '2024-06-01' },
]

const planVariant: Record<string, 'default' | 'secondary' | 'outline'> = {
  Enterprise: 'default',
  Team: 'secondary',
  Pro: 'outline',
  Solo: 'outline',
}

export default function OrgsPage() {
  return (
    <div className="p-8 space-y-6">
      <h1 className="text-2xl font-semibold">Organizations</h1>

      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Org Name</TableHead>
              <TableHead>Plan</TableHead>
              <TableHead>Seats</TableHead>
              <TableHead>Created</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {orgs.map((org) => (
              <TableRow key={org.name}>
                <TableCell className="font-medium">{org.name}</TableCell>
                <TableCell>
                  <Badge variant={planVariant[org.plan] ?? 'outline'}>{org.plan}</Badge>
                </TableCell>
                <TableCell>
                  <span className={org.seatsUsed >= org.seatsTotal ? 'text-red-600 font-medium' : ''}>
                    {org.seatsUsed}
                  </span>
                  <span className="text-zinc-400">/{org.seatsTotal}</span>
                </TableCell>
                <TableCell className="text-zinc-500">{org.created}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}
