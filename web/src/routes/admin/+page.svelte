<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Summary } from '#lib/api.js';
	import * as Card from '#lib/components/ui/card/index.js';

	let s = $state<Summary | null>(null);
	let error = $state('');
	onMount(() => api<Summary>('/admin/summary').then((v) => (s = v)).catch((e) => (error = e.message)));

	const tiles = $derived(
		s
			? [
					{ label: 'Pelanggan', value: s.customers, sub: `${s.verified_customers} terverifikasi`, href: '/admin/customers' },
					{ label: 'QR aktif (belum di-scan)', value: s.codes_active, sub: `${s.codes_claimed} sudah di-scan`, href: '/admin/batches' },
					{ label: 'Poin diberikan', value: s.points_issued, sub: `${s.points_redeemed} sudah ditukar`, href: '/admin/customers' },
					{ label: 'Penukaran menunggu', value: s.redemptions_pending, sub: `${s.redemptions_fulfilled} sudah diambil`, href: '/admin/redemptions' }
				]
			: []
	);
</script>

{#if error}<p class="text-sm text-destructive" role="alert">{error}</p>{/if}
<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
	{#each tiles as t}
		<a href={t.href}>
			<Card.Root class="h-full transition-shadow hover:shadow-md">
				<Card.Header>
					<Card.Description>{t.label}</Card.Description>
					<Card.Title class="text-4xl text-navy">{t.value.toLocaleString('id-ID')}</Card.Title>
				</Card.Header>
				<Card.Content class="text-sm text-muted-foreground">{t.sub}</Card.Content>
			</Card.Root>
		</a>
	{/each}
</div>
