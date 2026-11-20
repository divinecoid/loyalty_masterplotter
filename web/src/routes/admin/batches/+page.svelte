<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Batch, type Product } from '#lib/api.js';
	import * as Card from '#lib/components/ui/card/index.js';
	import * as Table from '#lib/components/ui/table/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Input } from '#lib/components/ui/input/index.js';
	import { Label } from '#lib/components/ui/label/index.js';
	import { Badge } from '#lib/components/ui/badge/index.js';

	let batches = $state<Batch[]>([]);
	let products = $state<Product[]>([]);
	let error = $state('');
	let busy = $state(false);
	let form = $state({ product_id: 0, qty: 100, label: '' });
	let cell = $state(30);

	const statusLabel = { generated: 'Baru dibuat', printed: 'Tercetak', active: 'Aktif' } as const;

	async function load() {
		[batches, products] = await Promise.all([api<Batch[]>('/admin/batches'), api<Product[]>('/admin/products')]);
		if (!form.product_id) form.product_id = products.find((p) => p.active)?.id ?? 0;
	}
	onMount(() => load().catch((e) => (error = e.message)));

	async function run(fn: () => Promise<unknown>) {
		error = '';
		busy = true;
		try {
			await fn();
			await load();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Gagal';
		} finally {
			busy = false;
		}
	}

	const create = (e: SubmitEvent) => {
		e.preventDefault();
		return run(() =>
			api('/admin/batches', {
				method: 'POST',
				json: { product_id: Number(form.product_id), qty: Number(form.qty), label: form.label }
			})
		);
	};
	const markPrinted = (b: Batch) => run(() => api(`/admin/batches/${b.id}/printed`, { method: 'POST' }));
	const activate = (b: Batch) => {
		if (!confirm(`Aktifkan batch #${b.id}? ${b.qty} kode akan bisa diklaim pembeli.`)) return;
		return run(() => api(`/admin/batches/${b.id}/activate`, { method: 'POST' }));
	};
</script>

<div class="space-y-6">
	<Card.Root>
		<Card.Header>
			<Card.Title class="text-navy">Buat batch kode QR</Card.Title>
			<Card.Description>
				Tiap kode berupa UUID acak. Batch baru belum bisa diklaim sampai dicetak lalu diaktifkan.
			</Card.Description>
		</Card.Header>
		<Card.Content>
			<form onsubmit={create} class="grid gap-4 sm:grid-cols-[2fr_8rem_2fr_auto] sm:items-end">
				<div class="space-y-2">
					<Label for="product">Produk</Label>
					<select
						id="product"
						required
						bind:value={form.product_id}
						class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
					>
						{#each products.filter((p) => p.active) as p (p.id)}
							<option value={p.id}>{p.name} ({p.points_per_unit} poin)</option>
						{/each}
					</select>
				</div>
				<div class="space-y-2">
					<Label for="qty">Jumlah</Label>
					<Input id="qty" type="number" min="1" max="10000" required bind:value={form.qty} />
				</div>
				<div class="space-y-2">
					<Label for="label">Catatan (opsional)</Label>
					<Input id="label" maxlength={100} bind:value={form.label} />
				</div>
				<Button type="submit" disabled={busy || !form.product_id}>Generate</Button>
			</form>
		</Card.Content>
	</Card.Root>

	{#if error}<p class="text-sm text-destructive" role="alert">{error}</p>{/if}

	<Card.Root>
		<Card.Header class="flex-row items-center justify-between">
			<Card.Title class="text-navy">Daftar batch</Card.Title>
			<div class="flex items-center gap-2 text-sm">
				<Label for="cell">Ukuran sel stiker (mm)</Label>
				<Input id="cell" type="number" min="15" max="90" class="h-8 w-20" bind:value={cell} />
			</div>
		</Card.Header>
		<Card.Content>
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>#</Table.Head>
						<Table.Head>Produk</Table.Head>
						<Table.Head>Jumlah</Table.Head>
						<Table.Head>Status</Table.Head>
						<Table.Head>Terklaim</Table.Head>
						<Table.Head class="text-right">Aksi</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each batches as b (b.id)}
						<Table.Row>
							<Table.Cell class="font-mono">B{b.id}</Table.Cell>
							<Table.Cell>
								{b.product_name}
								{#if b.label}<span class="block text-xs text-muted-foreground">{b.label}</span>{/if}
							</Table.Cell>
							<Table.Cell>{b.qty}</Table.Cell>
							<Table.Cell>
								<Badge variant={b.status === 'active' ? 'default' : 'secondary'}>{statusLabel[b.status]}</Badge>
							</Table.Cell>
							<Table.Cell>{b.counts.claimed ?? 0}</Table.Cell>
							<Table.Cell class="space-x-2 text-right whitespace-nowrap">
								<Button
									variant="outline"
									size="sm"
									href={`/api/v1/admin/batches/${b.id}/sheet.pdf?cell=${cell}`}
									target="_blank"
								>
									PDF stiker
								</Button>
								{#if b.status === 'generated'}
									<Button size="sm" disabled={busy} onclick={() => markPrinted(b)}>Tandai tercetak</Button>
								{:else if b.status === 'printed'}
									<Button size="sm" disabled={busy} onclick={() => activate(b)}>Aktifkan</Button>
								{/if}
							</Table.Cell>
						</Table.Row>
					{:else}
						<Table.Row><Table.Cell colspan={6} class="text-center text-muted-foreground">Belum ada batch.</Table.Cell></Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</Card.Content>
	</Card.Root>
</div>
