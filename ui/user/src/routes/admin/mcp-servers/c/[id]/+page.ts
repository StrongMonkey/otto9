import { DEFAULT_MCP_CATALOG_ID } from '$lib/constants';
import { handleRouteError } from '$lib/errors';
import { AdminService, ChatService } from '$lib/services';
import { profile } from '$lib/stores';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch }) => {
	const { id } = params;

	let catalogEntry;
	try {
		catalogEntry = await AdminService.getMCPCatalogEntry(DEFAULT_MCP_CATALOG_ID, id, { fetch });
	} catch (err) {
		handleRouteError(err, `/admin/mcp-servers/c/${id}`, profile.current);
	}

	let mcpServers;
	if (catalogEntry?.manifest.projectID) {
		const project = await ChatService.getProject(catalogEntry.manifest.projectID, { fetch });
		mcpServers = await ChatService.listProjectMCPs(project.assistantID, project.id, { fetch });
	}

	return {
		catalogEntry,
		mcpServers
	};
};
