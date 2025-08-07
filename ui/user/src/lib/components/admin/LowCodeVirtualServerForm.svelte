<script lang="ts">
	import { LoaderCircle } from 'lucide-svelte';
	import { AdminService, ChatService } from '$lib/services';
	import type { MCPCatalogEntryServerManifest, MCPCatalogEntry } from '$lib/services/admin/types';
	import SelectMcpAccessControlRules from './SelectMcpAccessControlRules.svelte';
	import VirtualServerConfiguration from './VirtualServerConfiguration.svelte';

	interface Props {
		catalogId?: string;
		onCancel?: () => void;
		onSubmit?: (id: string, type: 'virtual') => void;
	}

	let { catalogId, onCancel, onSubmit }: Props = $props();

	let saving = $state(false);
	let serverName = $state('');
	let serverDescription = $state('');
	let serverIcon = $state('');
	let serverId = $state<string>();
	let selectRulesDialog = $state<ReturnType<typeof SelectMcpAccessControlRules>>();
	let savedEntry = $state<MCPCatalogEntry>();
	let savedProject = $state<{ assistantID: string; id: string; name: string }>();
	let virtualServerConfig = $state<ReturnType<typeof VirtualServerConfiguration>>();

	async function handleSubmit() {
		if (!catalogId || !serverName.trim()) return;

		saving = true;
		try {
			// First, create the project
			const assistants = (await ChatService.listAssistants()).items;
			let defaultAssistant = assistants.find((a) => a.default);
			if (!defaultAssistant && assistants.length == 1) {
				defaultAssistant = assistants[0];
			}
			if (!defaultAssistant) {
				throw new Error('Failed to find default assistant');
			}

			// Create a project with the default assistant
			const project = await ChatService.createProject(defaultAssistant.id, {
				name: serverName,
				description: serverDescription
			});

			// Store the project information
			savedProject = {
				assistantID: project.assistantID,
				id: project.id,
				name: project.name
			};

			// Create the MCP catalog entry
			const manifest: MCPCatalogEntryServerManifest = {
				name: serverName,
				description: serverDescription,
				icon: serverIcon,
				runtime: 'virtual',
				env: [],
				metadata: {
					categories: ''
				},
				remoteConfig: {
					fixedURL: `${window.location.protocol}//${window.location.hostname}/virtual/mcp/${project.id}`
				},
				projectID: project.id
			};

			// Create the MCP catalog entry
			const result = await AdminService.createMCPCatalogEntry(catalogId, manifest);
			serverId = result.id;
			savedEntry = result;

			// Save tools as tasks in the project
			const config = virtualServerConfig?.getConfiguration();
			if (config?.tools && config.tools.length > 0) {
				await virtualServerConfig?.saveToolsAsTasks();
			}

			// Open access control dialog
			await selectRulesDialog?.open();
		} catch (error) {
			console.error('Failed to create project and catalog entry:', error);
		} finally {
			saving = false;
		}
	}
</script>

<div class="flex flex-col gap-8">
	<div class="flex flex-col gap-6">
		<!-- Server Name -->
		<div
			class="rounded-lg border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800"
		>
			<div class="flex flex-col gap-2">
				<label for="server-name" class="text-sm font-semibold">Name</label>
				<input
					id="server-name"
					bind:value={serverName}
					class="text-input-filled"
					placeholder="Enter server name"
				/>
			</div>
		</div>

		<!-- Server Description -->
		<div
			class="rounded-lg border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800"
		>
			<div class="flex flex-col gap-2">
				<label for="server-description" class="text-sm font-semibold">Description</label>
				<textarea
					bind:value={serverDescription}
					class="text-input-filled min-h-24 resize-none"
					id="server-description"
					placeholder="Enter server description"
				></textarea>
			</div>
		</div>

		<!-- Server Icon -->
		<div
			class="rounded-lg border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-700"
		>
			<div class="flex flex-col gap-2">
				<label for="server-icon" class="text-sm font-semibold">Icon URL</label>
				<input
					id="server-icon"
					bind:value={serverIcon}
					class="text-input-filled"
					placeholder="Enter icon URL (optional)"
				/>
			</div>
		</div>
	</div>

	<!-- Virtual Server Configuration -->
	<VirtualServerConfiguration bind:this={virtualServerConfig} projectId={savedProject?.id} />

	<!-- Action Buttons -->
	<div class="flex justify-end gap-3">
		<button onclick={onCancel} class="button" disabled={saving}> Cancel </button>
		<button onclick={handleSubmit} class="button-primary" disabled={saving || !serverName.trim()}>
			{#if saving}
				<LoaderCircle class="size-4 animate-spin" />
				Creating...
			{:else}
				Create Virtual MCP Server
			{/if}
		</button>
	</div>
</div>

<SelectMcpAccessControlRules
	bind:this={selectRulesDialog}
	entry={savedEntry}
	onSubmit={() => {
		if (savedEntry) {
			onSubmit?.(serverId!, 'virtual');
		}
	}}
/>
