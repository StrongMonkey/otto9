<script lang="ts">
	import { run } from 'svelte/legacy';

	import { profile } from '$lib/stores';

	let initials = $state('08');

	run(() => {
		if ($profile.email) {
			const parts = $profile.email.split('@')[0].split(/[.-]/);
			initials = parts[0].charAt(0).toUpperCase();
			if (parts.length > 1) {
				initials += parts[parts.length - 1].charAt(0).toUpperCase();
			}
		}
	});
</script>

{#if $profile.iconURL}
	<img class="h-8 w-8 rounded-full" src={$profile.iconURL} alt="profile" />
{:else}
	<div class="flex h-8 w-8 items-center justify-center rounded-full bg-gray-600 text-white">
		{initials}
	</div>
{/if}
