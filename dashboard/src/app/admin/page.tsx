import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

const stats = [
  { label: 'Total Licenses', value: '142' },
  { label: 'Active Seats', value: '89' },
  { label: 'MRR', value: '$4,320' },
  { label: 'Total Deploys', value: '8,741' },
]

const recentActivity = [
  { date: '2026-03-28', user: 'alice@example.com', action: 'Deploy', status: 'success' },
  { date: '2026-03-28', user: 'bob@acme.io', action: 'Provision', status: 'success' },
  { date: '2026-03-27', user: 'carol@startup.dev', action: 'Rollback', status: 'warning' },
  { date: '2026-03-27', user: 'dave@corp.com', action: 'Deploy', status: 'error' },
  { date: '2026-03-26', user: 'eve@example.com', action: 'Deploy', status: 'success' },
]

const statusVariant: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
  success: 'default',
  warning: 'secondary',
  error: 'destructive',
}

export default function AdminOverviewPage() {
  return (
    <div className="p-8 space-y-8">
      <h1 className="text-2xl font-semibold">Shuttle Admin</h1>

      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        {stats.map((stat) => (
          <Card key={stat.label}>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-zinc-500">{stat.label}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-2xl font-bold">{stat.value}</p>
            </CardContent>
          </Card>
        ))}
      </div>

      <div>
        <h2 className="text-lg font-medium mb-4">Recent Activity</h2>
        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Date</TableHead>
                <TableHead>User</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {recentActivity.map((row, i) => (
                <TableRow key={i}>
                  <TableCell className="text-zinc-500">{row.date}</TableCell>
                  <TableCell>{row.user}</TableCell>
                  <TableCell>{row.action}</TableCell>
                  <TableCell>
                    <Badge variant={statusVariant[row.status] ?? 'outline'}>{row.status}</Badge>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      </div>
    </div>
  )
}
