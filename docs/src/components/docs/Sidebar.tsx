import { cn } from '@/lib/utils'

interface SidebarProps {
	section?: string
}

const navigation = [
	{
		title: 'Getting Started',
		items: [
			{ title: 'Introduction', href: '/docs' },
			{ title: 'Installation', href: '/docs/installation' },
			{ title: 'Quick Start', href: '/docs/getting-started' },
			{ title: 'Project Scaffolding', href: '/docs/scaffolding' },
		],
	},
	{
		title: 'Configuration',
		items: [
			{ title: 'shuttle.yml', href: '/docs/configuration' },
			{ title: 'Services', href: '/docs/configuration/services' },
			{ title: 'Accessories', href: '/docs/configuration/accessories' },
			{ title: 'Environment Overlays', href: '/docs/configuration/environments' },
		],
	},
	{
		title: 'Deploy Strategies',
		items: [
			{ title: 'Blue-Green', href: '/docs/strategies/blue-green' },
			{ title: 'Rolling', href: '/docs/strategies/rolling' },
			{ title: 'Docker Swarm', href: '/docs/strategies/swarm' },
		],
	},
	{
		title: 'Features',
		items: [
			{ title: 'Health Checks', href: '/docs/features/healthchecks' },
			{ title: 'Secrets Management', href: '/docs/features/secrets' },
			{ title: 'SSL & Proxy', href: '/docs/features/proxy-ssl' },
			{ title: 'Multi-Server', href: '/docs/features/multi-server' },
			{ title: 'Hooks', href: '/docs/features/hooks' },
			{ title: 'Notifications', href: '/docs/features/notifications' },
			{ title: 'Registry Providers', href: '/docs/features/registries' },
		],
	},
	{
		title: 'Local Development',
		items: [
			{ title: 'Dev Environment', href: '/docs/dev' },
			{ title: 'SSL with mkcert', href: '/docs/dev/ssl' },
		],
	},
	{
		title: 'CI/CD',
		items: [
			{ title: 'GitHub Actions', href: '/docs/ci/github' },
			{ title: 'GitLab CI', href: '/docs/ci/gitlab' },
		],
	},
	{
		title: 'CLI Reference',
		items: [
			{ title: 'Commands', href: '/docs/commands' },
			{ title: 'shuttle deploy', href: '/docs/commands/deploy' },
			{ title: 'shuttle provision', href: '/docs/commands/provision' },
			{ title: 'shuttle rollback', href: '/docs/commands/rollback' },
			{ title: 'shuttle secrets', href: '/docs/commands/secrets' },
			{ title: 'shuttle dev', href: '/docs/commands/dev' },
			{ title: 'shuttle ci', href: '/docs/commands/ci' },
			{ title: 'shuttle new', href: '/docs/commands/new' },
		],
	},
]

export function Sidebar({ section }: SidebarProps) {
	return (
		<aside className="hidden lg:block w-64 shrink-0 border-r">
			<div className="sticky top-14 h-[calc(100vh-3.5rem)] overflow-y-auto py-6 px-4">
				<nav className="space-y-6">
					{navigation.map((group) => (
						<div key={group.title}>
							<h4 className="mb-2 px-2 text-sm font-semibold tracking-tight">
								{group.title}
							</h4>
							<div className="space-y-1">
								{group.items.map((item) => (
									<a
										key={item.href}
										href={item.href}
										className={cn(
											'block rounded-md px-2 py-1.5 text-sm transition-colors',
											'text-muted-foreground hover:text-foreground hover:bg-accent',
										)}
									>
										{item.title}
									</a>
								))}
							</div>
						</div>
					))}
				</nav>
			</div>
		</aside>
	)
}
