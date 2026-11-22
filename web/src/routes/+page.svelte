<script lang="ts">
	import * as Card from '#lib/components/ui/card/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
	import QrScan from '#lib/components/QrScan.svelte';
</script>

<div class="mx-auto max-w-md space-y-4">
	<div class="rounded-md bg-brand-yellow px-4 py-2 text-center text-sm font-semibold text-navy">
		Scan QR hologram di produk, kumpulkan poin!
	</div>

	<Card.Root>
		<Card.Header>
			<Card.Title class="text-navy">Scan QR hologram</Card.Title>
			<Card.Description>
				Arahkan kamera ke QR pada stiker hologram. Kali pertama, kamu akan diminta mengisi nama dan nomor WhatsApp.
			</Card.Description>
		</Card.Header>
		<Card.Content>
			<QrScan />
		</Card.Content>
	</Card.Root>

	<div class="flex items-center gap-3">
		<Button href="/me" variant="outline" class="flex-1">Poin saya</Button>
		<Button href="/rewards" variant="outline" class="flex-1">Lihat reward</Button>
	</div>
</div>
