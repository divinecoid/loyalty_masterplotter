<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

<svelte:head>
	<link rel="icon" type="image/png" href="/icon.png" />
	<link rel="apple-touch-icon" href="/icon.png" />
	<title>Master Plotter Loyalty</title>
	<meta name="theme-color" content="#112f60" />
</svelte:head>

<header class="border-b-4 border-brand-yellow bg-navy text-white">
	<div class="mx-auto flex h-14 max-w-5xl items-center px-4">
		<a href="/" class="flex items-center"><img src="/logo.png" alt="Master Plotter" width="623" height="194" class="h-9 w-auto" /></a>
		<span class="ml-3 text-sm text-mist">Loyalty</span>
		{#if !page.url.pathname.startsWith('/admin')}
			<nav class="ml-auto flex gap-1 text-sm">
				{#each [{ href: '/me', label: 'Poin' }, { href: '/rewards', label: 'Reward' }] as n}
					<a
						href={n.href}
						class="rounded-md px-3 py-1.5 hover:bg-white/10 {page.url.pathname.startsWith(n.href) ? 'bg-white/15 font-semibold' : 'text-mist'}"
						>{n.label}</a
					>
				{/each}
			</nav>
		{/if}
	</div>
</header>

<main class="mx-auto max-w-5xl px-4 py-6">
	{@render children()}
</main>
