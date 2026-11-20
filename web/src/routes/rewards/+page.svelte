<script lang="ts">
	import { onMount } from 'svelte';
	import { api, ApiError } from '#lib/api.js';
	import * as Card from '#lib/components/ui/card/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Badge } from '#lib/components/ui/badge/index.js';
	import VerifyPhone from '#lib/components/VerifyPhone.svelte';
	import type { Me, Reward, Redemption } from '#lib/api.js';

	let me = $state<Me | null>(null);
	let rewards = $state<Reward[]>([]);
	let loaded = $state(false);
	let busyId = $state<number | null>(null);
	let error = $state('');
	let result = $state<{ redemption: Redemption; balance: number } | null>(null);

	async function load() {
		rewards = await api<Reward[]>('/rewards');
		try {
			me = await api<Me>('/me');
		} catch (e) {
			if (!(e instanceof ApiError && e.status === 401)) throw e;
			me = null;
		}
	}
	onMount(async () => {
		try {
			await load();
		} catch {
			error = 'Gagal memuat reward.';
		} finally {
			loaded = true;
		}
	});

	async function redeem(r: Reward) {
		if (!confirm(`Tukar ${r.cost_points} poin dengan "${r.name}"?`)) return;
		error = '';
		busyId = r.id;
		try {
			result = await api(`/rewards/${r.id}/redeem`, { method: 'POST' });
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Gagal menukar';
		} finally {
			busyId = null;
		}
	}

	function hint(r: Reward): string | null {
		if (!me) return 'Scan QR produk dulu untuk mulai mengumpulkan poin';
		if (r.stock <= 0) return 'Stok habis';
		if (me.points < r.cost_points) return `Kurang ${r.cost_points - me.points} poin`;
		if (!me.phone_verified) return 'Verifikasi nomor WhatsApp dulu (lihat di atas)';
		return null;
	}
</script>

<div class="mx-auto max-w-md space-y-4">
	{#if me}
		<div class="flex items-center justify-between rounded-md bg-navy px-4 py-3 text-white">
			<span class="text-sm text-mist">Poin kamu</span>
			<span class="text-2xl font-bold text-brand-yellow">{me.points}</span>
		</div>
	{/if}

	{#if me && !me.phone_verified}
		<VerifyPhone {me} onverified={load} />
	{/if}

	{#if result}
		<Card.Root class="border-brand-yellow">
			<Card.Header class="text-center">
				<Card.Description>Penukaran berhasil</Card.Description>
				<Card.Title class="text-navy">{result.redemption.reward}</Card.Title>
			</Card.Header>
			<Card.Content class="space-y-3 text-center">
				<p class="rounded-md bg-brand-yellow px-4 py-3 font-mono text-2xl font-bold tracking-wider text-navy">
					{result.redemption.code}
				</p>
				<p class="text-sm text-muted-foreground">
					Tunjukkan kode ini ke tim Master Plotter untuk mengambil reward. Sisa poin: {result.balance}.
				</p>
				<Button variant="outline" class="w-full" onclick={() => (result = null)}>Tutup</Button>
			</Card.Content>
		</Card.Root>
	{/if}

	{#if error}<p class="text-sm text-destructive" role="alert">{error}</p>{/if}

	{#if !loaded}
		<p class="py-10 text-center text-muted-foreground">Memuat…</p>
	{:else}
		{#each rewards as r (r.id)}
			{@const h = hint(r)}
			<Card.Root>
				<Card.Header>
					<div class="flex items-start justify-between gap-3">
						<Card.Title class="text-navy">{r.name}</Card.Title>
						<Badge>{r.cost_points} poin</Badge>
					</div>
					<Card.Description>Stok: {r.stock}</Card.Description>
				</Card.Header>
				<Card.Content class="space-y-2">
					<Button class="w-full" disabled={h !== null || busyId !== null} onclick={() => redeem(r)}>
						{busyId === r.id ? 'Memproses…' : 'Tukar'}
					</Button>
					{#if h}<p class="text-center text-xs text-muted-foreground">{h}</p>{/if}
				</Card.Content>
			</Card.Root>
		{:else}
			<p class="py-10 text-center text-muted-foreground">Belum ada reward.</p>
		{/each}
	{/if}
</div>
