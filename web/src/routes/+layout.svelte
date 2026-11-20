<script lang="ts">
	import '../app.css';
	import favicon from '#lib/assets/favicon.svg';
	import { page } from '$app/state';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Master Plotter Loyalty</title>
	<meta name="theme-color" content="#112f60" />
</svelte:head>

<header class="border-b-4 border-brand-yellow bg-navy text-white">
	<div class="mx-auto flex h-14 max-w-5xl items-center px-4">
		<span class="text-lg font-bold tracking-tight">MASTER <span class="text-brand-yellow">PLOTTER</span></span>
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
