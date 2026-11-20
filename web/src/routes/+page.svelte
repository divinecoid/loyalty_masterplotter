<script lang="ts">
	import * as Card from '#lib/components/ui/card/index.js';
	import { Button } from '#lib/components/ui/button/index.js';
</script>

<div class="space-y-4">
	<div class="rounded-md bg-brand-yellow px-4 py-2 text-center text-sm font-semibold text-navy">
		Scan QR hologram di produk, kumpulkan poin!
	</div>

	<Card.Root>
		<Card.Header>
			<Card.Title class="text-navy">Selamat datang</Card.Title>
			<Card.Description>Scan QR pada stiker hologram untuk mulai mengumpulkan poin.</Card.Description>
		</Card.Header>
		<Card.Content class="flex items-center gap-3">
			<Button href="/me">Poin saya</Button>
			<Button href="/rewards" variant="outline">Lihat reward</Button>
		</Card.Content>
	</Card.Root>
</div>
