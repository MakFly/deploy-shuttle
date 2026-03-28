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

const members = [
  { email: 'alice@example.com', role: 'owner', joined: '2026-01-15' },
  { email: 'bob@example.com', role: 'admin', joined: '2026-01-20' },
  { email: 'carol@example.com', role: 'member', joined: '2026-02-05' },
  { email: 'dave@example.com', role: 'member', joined: '2026-03-01' },
]

const roleVariant: Record<string, 'default' | 'secondary' | 'outline'> = {
  owner: 'default',
  admin: 'secondary',
  member: 'outline',
}

export default function TeamPage() {
  return (
    <div className="p-8 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Team</h1>
        <Button>Invite Member</Button>
      </div>

      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Email</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Joined</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {members.map((member) => (
              <TableRow key={member.email}>
                <TableCell>{member.email}</TableCell>
                <TableCell>
                  <Badge variant={roleVariant[member.role] ?? 'outline'}>{member.role}</Badge>
                </TableCell>
                <TableCell className="text-zinc-500">{member.joined}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Card>
    </div>
  )
}
