<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type AdminRedemption } from '#lib/api.js';
	import * as Card from '#lib/components/ui/card/index.js';
	import * as Table from '#lib/components/ui/table/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Input } from '#lib/components/ui/input/index.js';
	import { Badge } from '#lib/components/ui/badge/index.js';

	let items = $state<AdminRedemption[]>([]);
	let status = $state('pending');
	let q = $state('');
	let error = $state('');

	const label = { pending: 'Menunggu', fulfilled: 'Sudah diambil', cancelled: 'Dibatalkan' } as const;
	const fmt = (s: string) => new Date(s).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' });

	async function load() {
		const p = new URLSearchParams({ status, q: q.trim() });
		items = await api<AdminRedemption[]>('/admin/redemptions?' + p);
	}
	onMount(() => load().catch((e) => (error = e.message)));

	async function act(r: AdminRedemption, what: 'fulfill' | 'cancel') {
		const msg =
			what === 'fulfill'
				? `Tandai "${r.reward}" untuk ${r.customer} sudah diserahkan?`
				: `Batalkan penukaran ini? ${r.cost} poin dikembalikan ke ${r.customer} dan stok bertambah 1.`;
		if (!confirm(msg)) return;
		error = '';
		try {
			await api(`/admin/redemptions/${r.id}/${what}`, { method: 'POST' });
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Gagal';
			await load().catch(() => {});
		}
	}
</script>

<div class="space-y-4">
	<form
		class="flex flex-wrap items-center gap-2"
		onsubmit={(e) => {
			e.preventDefault();
			load().catch((err) => (error = err.message));
		}}
	>
		<select bind:value={status} onchange={() => load()} class="h-9 rounded-md border bg-background px-3 text-sm">
			<option value="pending">Menunggu</option>
			<option value="fulfilled">Sudah diambil</option>
			<option value="cancelled">Dibatalkan</option>
			<option value="">Semua</option>
		</select>
		<Input class="max-w-xs" placeholder="Cari kode / nama / nomor" bind:value={q} />
		<Button type="submit" variant="outline">Cari</Button>
	</form>

	{#if error}<p class="text-sm text-destructive" role="alert">{error}</p>{/if}

	<Card.Root>
		<Card.Content class="pt-6">
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>Kode</Table.Head>
						<Table.Head>Reward</Table.Head>
						<Table.Head>Pelanggan</Table.Head>
						<Table.Head>Tanggal</Table.Head>
						<Table.Head>Status</Table.Head>
						<Table.Head class="text-right">Aksi</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each items as r (r.id)}
						<Table.Row>
							<Table.Cell class="font-mono font-semibold text-navy">{r.code}</Table.Cell>
							<Table.Cell>{r.reward}<span class="block text-xs text-muted-foreground">{r.cost} poin</span></Table.Cell>
							<Table.Cell>{r.customer}<span class="block text-xs text-muted-foreground">{r.phone}</span></Table.Cell>
							<Table.Cell class="text-xs">{fmt(r.created_at)}</Table.Cell>
							<Table.Cell><Badge variant={r.status === 'pending' ? 'default' : 'secondary'}>{label[r.status]}</Badge></Table.Cell>
							<Table.Cell class="space-x-2 text-right whitespace-nowrap">
								{#if r.status === 'pending'}
									<Button size="sm" onclick={() => act(r, 'fulfill')}>Serahkan</Button>
									<Button size="sm" variant="outline" onclick={() => act(r, 'cancel')}>Batalkan</Button>
								{/if}
							</Table.Cell>
						</Table.Row>
					{:else}
						<Table.Row><Table.Cell colspan={6} class="text-center text-muted-foreground">Tidak ada data.</Table.Cell></Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</Card.Content>
	</Card.Root>
</div>
