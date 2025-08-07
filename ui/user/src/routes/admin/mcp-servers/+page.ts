import { handleRouteError } from '$lib/errors';
import { ChatService } from '$lib/services';
import { profile } from '$lib/stores';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ url, fetch }) => {
    const projectId = url.searchParams.get('projectId') || ''
	console.log('projectId', projectId)

	if (!projectId) {
		return {
			mcpServers: []
		};
	}

	try {
        const project = await ChatService.getProject(projectId, {fetch})
        const mcpServers = await ChatService.listProjectMCPs(project.assistantID, project.id, {fetch})  
        
        return {
            mcpServers
        }
	} catch (err) {
		handleRouteError(err, '/admin/mcp-servers', profile.current);
		
		return {
			mcpServers: []
		};
	}
};
