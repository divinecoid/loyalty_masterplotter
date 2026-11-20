<script lang="ts">
	import { api } from '#lib/api.js';
	import * as Card from '#lib/components/ui/card/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Input } from '#lib/components/ui/input/index.js';
	import type { Me } from '#lib/api.js';

	let { me, onverified }: { me: Me; onverified: () => void } = $props();

	let sent = $state(false);
	let code = $state('');
	let busy = $state(false);
	let error = $state('');
	let wait = $state(0);
	let timer: ReturnType<typeof setInterval> | undefined;

	function startWait(s: number) {
		wait = s;
		clearInterval(timer);
		timer = setInterval(() => {
			wait -= 1;
			if (wait <= 0) clearInterval(timer);
		}, 1000);
	}

	async function request() {
		error = '';
		busy = true;
		try {
			await api('/me/verify/request', { method: 'POST' });
			sent = true;
			code = '';
			startWait(60);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Gagal mengirim kode';
		} finally {
			busy = false;
		}
	}

	async function confirm(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		busy = true;
		try {
			await api('/me/verify/confirm', { method: 'POST', json: { code: code.trim() } });
			onverified();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Gagal memverifikasi';
		} finally {
			busy = false;
		}
	}
</script>

<Card.Root class="border-brand-yellow">
	<Card.Header>
		<Card.Title class="text-navy">Verifikasi nomor WhatsApp</Card.Title>
		<Card.Description>
			Untuk menukar reward, nomor {me.phone} perlu diverifikasi supaya poinmu aman.
		</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-3">
		{#if !me.otp_available}
			<p class="text-sm text-muted-foreground">Verifikasi otomatis belum tersedia. Hubungi tim Master Plotter untuk memverifikasi nomormu.</p>
		{:else if !sent}
			<Button class="w-full" disabled={busy} onclick={request}>{busy ? 'Mengirim…' : 'Kirim kode ke WhatsApp'}</Button>
		{:else}
			<form onsubmit={confirm} class="space-y-3">
				<Input
					inputmode="numeric"
					autocomplete="one-time-code"
					maxlength={6}
					placeholder="6 digit kode"
					class="text-center font-mono text-xl tracking-widest"
					bind:value={code}
				/>
				<Button type="submit" class="w-full" disabled={busy || code.trim().length !== 6}>Verifikasi</Button>
				<Button type="button" variant="ghost" class="w-full" disabled={busy || wait > 0} onclick={request}>
					{wait > 0 ? `Kirim ulang dalam ${wait} dtk` : 'Kirim ulang kode'}
				</Button>
			</form>
		{/if}
		{#if error}<p class="text-sm text-destructive" role="alert">{error}</p>{/if}
	</Card.Content>
</Card.Root>
