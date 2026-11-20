<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type AdminCustomer } from '#lib/api.js';
	import * as Card from '#lib/components/ui/card/index.js';
	import * as Table from '#lib/components/ui/table/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Input } from '#lib/components/ui/input/index.js';
	import { Badge } from '#lib/components/ui/badge/index.js';

	let items = $state<AdminCustomer[]>([]);
	let q = $state('');
	let error = $state('');

	async function load() {
		items = await api<AdminCustomer[]>('/admin/customers?' + new URLSearchParams({ q: q.trim() }));
	}
	onMount(() => load().catch((e) => (error = e.message)));

	async function run(fn: () => Promise<unknown>) {
		error = '';
		try {
			await fn();
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Gagal';
		}
	}

	const verify = (c: AdminCustomer) =>
		run(() => api(`/admin/customers/${c.id}/verify`, { method: 'POST', json: { verified: !c.phone_verified } }));

	function adjust(c: AdminCustomer) {
		const v = prompt(`Koreksi poin untuk ${c.name} (saldo ${c.points}). Isi angka, mis. 10 atau -5:`);
		if (v === null) return;
		const delta = Number(v);
		if (!Number.isInteger(delta) || delta === 0) {
			error = 'Masukkan bilangan bulat selain 0';
			return;
		}
		return run(() => api(`/admin/customers/${c.id}/adjust`, { method: 'POST', json: { delta } }));
	}
</script>

<div class="space-y-4">
	<form
		class="flex gap-2"
		onsubmit={(e) => {
			e.preventDefault();
			load().catch((err) => (error = err.message));
		}}
	>
		<Input class="max-w-xs" placeholder="Cari nama / nomor" bind:value={q} />
		<Button type="submit" variant="outline">Cari</Button>
	</form>

	{#if error}<p class="text-sm text-destructive" role="alert">{error}</p>{/if}

	<Card.Root>
		<Card.Content class="pt-6">
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>Nama</Table.Head>
						<Table.Head>WhatsApp</Table.Head>
						<Table.Head>Poin</Table.Head>
						<Table.Head>Verifikasi</Table.Head>
						<Table.Head class="text-right">Aksi</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each items as c (c.id)}
						<Table.Row>
							<Table.Cell>{c.name}</Table.Cell>
							<Table.Cell class="font-mono text-xs">{c.phone}</Table.Cell>
							<Table.Cell class="font-semibold text-navy">{c.points}</Table.Cell>
							<Table.Cell><Badge variant={c.phone_verified ? 'default' : 'secondary'}>{c.phone_verified ? 'Terverifikasi' : 'Belum'}</Badge></Table.Cell>
							<Table.Cell class="space-x-2 text-right whitespace-nowrap">
								<Button size="sm" variant="outline" onclick={() => verify(c)}>{c.phone_verified ? 'Cabut verifikasi' : 'Verifikasi'}</Button>
								<Button size="sm" variant="outline" onclick={() => adjust(c)}>Koreksi poin</Button>
							</Table.Cell>
						</Table.Row>
					{:else}
						<Table.Row><Table.Cell colspan={5} class="text-center text-muted-foreground">Tidak ada pelanggan.</Table.Cell></Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</Card.Content>
	</Card.Root>
</div>
