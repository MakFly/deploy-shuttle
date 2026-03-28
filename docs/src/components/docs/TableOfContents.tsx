import { cn } from '@/lib/utils'
import { useEffect, useState } from 'react'

interface TocItem {
	id: string
	text: string
	level: number
}

export function TableOfContents() {
	const [headings, setHeadings] = useState<TocItem[]>([])
	const [activeId, setActiveId] = useState<string>('')

	useEffect(() => {
		const elements = Array.from(document.querySelectorAll('h2, h3')) as HTMLElement[]

		const items: TocItem[] = elements.map((el) => ({
			id: el.id,
			text: el.textContent ?? '',
			level: Number.parseInt(el.tagName[1], 10),
		}))

		setHeadings(items)

		const observer = new IntersectionObserver(
			(entries) => {
				for (const entry of entries) {
					if (entry.isIntersecting) {
						setActiveId(entry.target.id)
					}
				}
			},
			{ rootMargin: '0% 0% -80% 0%' },
		)

		elements.forEach((el) => observer.observe(el))
		return () => observer.disconnect()
	}, [])

	if (headings.length === 0) return null

	return (
		<aside className="hidden xl:block w-56 shrink-0">
			<div className="sticky top-14 h-[calc(100vh-3.5rem)] overflow-y-auto py-6 pl-4">
				<p className="mb-3 text-sm font-semibold">On this page</p>
				<nav className="space-y-1">
					{headings.map((heading) => (
						<a
							key={heading.id}
							href={`#${heading.id}`}
							className={cn(
								'block text-sm transition-colors',
								heading.level === 3 && 'pl-4',
								activeId === heading.id
									? 'text-foreground font-medium'
									: 'text-muted-foreground hover:text-foreground',
							)}
						>
							{heading.text}
						</a>
					))}
				</nav>
			</div>
		</aside>
	)
}
