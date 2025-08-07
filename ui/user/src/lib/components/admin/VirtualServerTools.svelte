<script lang="ts">
	import { ChatService, type Project, type Task, type MCPServerTool } from '$lib/services';
	import {
		AlertCircle,
		ChevronDown,
		ChevronUp,
		LoaderCircle,
		Wrench,
		Plus,
		Trash2,
		X
	} from 'lucide-svelte';
	import Toggle from '../Toggle.svelte';
	import { slide } from 'svelte/transition';
	import { responsive } from '$lib/stores';
	import { parseErrorContent } from '$lib/errors';
	import Search from '../Search.svelte';
	import Table from '../Table.svelte';
	import { clickOutside } from '$lib/actions/clickoutside';

	interface ToolParameter {
		id: string;
		name: string;
		description: string;
	}

	interface VirtualTool {
		name: string;
		description: string;
		parameters: ToolParameter[];
		instructions: string;
	}

	interface Props {
		projectId?: string;
		project?: Project;
		readonly?: boolean;
	}

	let { projectId, project, readonly = false }: Props = $props();

	let search = $state('');
	let tools = $state<VirtualTool[]>([]);
	let loading = $state(false);
	let error = $state('');
	let expandedDescriptions = $state<Record<string, boolean>>({});
	let expandedParams = $state<Record<string, boolean>>({});
	let allDescriptionsEnabled = $state(true);
	let allParamsEnabled = $state(false);

	// Tool editor state
	let editingTool = $state<VirtualTool | null>(null);
	let editingToolIndex = $state<number>(-1);
	let toolEditorDialog = $state<HTMLDialogElement>();
	let isCreatingNewTool = $state(false);

	// Convert VirtualTool to MCPServerTool format for display
	function convertVirtualToolToMcpServerTool(tool: VirtualTool): MCPServerTool {
		const params: Record<string, string> = {};
		tool.parameters.forEach((param) => {
			params[param.name] = param.description;
		});

		return {
			id: tool.name, // Use name as ID for virtual tools
			name: tool.name,
			description: tool.description,
			params,
			enabled: true
		};
	}

	let displayTools = $derived(
		tools
			.map((tool) => convertVirtualToolToMcpServerTool(tool))
			.filter(
				(tool) =>
					tool.name.toLowerCase().includes(search.toLowerCase()) ||
					tool.description?.toLowerCase().includes(search.toLowerCase())
			)
	);

	// Load tasks from the project
	async function loadTasks() {
		if (!projectId || !project?.assistantID) return;

		loading = true;
		error = '';

		try {
			const taskList = await ChatService.listTasks(project.assistantID, projectId);
			// Convert tasks to VirtualTool format
			tools = taskList.items.map((task) => convertTaskToVirtualTool(task));
		} catch (err: unknown) {
			console.error(err);
			const { message } = parseErrorContent(err);
			error = message;
		} finally {
			loading = false;
		}
	}

	// Convert task to VirtualTool
	function convertTaskToVirtualTool(task: Task): VirtualTool {
		return {
			name: task.name || `Tool_${task.id}`,
			description: task.description || '',
			parameters: task.onDemand?.params
				? Object.entries(task.onDemand.params).map(([name, description]) => ({
						id: `param_${Date.now()}_${Math.random()}`,
						name,
						description
					}))
				: [],
			instructions: task.steps[0]?.step || ''
		};
	}

	// Convert VirtualTool to Task
	function convertVirtualToolToTask(tool: VirtualTool): Task {
		return {
			id: '', // Will be set by the server when creating
			name: tool.name,
			description: tool.description,
			steps: [
				{
					id: `step_${Date.now()}`,
					step: tool.instructions
				}
			],
			onDemand: {
				params: tool.parameters.reduce(
					(acc, param) => {
						acc[param.name] = param.description;
						return acc;
					},
					{} as Record<string, string>
				)
			}
		};
	}

	// Save tools as tasks
	async function saveToolsAsTasks() {
		if (!projectId || !project?.assistantID) return;

		try {
			// Get existing tasks to check for updates vs creates
			const existingTasks = await ChatService.listTasks(project.assistantID, projectId);
			const existingTaskMap = new Map(existingTasks.items.map((task) => [task.name, task]));

			// Process each tool
			for (const tool of tools) {
				const existingTask = existingTaskMap.get(tool.name);
				const taskData = convertVirtualToolToTask(tool);

				if (existingTask) {
					// Update existing task
					taskData.id = existingTask.id;
					await ChatService.saveTask(project.assistantID, projectId, taskData);
				} else {
					// Create new task
					await ChatService.createTask(project.assistantID, projectId, taskData);
				}
			}

			// Remove tasks that no longer exist in tools
			for (const existingTask of existingTasks.items) {
				const toolExists = tools.some((tool) => tool.name === existingTask.name);
				if (!toolExists) {
					await ChatService.deleteTask(project.assistantID, projectId, existingTask.id);
				}
			}

			console.log('Successfully saved tools as tasks');
		} catch (error) {
			console.error('Failed to save tools as tasks:', error);
		}
	}

	function handleToggleDescription(toolId: string) {
		if (allDescriptionsEnabled) {
			allDescriptionsEnabled = false;
			for (const { id: refToolId } of displayTools) {
				if (toolId !== refToolId) {
					expandedDescriptions[refToolId] = true;
				}
			}
			expandedDescriptions[toolId] = false;
		} else {
			expandedDescriptions[toolId] = !expandedDescriptions[toolId];
		}

		const expandedDescriptionValues = Object.values(expandedDescriptions);
		if (
			expandedDescriptionValues.length === displayTools.length &&
			expandedDescriptionValues.every((v) => v)
		) {
			allDescriptionsEnabled = true;
		}
	}

	// Load tasks when component mounts
	$effect(() => {
		if (projectId && project?.assistantID) {
			loadTasks();
		}
	});

	// Tool editor functions
	function openToolEditor(tool?: VirtualTool, index?: number) {
		if (tool && index !== undefined) {
			// Editing existing tool
			editingTool = JSON.parse(JSON.stringify(tool)); // Deep copy
			editingToolIndex = index;
			isCreatingNewTool = false;
		} else {
			// Creating new tool
			editingTool = {
				name: `Tool_${tools.length + 1}`,
				description: '',
				parameters: [],
				instructions: ''
			};
			editingToolIndex = -1;
			isCreatingNewTool = true;
		}
		toolEditorDialog?.showModal();
	}

	function closeToolEditor() {
		toolEditorDialog?.close();
		editingTool = null;
		editingToolIndex = -1;
		isCreatingNewTool = false;
	}

	async function saveToolFromEditor() {
		if (!editingTool) return;

		if (isCreatingNewTool) {
			// Add new tool
			tools = [...tools, editingTool];
		} else if (editingToolIndex >= 0) {
			// Update existing tool
			tools = tools.map((tool, index) => (index === editingToolIndex ? editingTool! : tool));
		}

		// Save the updated tools as tasks
		await saveToolsAsTasks();

		closeToolEditor();
	}

	async function deleteToolFromEditor() {
		if (editingToolIndex >= 0) {
			tools = tools.filter((_, index) => index !== editingToolIndex);
			// Save the updated tools as tasks
			await saveToolsAsTasks();
		}
		closeToolEditor();
	}

	function addParameterToEditingTool() {
		if (!editingTool) return;
		editingTool.parameters = [
			...editingTool.parameters,
			{ id: `param_${Date.now()}`, name: '', description: '' }
		];
	}

	function removeParameterFromEditingTool(paramIndex: number) {
		if (!editingTool) return;
		editingTool.parameters = editingTool.parameters.filter((_, index) => index !== paramIndex);
	}

	function updateParameterInEditingTool(
		paramIndex: number,
		field: keyof ToolParameter,
		value: string
	) {
		if (!editingTool) return;
		editingTool.parameters = editingTool.parameters.map((param, index) =>
			index === paramIndex ? { ...param, [field]: value } : param
		);
	}

	// Export functions for parent component
	export { saveToolsAsTasks };
</script>

<div class="flex w-full flex-col gap-4">
	<div class="flex w-full flex-col items-center gap-2 md:flex-row">
		{#if error}
			<div class="notification-error flex w-full items-center gap-2 p-3">
				<AlertCircle class="size-4" />
				<div class="flex flex-col">
					<p class="text-sm font-semibold">Unable to retrieve the server's tools</p>
					<p class="text-sm font-light">
						{error}
					</p>
				</div>
			</div>
		{/if}
	</div>

	<div class="flex w-full flex-col gap-2">
		<div class="mb-2 flex w-full flex-col justify-between gap-4">
			<div class="flex flex-wrap items-center justify-end gap-2 md:flex-shrink-0">
				<Toggle
					checked={allDescriptionsEnabled}
					onChange={(checked) => {
						allDescriptionsEnabled = checked;
						expandedDescriptions = {};
					}}
					label="All Descriptions"
					labelInline
					classes={{
						label: 'text-sm gap-2'
					}}
				/>

				{#if !responsive.isMobile}
					<div class="bg-surface3 mx-2 h-5 w-0.5"></div>
				{/if}

				<Toggle
					checked={allParamsEnabled}
					onChange={(checked) => {
						allParamsEnabled = checked;
						expandedParams = {};
					}}
					label="All Parameters"
					labelInline
					classes={{
						label: 'text-sm gap-2'
					}}
				/>
			</div>

			<Search
				class="dark:bg-surface1 dark:border-surface3 border border-transparent bg-white shadow-sm"
				onChange={(val) => (search = val)}
				placeholder="Search tools..."
			/>
		</div>
		<div class="flex flex-col gap-4 overflow-hidden">
			{#if loading}
				<div class="flex items-center justify-center">
					<LoaderCircle class="size-6 animate-spin" />
				</div>
			{:else if displayTools.length > 0}
				{#each displayTools as tool, index (tool.name)}
					<div
						class="border-surface2 dark:bg-surface1 dark:border-surface3 flex flex-col gap-2 rounded-md border bg-white p-3 shadow-sm transition-colors hover:bg-gray-50 dark:hover:bg-gray-700"
						class:pb-2={!expandedDescriptions[tool.id] && !allDescriptionsEnabled}
						class:cursor-pointer={!readonly}
						onclick={() => !readonly && openToolEditor(tools[index], index)}
					>
						<div class="flex items-center justify-between gap-2">
							<p class="text-md font-semibold">
								{tool.name}
							</p>
							<div class="flex flex-shrink-0 items-center gap-2">
								<button
									class="icon-button h-fit min-h-auto w-fit min-w-auto flex-shrink-0 p-1"
									onclick={() => handleToggleDescription(tool.id)}
								>
									{#if expandedDescriptions[tool.id]}
										<ChevronUp class="size-4" />
									{:else}
										<ChevronDown class="size-4" />
									{/if}
								</button>
							</div>
						</div>
						{#if expandedDescriptions[tool.id] || allDescriptionsEnabled}
							<p in:slide={{ axis: 'y' }} class="text-sm font-light text-gray-500">
								{tool.description}
							</p>
							{#if Object.keys(tool.params ?? {}).length > 0}
								{#if expandedParams[tool.id] || allParamsEnabled}
									<div
										class="from-surface2 dark:from-surface3 flex w-full flex-shrink-0 bg-linear-to-r to-transparent px-4 py-2 text-xs font-semibold text-gray-500 md:w-sm"
									>
										Parameters
									</div>
									<div class="flex flex-col px-4 text-xs" in:slide={{ axis: 'y' }}>
										<div class="flex flex-col gap-2">
											{#each Object.keys(tool.params ?? {}) as paramKey (paramKey)}
												<div class="flex flex-col items-center gap-2 md:flex-row">
													<p class="self-start font-semibold text-gray-500 md:min-w-xs">
														{paramKey}
													</p>
													<p class="self-start font-light text-gray-500">
														{tool.params?.[paramKey]}
													</p>
												</div>
											{/each}
										</div>
									</div>
								{/if}
							{/if}
						{/if}
					</div>
				{/each}
			{:else}
				<div class="mt-12 flex w-md flex-col items-center gap-4 self-center text-center">
					<Wrench class="size-24 text-gray-200 dark:text-gray-900" />
					<h4 class="text-lg font-semibold text-gray-400 dark:text-gray-600">No tools</h4>
					<p class="text-sm font-light text-gray-400 dark:text-gray-600">
						This virtual server doesn't have any tools configured yet.
					</p>
				</div>
			{/if}
		</div>
	</div>
</div>

{#if !readonly}
	<div
		class="sticky bottom-0 left-0 flex w-full justify-end bg-gray-50 py-4 md:px-4 dark:bg-inherit"
	>
		<button onclick={() => openToolEditor()} class="button-primary flex items-center gap-1">
			<Plus class="size-4" />
			Add Tool
		</button>
	</div>
{/if}

<!-- Tool Editor Dialog -->
<dialog
	bind:this={toolEditorDialog}
	use:clickOutside={() => closeToolEditor()}
	class="default-dialog w-[90vw] max-w-4xl bg-gray-50 p-0 dark:bg-black"
>
	<div class="default-scrollbar-thin relative mx-auto h-full min-h-0 w-full overflow-y-auto">
		<div
			class="flex items-center justify-between border-b border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800"
		>
			<h2 class="text-lg font-semibold">
				{isCreatingNewTool ? 'Add New Tool' : 'Edit Tool'}
			</h2>
			<button class="icon-button" onclick={closeToolEditor}>
				<X class="size-6" />
			</button>
		</div>

		{#if editingTool}
			<div class="p-6">
				<div class="flex flex-col gap-6">
					<!-- Tool Name -->
					<div class="flex flex-col gap-2">
						<label class="text-sm font-semibold">Name</label>
						<input
							bind:value={editingTool.name}
							class="text-input-filled"
							placeholder="Enter tool name"
						/>
					</div>

					<!-- Tool Description -->
					<div class="flex flex-col gap-2">
						<label class="text-sm font-semibold">Description</label>
						<textarea
							bind:value={editingTool.description}
							class="text-input-filled min-h-20 resize-none"
							placeholder="Enter tool description"
						></textarea>
					</div>

					<!-- Parameters -->
					<div class="flex flex-col gap-2">
						<div class="flex items-center justify-between">
							<label class="text-sm font-semibold">Parameters</label>
							<button
								onclick={addParameterToEditingTool}
								class="button-small flex items-center gap-1"
							>
								<Plus class="size-3" />
								Add Parameter
							</button>
						</div>
						<p class="text-xs text-gray-500 dark:text-gray-400">
							Parameters will be provided by the LLM when invoking tool calls
						</p>

						{#if editingTool.parameters.length > 0}
							<Table
								data={editingTool.parameters}
								fields={['name', 'description']}
								headers={[
									{ title: 'Name', property: 'name' },
									{ title: 'Description', property: 'description' }
								]}
							>
								{#snippet onRenderColumn(property, d)}
									<input
										value={d[property as keyof ToolParameter]}
										onchange={(e) => {
											const target = e.target as HTMLInputElement;
											const paramIndex = editingTool!.parameters.indexOf(d);
											updateParameterInEditingTool(
												paramIndex,
												property as keyof ToolParameter,
												target.value
											);
										}}
										class="text-input-filled text-sm"
									/>
								{/snippet}
								{#snippet actions(d)}
									<button
										onclick={() => {
											const paramIndex = editingTool!.parameters.indexOf(d);
											removeParameterFromEditingTool(paramIndex);
										}}
										class="icon-button hover:text-red-500"
									>
										<Trash2 class="size-3" />
									</button>
								{/snippet}
							</Table>
						{/if}
					</div>

					<!-- Instructions -->
					<div class="flex flex-col gap-2">
						<label class="text-sm font-semibold">Instructions</label>
						<p class="text-xs text-gray-500 dark:text-gray-400">
							You can use parameters in your instructions with the syntax $NAME
						</p>
						<textarea
							bind:value={editingTool.instructions}
							class="text-input-filled min-h-32 resize-none"
							placeholder="Enter instructions for this tool"
						></textarea>
					</div>
				</div>
			</div>

			<!-- Dialog Footer -->
			<div
				class="flex items-center justify-between border-t border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800"
			>
				<div>
					{#if !isCreatingNewTool}
						<button onclick={deleteToolFromEditor} class="button-destructive"> Delete Tool </button>
					{/if}
				</div>
				<div class="flex gap-2">
					<button onclick={closeToolEditor} class="button-secondary"> Cancel </button>
					<button onclick={saveToolFromEditor} class="button-primary">
						{isCreatingNewTool ? 'Add Tool' : 'Save Changes'}
					</button>
				</div>
			</div>
		{/if}
	</div>
</dialog>
