<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type AdminReward } from '#lib/api.js';
	import * as Card from '#lib/components/ui/card/index.js';
	import * as Table from '#lib/components/ui/table/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Input } from '#lib/components/ui/input/index.js';
	import { Label } from '#lib/components/ui/label/index.js';
	import { Badge } from '#lib/components/ui/badge/index.js';

	let rewards = $state<AdminReward[]>([]);
	let error = $state('');
	let form = $state({ name: '', cost_points: 50, stock: 10 });

	async function load() {
		rewards = await api<AdminReward[]>('/admin/rewards');
	}
	onMount(() => load().catch((e) => (error = e.message)));

	async function create(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		try {
			await api('/admin/rewards', {
				method: 'POST',
				json: { name: form.name, cost_points: Number(form.cost_points), stock: Number(form.stock) }
			});
			form = { name: '', cost_points: 50, stock: 10 };
			await load();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Gagal';
		}
	}

	async function patch(r: AdminReward, body: Record<string, unknown>) {
		error = '';
		try {
			await api(`/admin/rewards/${r.id}`, { method: 'PATCH', json: body });
			await load();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Gagal';
		}
	}
</script>

<div class="space-y-6">
	<Card.Root>
		<Card.Header>
			<Card.Title class="text-navy">Tambah reward</Card.Title>
			<Card.Description>Reward nonaktif tidak tampil di katalog pembeli.</Card.Description>
		</Card.Header>
		<Card.Content>
			<form onsubmit={create} class="grid gap-4 sm:grid-cols-[2fr_8rem_8rem_auto] sm:items-end">
				<div class="space-y-2"><Label for="rn">Nama</Label><Input id="rn" required bind:value={form.name} /></div>
				<div class="space-y-2"><Label for="rc">Poin</Label><Input id="rc" type="number" min="1" required bind:value={form.cost_points} /></div>
				<div class="space-y-2"><Label for="rs">Stok</Label><Input id="rs" type="number" min="0" required bind:value={form.stock} /></div>
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
						<Table.Head>Nama</Table.Head>
						<Table.Head class="w-32">Poin</Table.Head>
						<Table.Head class="w-32">Stok</Table.Head>
						<Table.Head>Status</Table.Head>
						<Table.Head class="text-right">Aksi</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each rewards as r (r.id)}
						<Table.Row>
							<Table.Cell>{r.name}</Table.Cell>
							<Table.Cell>
								<Input type="number" min="1" class="h-8 w-24" value={r.cost_points}
									onchange={(e) => patch(r, { cost_points: Number(e.currentTarget.value) })} />
							</Table.Cell>
							<Table.Cell>
								<Input type="number" min="0" class="h-8 w-24" value={r.stock}
									onchange={(e) => patch(r, { stock: Number(e.currentTarget.value) })} />
							</Table.Cell>
							<Table.Cell><Badge variant={r.active ? 'default' : 'secondary'}>{r.active ? 'Aktif' : 'Nonaktif'}</Badge></Table.Cell>
							<Table.Cell class="text-right">
								<Button variant="outline" size="sm" onclick={() => patch(r, { active: !r.active })}>
									{r.active ? 'Nonaktifkan' : 'Aktifkan'}
								</Button>
							</Table.Cell>
						</Table.Row>
					{:else}
						<Table.Row><Table.Cell colspan={5} class="text-center text-muted-foreground">Belum ada reward.</Table.Cell></Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</Card.Content>
	</Card.Root>
</div>
