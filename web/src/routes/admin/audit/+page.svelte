<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type AuditEntry } from '#lib/api.js';
	import * as Card from '#lib/components/ui/card/index.js';
	import * as Table from '#lib/components/ui/table/index.js';

	let items = $state<AuditEntry[]>([]);
	let error = $state('');
	onMount(() => api<AuditEntry[]>('/admin/audit?limit=100').then((v) => (items = v)).catch((e) => (error = e.message)));
	const fmt = (s: string) => new Date(s).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' });
</script>

{#if error}<p class="text-sm text-destructive" role="alert">{error}</p>{/if}
<Card.Root>
	<Card.Content class="pt-6">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>Waktu</Table.Head>
					<Table.Head>Admin</Table.Head>
					<Table.Head>Aksi</Table.Head>
					<Table.Head>Detail</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each items as a (a.id)}
					<Table.Row>
						<Table.Cell class="text-xs whitespace-nowrap">{fmt(a.created_at)}</Table.Cell>
						<Table.Cell class="text-xs">{a.admin ?? '-'}</Table.Cell>
						<Table.Cell class="font-mono text-xs">{a.action}</Table.Cell>
						<Table.Cell class="max-w-xs truncate font-mono text-xs text-muted-foreground" title={a.payload}>{a.payload}</Table.Cell>
					</Table.Row>
				{:else}
					<Table.Row><Table.Cell colspan={4} class="text-center text-muted-foreground">Belum ada aktivitas.</Table.Cell></Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</Card.Content>
</Card.Root>
