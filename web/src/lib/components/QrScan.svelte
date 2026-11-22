<script lang="ts">
	import { onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { parseScan } from '#lib/scan.js';
	import { Button } from '#lib/components/ui/button/index.js';

	type Scanner = { start(): Promise<void>; stop(): void; destroy(): void };
	type Phase = 'idle' | 'starting' | 'scanning';

	let video = $state<HTMLVideoElement>();
	let scanner: Scanner | null = null;
	let phase = $state<Phase>('idle');
	let error = $state('');
	let busy = $state(false);

	// getUserMedia hanya tersedia di konteks aman (HTTPS atau localhost).
	const cameraOk = typeof window !== 'undefined' && window.isSecureContext && !!navigator.mediaDevices?.getUserMedia;

	function handle(text: string): boolean {
		const code = parseScan(text);
		if (!code) {
			error = 'QR ini bukan kode stiker hologram Master Plotter.';
			return false;
		}
		stop();
		goto(`/s/${code}`);
		return true;
	}

	async function startCamera() {
		error = '';
		phase = 'starting';
		try {
			const { default: QrScanner } = await import('qr-scanner');
			if (!(await QrScanner.hasCamera())) throw new Error('NoCamera');
			if (!video) throw new Error('NoVideo');
			let last = '';
			scanner = new QrScanner(
				video,
				(r) => {
					if (r.data === last) return; // hindari memproses frame yang sama berulang
					last = r.data;
					handle(r.data);
				},
				{ preferredCamera: 'environment', highlightScanRegion: true, highlightCodeOutline: true, maxScansPerSecond: 8, returnDetailedScanResult: true }
			);
			await scanner.start();
			phase = 'scanning';
		} catch (e) {
			stop();
			const name = e instanceof DOMException ? e.name : e instanceof Error ? e.message : '';
			error =
				name === 'NotAllowedError'
					? 'Izin kamera ditolak. Aktifkan izin kamera di browser, atau ambil foto QR di bawah.'
					: name === 'NoCamera' || name === 'NotFoundError'
						? 'Kamera tidak ditemukan. Gunakan tombol ambil foto di bawah.'
						: 'Kamera tidak bisa dibuka. Gunakan tombol ambil foto di bawah.';
		}
	}

	function stop() {
		scanner?.stop();
		scanner?.destroy();
		scanner = null;
		phase = 'idle';
	}

	async function fromFile(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file) return;
		error = '';
		busy = true;
		try {
			const { default: QrScanner } = await import('qr-scanner');
			const r = await QrScanner.scanImage(file, { returnDetailedScanResult: true });
			handle(r.data);
		} catch {
			error = 'QR tidak terbaca di foto itu. Coba foto lagi lebih dekat dan terang.';
		} finally {
			busy = false;
		}
	}

	onDestroy(() => {
		scanner?.stop();
		scanner?.destroy();
	});
</script>

<div class="space-y-3">
	<div class="relative overflow-hidden rounded-lg bg-navy {phase === 'idle' ? 'hidden' : ''}" style="aspect-ratio: 1 / 1">
		<!-- svelte-ignore a11y_media_has_caption -->
		<video bind:this={video} class="h-full w-full object-cover" playsinline muted></video>
		{#if phase === 'starting'}
			<p class="absolute inset-0 flex items-center justify-center text-sm text-white">Membuka kamera…</p>
		{/if}
	</div>

	{#if phase === 'idle'}
		{#if cameraOk}
			<Button size="lg" class="w-full bg-brand-yellow text-navy hover:bg-brand-yellow/90" onclick={startCamera}>Scan QR dengan kamera</Button>
		{:else}
			<p class="rounded-md bg-secondary px-3 py-2 text-xs text-muted-foreground">
				Kamera langsung hanya tersedia di koneksi aman (HTTPS). Gunakan tombol ambil foto di bawah.
			</p>
		{/if}
	{:else}
		<Button variant="outline" class="w-full" onclick={stop}>Tutup kamera</Button>
	{/if}

	<label class="block">
		<span
			class="inline-flex h-10 w-full cursor-pointer items-center justify-center rounded-md border bg-background text-sm font-medium hover:bg-secondary {busy
				? 'pointer-events-none opacity-60'
				: ''}"
		>
			{busy ? 'Membaca QR…' : 'Ambil foto / pilih gambar QR'}
		</span>
		<input type="file" accept="image/*" capture="environment" class="sr-only" onchange={fromFile} />
	</label>

	{#if error}<p class="text-sm text-destructive" role="alert">{error}</p>{/if}
</div>
