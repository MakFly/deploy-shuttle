import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'

const billing = {
  plan: 'Pro',
  price: '$29/month',
  nextBillingDate: '2026-04-15',
  paymentMethod: 'Visa ending in 4242',
}

export default function BillingPage() {
  return (
    <div className="p-8 space-y-6">
      <h1 className="text-2xl font-semibold">Billing</h1>

      <Card className="max-w-md">
        <CardHeader>
          <CardTitle className="text-base">Current Plan</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex justify-between items-center text-sm">
            <span className="text-zinc-500">Plan</span>
            <div className="flex items-center gap-2">
              <Badge>{billing.plan}</Badge>
              <span className="font-medium">{billing.price}</span>
            </div>
          </div>
          <div className="flex justify-between text-sm">
            <span className="text-zinc-500">Next billing date</span>
            <span>{billing.nextBillingDate}</span>
          </div>
          <div className="flex justify-between text-sm">
            <span className="text-zinc-500">Payment method</span>
            <span>{billing.paymentMethod}</span>
          </div>

          <Separator />

          <a
            href="https://billing.stripe.com/p/login/placeholder"
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center justify-center rounded-md bg-zinc-900 text-white text-sm font-medium px-4 py-2 w-full hover:bg-zinc-700 transition-colors"
          >
            Manage Subscription
          </a>
        </CardContent>
      </Card>
    </div>
  )
}
