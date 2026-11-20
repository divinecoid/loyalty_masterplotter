<script lang="ts">
	import { onMount } from 'svelte';
	import { api, ApiError } from '#lib/api.js';
	import * as Card from '#lib/components/ui/card/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Badge } from '#lib/components/ui/badge/index.js';
	import VerifyPhone from '#lib/components/VerifyPhone.svelte';
	import type { Me, Redemption } from '#lib/api.js';

	type Entry = { delta: number; reason: string; product: string | null; created_at: string };

	let me = $state<Me | null>(null);
	let ledger = $state<Entry[]>([]);
	let redemptions = $state<Redemption[]>([]);
	let status = $state<'loading' | 'anon' | 'error' | 'ok'>('loading');

	const redStatus = { pending: 'Menunggu diambil', fulfilled: 'Sudah diambil', cancelled: 'Dibatalkan' } as const;

	onMount(async () => {
		try {
			[me, ledger, redemptions] = await Promise.all([
				api<Me>('/me'),
				api<Entry[]>('/me/ledger'),
				api<Redemption[]>('/me/redemptions')
			]);
			status = 'ok';
		} catch (e) {
			status = e instanceof ApiError && e.status === 401 ? 'anon' : 'error';
		}
	});

	const reasonLabel: Record<string, string> = { scan: 'Scan produk', redeem: 'Tukar reward', adjust: 'Penyesuaian', expire: 'Kedaluwarsa' };
	const fmt = (s: string) => new Date(s).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' });
</script>

<div class="mx-auto max-w-md space-y-4">
	{#if status === 'loading'}
		<p class="py-10 text-center text-muted-foreground">Memuat…</p>
	{:else if status === 'anon'}
		<Card.Root>
			<Card.Header>
				<Card.Title class="text-navy">Belum ada poin</Card.Title>
				<Card.Description>Scan QR pada stiker hologram di produk Master Plotter untuk mulai mengumpulkan poin.</Card.Description>
			</Card.Header>
		</Card.Root>
	{:else if status === 'error' || !me}
		<p class="py-10 text-center text-destructive">Gagal memuat data. Coba lagi.</p>
	{:else}
		<Card.Root class="bg-navy text-white">
			<Card.Header>
				<Card.Description class="text-mist">{me.name} · {me.phone}</Card.Description>
				<Card.Title class="text-5xl font-bold text-brand-yellow">{me.points}</Card.Title>
				<Card.Description class="text-mist">poin</Card.Description>
			</Card.Header>
			<Card.Content>
				<Button href="/rewards" class="w-full bg-brand-yellow text-navy hover:bg-brand-yellow/90">Tukar reward</Button>
			</Card.Content>
		</Card.Root>

		{#if !me.phone_verified}
			<VerifyPhone {me} onverified={() => api<Me>('/me').then((m) => (me = m))} />
		{/if}

		{#if redemptions.length}
			<Card.Root>
				<Card.Header><Card.Title class="text-navy">Penukaran reward</Card.Title></Card.Header>
				<Card.Content>
					<ul class="divide-y">
						{#each redemptions as r (r.id)}
							<li class="flex items-center justify-between gap-3 py-2 text-sm">
								<span>
									{r.reward}
									<span class="block text-xs text-muted-foreground">{fmt(r.created_at)} · {r.cost} poin</span>
								</span>
								<span class="text-right">
									<span class="block font-mono font-semibold text-navy">{r.code}</span>
									<Badge variant={r.status === 'pending' ? 'default' : 'secondary'}>{redStatus[r.status]}</Badge>
								</span>
							</li>
						{/each}
					</ul>
				</Card.Content>
			</Card.Root>
		{/if}

		<Card.Root>
			<Card.Header><Card.Title class="text-navy">Riwayat</Card.Title></Card.Header>
			<Card.Content>
				<ul class="divide-y">
					{#each ledger as e}
						<li class="flex items-center justify-between py-2 text-sm">
							<span>
								{e.product ?? reasonLabel[e.reason] ?? e.reason}
								<span class="block text-xs text-muted-foreground">{reasonLabel[e.reason] ?? e.reason} · {fmt(e.created_at)}</span>
							</span>
							<span class="font-semibold {e.delta >= 0 ? 'text-navy' : 'text-destructive'}">{e.delta > 0 ? '+' : ''}{e.delta}</span>
						</li>
					{:else}
						<li class="py-2 text-sm text-muted-foreground">Belum ada aktivitas.</li>
					{/each}
				</ul>
			</Card.Content>
		</Card.Root>
	{/if}
</div>
