<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { api, type Admin } from '#lib/api.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
	let admin = $state<Admin | null>(null);
	let ready = $state(false);

	const isLogin = $derived(page.url.pathname.startsWith('/admin/login'));

	$effect(() => {
		if (isLogin) {
			ready = true;
			return;
		}
		api<Admin>('/admin/me')
			.then((a) => (admin = a))
			.catch(() => goto('/admin/login'))
			.finally(() => (ready = true));
	});

	async function logout() {
		await api('/admin/auth/logout', { method: 'POST' });
		admin = null;
		goto('/admin/login');
	}

	const nav = [
		{ href: '/admin/products', label: 'Produk' },
		{ href: '/admin/batches', label: 'Batch & Stiker' },
		{ href: '/admin/rewards', label: 'Reward' },
		{ href: '/admin/redemptions', label: 'Penukaran' },
		{ href: '/admin/customers', label: 'Pelanggan' },
		{ href: '/admin/audit', label: 'Audit' }
	];
</script>

{#if ready}
	{#if !isLogin && admin}
		<nav class="mb-6 flex flex-wrap items-center gap-1 border-b pb-3"><a href="/admin" class="rounded-md px-3 py-1.5 text-sm font-medium hover:bg-secondary {page.url.pathname === '/admin' ? 'bg-secondary text-navy' : 'text-muted-foreground'}">Ringkasan</a>
			{#each nav as n}
				<a
					href={n.href}
					class="rounded-md px-3 py-1.5 text-sm font-medium hover:bg-secondary {page.url.pathname.startsWith(n.href)
						? 'bg-secondary text-navy'
						: 'text-muted-foreground'}">{n.label}</a
				>
			{/each}
			<span class="ml-auto text-sm text-muted-foreground">{admin.email}</span>
			<Button variant="outline" size="sm" onclick={logout}>Keluar</Button>
		</nav>
	{/if}
	{#if isLogin || admin}
		{@render children()}
	{/if}
{/if}
