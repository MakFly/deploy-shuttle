import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

const account = {
  email: 'alice@example.com',
  plan: 'Pro',
  features: ['blue-green', 'rolling', 'secrets', 'notifications'],
  expires: '2027-01-15',
}

const recentDeploys = [
  { date: '2026-03-28', app: 'api-server', env: 'production', status: 'success' },
  { date: '2026-03-27', app: 'web-frontend', env: 'production', status: 'success' },
  { date: '2026-03-26', app: 'worker', env: 'staging', status: 'success' },
  { date: '2026-03-25', app: 'api-server', env: 'production', status: 'error' },
]

export default function AccountPage() {
  return (
    <div className="p-8 space-y-8">
      <h1 className="text-2xl font-semibold">My Account</h1>

      <Card className="max-w-md">
        <CardHeader>
          <CardTitle className="text-base">License</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex justify-between text-sm">
            <span className="text-zinc-500">Email</span>
            <span>{account.email}</span>
          </div>
          <div className="flex justify-between text-sm">
            <span className="text-zinc-500">Plan</span>
            <Badge>{account.plan}</Badge>
          </div>
          <div className="flex justify-between text-sm">
            <span className="text-zinc-500">Expires</span>
            <span>{account.expires}</span>
          </div>
          <div className="space-y-1">
            <span className="text-sm text-zinc-500">Features</span>
            <div className="flex flex-wrap gap-1 mt-1">
              {account.features.map((f) => (
                <Badge key={f} variant="outline" className="text-xs">
                  {f}
                </Badge>
              ))}
            </div>
          </div>
        </CardContent>
      </Card>

      <div>
        <h2 className="text-lg font-medium mb-4">Recent Deploys</h2>
        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Date</TableHead>
                <TableHead>App</TableHead>
                <TableHead>Environment</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {recentDeploys.map((row, i) => (
                <TableRow key={i}>
                  <TableCell className="text-zinc-500">{row.date}</TableCell>
                  <TableCell className="font-medium">{row.app}</TableCell>
                  <TableCell>{row.env}</TableCell>
                  <TableCell>
                    <Badge variant={row.status === 'success' ? 'default' : 'destructive'}>
                      {row.status}
                    </Badge>
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
