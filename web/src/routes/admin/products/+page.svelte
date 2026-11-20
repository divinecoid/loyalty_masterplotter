<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type Product } from '#lib/api.js';
	import * as Card from '#lib/components/ui/card/index.js';
	import * as Table from '#lib/components/ui/table/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Input } from '#lib/components/ui/input/index.js';
	import { Label } from '#lib/components/ui/label/index.js';
	import { Badge } from '#lib/components/ui/badge/index.js';

	let products = $state<Product[]>([]);
	let error = $state('');
	let form = $state({ sku: '', name: '', points_per_unit: 10 });

	async function load() {
		products = await api<Product[]>('/admin/products');
	}
	onMount(() => load().catch((e) => (error = e.message)));

	async function create(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		try {
			await api('/admin/products', { method: 'POST', json: { ...form, points_per_unit: Number(form.points_per_unit) } });
			form = { sku: '', name: '', points_per_unit: 10 };
			await load();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Gagal';
		}
	}

	async function patch(p: Product, body: Record<string, unknown>) {
		error = '';
		try {
			await api(`/admin/products/${p.id}`, { method: 'PATCH', json: body });
			await load();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Gagal';
		}
	}
</script>

<div class="space-y-6">
	<Card.Root>
		<Card.Header>
			<Card.Title class="text-navy">Tambah produk</Card.Title>
			<Card.Description>Poin per unit di-snapshot ke tiap kode saat batch dibuat.</Card.Description>
		</Card.Header>
		<Card.Content>
			<form onsubmit={create} class="grid gap-4 sm:grid-cols-[1fr_2fr_8rem_auto] sm:items-end">
				<div class="space-y-2"><Label for="sku">SKU</Label><Input id="sku" required bind:value={form.sku} /></div>
				<div class="space-y-2"><Label for="name">Nama</Label><Input id="name" required bind:value={form.name} /></div>
				<div class="space-y-2">
					<Label for="pts">Poin</Label>
					<Input id="pts" type="number" min="0" required bind:value={form.points_per_unit} />
				</div>
				<Button type="submit">Simpan</Button>
			</form>
		</Card.Content>
	</Card.Root>

	{#if error}<p class="text-sm text-destructive" role="alert">{error}</p>{/if}

	<Card.Root>
		<Card.Content class="pt-6">
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>SKU</Table.Head>
						<Table.Head>Nama</Table.Head>
						<Table.Head class="w-32">Poin</Table.Head>
						<Table.Head>Status</Table.Head>
						<Table.Head class="text-right">Aksi</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each products as p (p.id)}
						<Table.Row>
							<Table.Cell class="font-mono text-xs">{p.sku}</Table.Cell>
							<Table.Cell>{p.name}</Table.Cell>
							<Table.Cell>
								<Input
									type="number"
									min="0"
									class="h-8 w-24"
									value={p.points_per_unit}
									onchange={(e) => patch(p, { points_per_unit: Number(e.currentTarget.value) })}
								/>
							</Table.Cell>
							<Table.Cell>
								<Badge variant={p.active ? 'default' : 'secondary'}>{p.active ? 'Aktif' : 'Nonaktif'}</Badge>
							</Table.Cell>
							<Table.Cell class="text-right">
								<Button variant="outline" size="sm" onclick={() => patch(p, { active: !p.active })}>
									{p.active ? 'Nonaktifkan' : 'Aktifkan'}
								</Button>
							</Table.Cell>
						</Table.Row>
					{:else}
						<Table.Row><Table.Cell colspan={5} class="text-center text-muted-foreground">Belum ada produk.</Table.Cell></Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</Card.Content>
	</Card.Root>
</div>
