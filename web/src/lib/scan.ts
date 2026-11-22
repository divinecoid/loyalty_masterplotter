const UUID = '[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}';
const BARE = new RegExp(`^${UUID}$`, 'i');
const IN_URL = new RegExp('^https?://[^/\\s]+/s/(' + UUID + ')/?(?:[?#][^\\s]*)?$', 'i');

/**
 * Ambil UUID kode dari hasil scan. Hanya menerima UUID polos atau URL berbentuk `http(s)://host/s/<uuid>`.
 * Teks lain (URL asing, teks bebas) ditolak: aplikasi tidak pernah membuka alamat dari QR, hanya rute internal /s/<uuid>.
 */
export function parseScan(text: string): string | null {
	const t = text.trim();
	if (BARE.test(t)) return t.toLowerCase();
	const m = IN_URL.exec(t);
	return m ? m[1].toLowerCase() : null;
}
