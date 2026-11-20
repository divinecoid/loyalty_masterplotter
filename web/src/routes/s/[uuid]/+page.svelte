<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { api, ApiError } from '#lib/api.js';
	import * as Card from '#lib/components/ui/card/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Input } from '#lib/components/ui/input/index.js';
	import { Label } from '#lib/components/ui/label/index.js';

	type Info = { state: 'ok' | 'claimed' | 'invalid'; known: boolean; name?: string; product?: string; points?: number; mine?: boolean };
	type Done = { earned: number; balance: number; name: string; product: string; new_customer: boolean };

	const uuid = $derived(page.params.uuid ?? '');
	let info = $state<Info | null>(null);
	let done = $state<Done | null>(null);
	let error = $state('');
	let loading = $state(false);
	let form = $state({ name: '', phone: '', consent: false });

	async function load() {
		try {
			info = await api<Info>(`/scan/${encodeURIComponent(uuid)}`);
		} catch {
			error = 'Tidak bisa terhubung. Coba lagi.';
		}
	}
	onMount(load);

	async function claim(e?: SubmitEvent) {
		e?.preventDefault();
		error = '';
		loading = true;
		try {
			const body = info?.known ? undefined : { name: form.name, phone: form.phone, consent: form.consent };
			done = await api<Done>(`/scan/${encodeURIComponent(uuid)}/claim`, { method: 'POST', json: body ?? {} });
		} catch (err) {
			if (err instanceof ApiError && err.status === 409) await load(); // sudah diklaim -> tampilkan status terbaru
			else if (err instanceof ApiError && err.status === 404) await load();
			else error = err instanceof Error ? err.message : 'Gagal';
		} finally {
			loading = false;
		}
	}
</script>

<div class="mx-auto max-w-md space-y-4">
	{#if done}
		<Card.Root class="border-brand-yellow">
			<Card.Header class="text-center">
				<Card.Description>{done.new_customer ? 'Selamat datang' : 'Halo lagi'}, {done.name}!</Card.Description>
				<Card.Title class="text-5xl font-bold text-navy">+{done.earned}</Card.Title>
				<Card.Description>poin dari {done.product}</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-4 text-center">
				<p class="rounded-md bg-brand-yellow px-4 py-2 font-semibold text-navy">Total poin kamu: {done.balance}</p>
				<Button href="/me" class="w-full">Lihat poin saya</Button>
			</Card.Content>
		</Card.Root>
	{:else if !info}
		<p class="py-10 text-center text-muted-foreground" aria-live="polite">{error || 'Memeriksa kode…'}</p>
	{:else if info.state === 'invalid'}
		<Card.Root>
			<Card.Header>
				<Card.Title class="text-navy">Kode tidak valid</Card.Title>
				<Card.Description>
					Kode ini tidak dikenali atau belum diaktifkan. Pastikan kamu memindai stiker hologram asli pada produk Master Plotter.
				</Card.Description>
			</Card.Header>
		</Card.Root>
	{:else if info.state === 'claimed'}
		<Card.Root>
			<Card.Header>
				<Card.Title class="text-navy">{info.mine ? 'Poin sudah kamu klaim' : 'Kode sudah digunakan'}</Card.Title>
				<Card.Description>
					{info.mine
						? 'Kode ini sudah ditambahkan ke poin kamu.'
						: 'Kode ini sudah diklaim. Jika kamu merasa ini keliru, hubungi Master Plotter.'}
				</Card.Description>
			</Card.Header>
			{#if info.known}<Card.Content><Button href="/me" variant="outline" class="w-full">Lihat poin saya</Button></Card.Content>{/if}
		</Card.Root>
	{:else}
		<Card.Root>
			<Card.Header>
				<Card.Description>{info.product}</Card.Description>
				<Card.Title class="text-3xl text-navy">Dapat {info.points} poin</Card.Title>
				{#if info.known}<Card.Description>Halo lagi, {info.name}!</Card.Description>{/if}
			</Card.Header>
			<Card.Content>
				<form onsubmit={claim} class="space-y-4">
					{#if !info.known}
						<div class="space-y-2">
							<Label for="name">Nama</Label>
							<Input id="name" autocomplete="name" required minlength={2} maxlength={60} bind:value={form.name} />
						</div>
						<div class="space-y-2">
							<Label for="phone">Nomor WhatsApp</Label>
							<Input
								id="phone"
								type="tel"
								inputmode="tel"
								autocomplete="tel"
								placeholder="0812xxxxxxxx"
								required
								bind:value={form.phone}
							/>
						</div>
						<label class="flex items-start gap-2 text-sm text-muted-foreground">
							<input type="checkbox" class="mt-1 size-4 accent-[var(--mp-navy)]" required bind:checked={form.consent} />
							<span>Saya setuju nama dan nomor WhatsApp saya disimpan untuk program loyalty Master Plotter.</span>
						</label>
					{/if}
					{#if error}<p class="text-sm text-destructive" role="alert">{error}</p>{/if}
					<Button type="submit" size="lg" class="w-full" disabled={loading}>
						{loading ? 'Memproses…' : `Klaim ${info.points} poin`}
					</Button>
				</form>
			</Card.Content>
		</Card.Root>
	{/if}
</div>
