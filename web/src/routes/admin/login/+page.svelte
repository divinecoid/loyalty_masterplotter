<script lang="ts">
	import { goto } from '$app/navigation';
	import { api } from '#lib/api.js';
	import * as Card from '#lib/components/ui/card/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Input } from '#lib/components/ui/input/index.js';
	import { Label } from '#lib/components/ui/label/index.js';

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		loading = true;
		try {
			await api('/admin/auth/login', { method: 'POST', json: { email, password } });
			await goto('/admin/batches');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Gagal login';
		} finally {
			loading = false;
		}
	}
</script>

<Card.Root class="mx-auto mt-10 max-w-sm">
	<Card.Header>
		<Card.Title class="text-navy">Masuk Admin</Card.Title>
		<Card.Description>Kelola produk, batch QR, dan stiker hologram.</Card.Description>
	</Card.Header>
	<Card.Content>
		<form onsubmit={submit} class="space-y-4">
			<div class="space-y-2">
				<Label for="email">Email</Label>
				<Input id="email" type="email" autocomplete="username" required bind:value={email} />
			</div>
			<div class="space-y-2">
				<Label for="password">Password</Label>
				<Input id="password" type="password" autocomplete="current-password" required bind:value={password} />
			</div>
			{#if error}<p class="text-sm text-destructive" role="alert">{error}</p>{/if}
			<Button type="submit" class="w-full" disabled={loading}>{loading ? 'Memproses…' : 'Masuk'}</Button>
		</form>
	</Card.Content>
</Card.Root>
