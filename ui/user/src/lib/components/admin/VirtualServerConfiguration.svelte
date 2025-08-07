<script lang="ts">
	import { Plus, Trash2, Server, X, AlertCircle, LoaderCircle } from 'lucide-svelte';
	import McpServerSetup from '../chat/McpServerSetup.svelte';
	import McpServerInfoAndTools from '../mcp/McpServerInfoAndTools.svelte';
	import {
		ChatService,
		type Project,
		type Task,
		type ProjectMCP,
		type MCPServerTool
	} from '$lib/services';
	import { clickOutside } from '$lib/actions/clickoutside';

	import Table from '../Table.svelte';

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
		showTools?: boolean;
	}

	let { projectId, showTools = false }: Props = $props();

	let tools = $state<VirtualTool[]>([]);
	let projectMcpServers = $state<{ name: string; icon?: string; id: string }[]>([]);

	// MCP Server Setup
	let mcpServerSetup = $state<ReturnType<typeof McpServerSetup>>();
	let project = $state<Project | null>(null);

	// MCP Server Info Dialog
	let mcpServerInfoDialog = $state<HTMLDialogElement>();
	let selectedMcpServer = $state<ProjectMCP | null>(null);

	// Tool display state (when showTools is enabled)
	let loading = $state(false);
	let error = $state('');

	// Tool editor state
	let editingTool = $state<VirtualTool | null>(null);
	let editingToolIndex = $state<number>(-1);
	let toolEditorDialog = $state<HTMLDialogElement>();
	let isCreatingNewTool = $state(false);

	// Load existing project MCP servers and tasks on initialization
	$effect(() => {
		if (projectId) {
			loadProject().then(() => {
				if (project?.assistantID) {
					loadProjectMcpServers();
					// Load tasks if showTools is enabled
					if (showTools) {
						loadTasks();
					}
				}
			});
		}
	});

	// Project MCP Server management
	async function removeProjectMcpServer(index: number) {
		if (!projectId || !project?.assistantID) return;

		const serverToRemove = projectMcpServers[index];
		if (!serverToRemove) return;

		try {
			// Find the actual ProjectMCP to get the correct ID for deletion
			const projectMcps = await ChatService.listProjectMCPs(project.assistantID, projectId);
			const projectMcp = projectMcps.find((mcp) => mcp.mcpID === serverToRemove.id);

			if (projectMcp) {
				// Remove from the project via API using the ProjectMCP id
				await ChatService.deleteProjectMCP(project.assistantID, projectId, projectMcp.id);
			}

			// Remove from local list
			projectMcpServers = projectMcpServers.filter((_, i) => i !== index);
		} catch (error) {
			console.error('Failed to remove MCP server from project:', error);
		}
	}

	async function setupProjectMcp(projectMcp?: ProjectMCP) {
		if (!projectId || !project?.assistantID || !projectMcp) return;

		try {
			// Check if server is already added to prevent duplicates
			const serverId = projectMcp.mcpID;
			const isAlreadyAdded = projectMcpServers.some((server) => server.id === serverId);

			if (isAlreadyAdded) {
				console.log('Server already added to project');
				return;
			}

			// Refresh the list to show the newly added server
			await loadProjectMcpServers();
		} catch (error) {
			console.error('Failed to add MCP server to project:', error);
		}
	}

	async function loadProject() {
		if (!projectId) return;

		try {
			project = await ChatService.getProject(projectId);
		} catch (error) {
			console.error('Failed to load project:', error);
		}
	}

	async function loadProjectMcpServers() {
		if (!projectId || !project?.assistantID) return;

		try {
			const projectMcps = await ChatService.listProjectMCPs(project.assistantID, projectId);
			projectMcpServers = projectMcps.map((mcp) => ({
				name: mcp.name || `Server ${mcp.id}`,
				icon: mcp.icon,
				id: mcp.mcpID
			}));
		} catch (error) {
			console.error('Failed to load project MCP servers:', error);
		}
	}

	async function loadTasks() {
		if (!projectId || !project?.assistantID) return;

		try {
			const taskList = await ChatService.listTasks(project.assistantID, projectId);
			// Convert tasks back to tools
			tools = taskList.items.map((task) => convertTaskToTool(task));
		} catch (error) {
			console.error('Failed to load tasks:', error);
		}
	}

	// Convert tool to task
	function convertToolToTask(tool: VirtualTool): Task {
		const task: Task = {
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
		return task;
	}

	// Convert task to tool
	function convertTaskToTool(task: Task): VirtualTool {
		const tool: VirtualTool = {
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
		return tool;
	}

	// Save all tools as tasks
	async function saveToolsAsTasks() {
		if (!projectId || !project?.assistantID) return;

		try {
			// Get existing tasks to check for updates vs creates
			const existingTasks = await ChatService.listTasks(project.assistantID, projectId);
			const existingTaskMap = new Map(existingTasks.items.map((task) => [task.name, task]));

			// Process each tool
			for (const tool of tools) {
				const existingTask = existingTaskMap.get(tool.name);
				const taskData = convertToolToTask(tool);

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

	async function openCatalogDialog() {
		if (!project) {
			await loadProject();
		}
		mcpServerSetup?.open();
	}

	async function openMcpServerInfoDialog(server: { name: string; icon?: string; id: string }) {
		if (!project?.assistantID) return;

		try {
			// Get the full ProjectMCP data
			const projectMcps = await ChatService.listProjectMCPs(project.assistantID, projectId!);
			const projectMcp = projectMcps.find((mcp) => mcp.mcpID === server.id);

			if (projectMcp) {
				selectedMcpServer = projectMcp;
				mcpServerInfoDialog?.showModal();
			}
		} catch (error) {
			console.error('Failed to load MCP server details:', error);
		}
	}

	function closeMcpServerInfoDialog() {
		mcpServerInfoDialog?.close();
		selectedMcpServer = null;
	}

	// Export data for parent component
	export function getConfiguration() {
		return {
			projectMcpServers,
			tools
		};
	}

	export function setConfiguration(config: {
		projectMcpServers: typeof projectMcpServers;
		tools: typeof tools;
	}) {
		projectMcpServers = config.projectMcpServers;
		tools = config.tools;
	}

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

	let displayTools = $derived(tools.map((tool) => convertVirtualToolToMcpServerTool(tool)));

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

	// Export save function for parent component
	export { saveToolsAsTasks };
</script>

<div class="flex flex-col gap-8">
	<div class="flex flex-col gap-6">
		<!-- Project MCP Servers -->
		<div class="flex flex-col gap-4">
			<h3 class="text-lg font-semibold">MCP Servers</h3>

			{#each projectMcpServers as server, index (index)}
				<div
					class="cursor-pointer rounded-lg border border-gray-200 bg-white p-4 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:hover:bg-gray-700"
					onclick={() => openMcpServerInfoDialog(server)}
				>
					<div class="flex items-center justify-between">
						<div class="flex items-center gap-2">
							{#if server.icon}
								<img src={server.icon} alt={server.name} class="size-6 rounded" />
							{:else}
								<div
									class="flex size-6 items-center justify-center rounded bg-gray-100 dark:bg-gray-600"
								>
									<Server class="size-4 text-gray-500" />
								</div>
							{/if}
							<span class="text-sm">{server.name}</span>
						</div>
						<button
							onclick={(e) => {
								e.stopPropagation();
								removeProjectMcpServer(index);
							}}
							class="icon-button hover:text-red-500"
						>
							<Trash2 class="size-4" />
						</button>
					</div>
				</div>
			{/each}

			{#if projectMcpServers.length === 0}
				<div
					class="rounded-lg border border-gray-200 bg-gray-50 p-4 text-center dark:border-gray-700 dark:bg-gray-800"
				>
					<p class="text-sm text-gray-600 dark:text-gray-400">
						No MCP servers added yet. Add servers to enable their capabilities in your tools.
					</p>
				</div>
			{/if}

			<div class="text-sm text-gray-600 dark:text-gray-400">
				<p>
					MCP servers provide additional capabilities that your tools can leverage. Once added,
					these servers will be available for your tools to invoke, helping you complete various
					tasks more effectively.
				</p>
			</div>

			<button onclick={openCatalogDialog} class="button-small flex items-center gap-1 self-end">
				<Plus class="size-4" />
				Add MCP Server
			</button>
		</div>

		<!-- Tools Section - Only show when showTools is enabled -->
		{#if showTools}
			<div class="flex flex-col gap-4">
				<h3 class="text-lg font-semibold">Tools</h3>

				<div class="text-sm text-gray-600 dark:text-gray-400">
					<p>
						Tools define custom capabilities that can be invoked using natural language. Each tool
						includes instructions and parameters that help the AI understand how to execute specific
						tasks effectively.
					</p>
				</div>

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
					<div class="flex flex-col gap-4 overflow-hidden">
						{#if loading}
							<div class="flex items-center justify-center">
								<LoaderCircle class="size-6 animate-spin" />
							</div>
						{:else if displayTools.length > 0}
							{#each displayTools as tool, index (tool.name)}
								<div
									class="border-surface2 dark:bg-surface1 dark:border-surface3 flex cursor-pointer flex-col gap-2 rounded-md border bg-white p-3 shadow-sm transition-colors hover:bg-gray-50 dark:hover:bg-gray-700"
									onclick={() => openToolEditor(tools[index], index)}
								>
									<div class="flex items-center justify-between gap-2">
										<p class="text-md font-semibold">
											{tool.name}
										</p>
									</div>

									<p class="text-sm font-light text-gray-500">
										{tool.description}
									</p>

									{#if Object.keys(tool.params ?? {}).length > 0}
										<div
											class="from-surface2 dark:from-surface3 flex w-full flex-shrink-0 bg-linear-to-r to-transparent px-4 py-2 text-xs font-semibold text-gray-500 md:w-sm"
										>
											Parameters
										</div>
										<div class="flex flex-col px-4 text-xs">
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
								</div>
							{/each}
						{/if}
					</div>
				</div>

				<button
					onclick={() => openToolEditor()}
					class="button-small flex items-center gap-1 self-end"
				>
					<Plus class="size-4" />
					Add Tool
				</button>
			</div>
		{/if}
	</div>
</div>

<!-- MCP Server Setup -->
{#if project}
	<McpServerSetup bind:this={mcpServerSetup} {project} onSuccess={setupProjectMcp} />
{/if}

<!-- MCP Server Info Dialog -->
<dialog
	bind:this={mcpServerInfoDialog}
	use:clickOutside={() => closeMcpServerInfoDialog()}
	class="default-dialog h-[80vh] w-[90vw] max-w-6xl bg-gray-50 p-0 dark:bg-black"
>
	<div class="default-scrollbar-thin relative mx-auto h-full min-h-0 w-full overflow-y-auto">
		<button
			class="icon-button sticky top-3 right-2 z-40 float-right self-end"
			onclick={() => closeMcpServerInfoDialog()}
		>
			<X class="size-7" />
		</button>
		<div class="pr-18">
			{#if selectedMcpServer}
				<McpServerInfoAndTools entry={selectedMcpServer} project={project || undefined} />
			{/if}
		</div>
	</div>
</dialog>

<!-- Tool Editor Dialog -->
{#if showTools}
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
							<button onclick={deleteToolFromEditor} class="button-destructive">
								Delete Tool
							</button>
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
{/if}
