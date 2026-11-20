import adapter from '@sveltejs/adapter-static';
import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			// SPA: semua route di-fallback ke index.html (UUID dinamis di /s/[uuid])
			adapter: adapter({ fallback: 'index.html' })
		})
	],
	server: {
		proxy: { '/api': 'http://localhost:8080' }
	}
});
