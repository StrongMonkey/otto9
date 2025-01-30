import { useEffect, useState } from "react";
import useSWR from "swr";

import { ToolInfo } from "~/lib/model/agents";
import { AssistantNamespace } from "~/lib/model/assistants";
import { AgentService } from "~/lib/service/api/agentService";
import { WorkflowService } from "~/lib/service/api/workflowService";

export function useToolAuthPolling(
	namespace: AssistantNamespace,
	entityId?: Nullish<string>
) {
	const [toolInfo, setToolInfo] = useState<Record<string, ToolInfo> | null>(
		null
	);
	const [isPolling, setIsPolling] = useState(false);
	const refreshInterval = isPolling ? 1000 : undefined;

	const { data: agent } = useSWR(
		namespace === AssistantNamespace.Agents
			? AgentService.getAgentById.key(entityId)
			: null,
		({ agentId }) => AgentService.getAgentById(agentId),
		{ refreshInterval }
	);

	const { data: workflow } = useSWR(
		namespace === AssistantNamespace.Workflows
			? WorkflowService.getWorkflowById.key(entityId)
			: null,
		({ workflowId }) => WorkflowService.getWorkflowById(workflowId),
		{ refreshInterval }
	);

	useEffect(() => {
		const getInfo = () => {
			const agentTools = [
				...(agent?.tools ?? []),
				...(agent?.availableThreadTools ?? []),
				...(agent?.defaultThreadTools ?? []),
			];

			switch (namespace) {
				case AssistantNamespace.Agents:
					return { tools: agentTools, toolInfo: agent?.toolInfo };
				case AssistantNamespace.Workflows:
					return { tools: workflow?.tools, toolInfo: workflow?.toolInfo };
				default:
					return {};
			}
		};

		const { tools, toolInfo: toolInfoFromAgent } = getInfo();
		if (toolInfoFromAgent) setToolInfo(toolInfoFromAgent);

		// when tool credentials are processing the api will respond with { toolInfo: null }
		// we need to poll until the toolInfo is not null or there are no tools
		const shouldPoll = !!tools?.length && !toolInfoFromAgent;
		if (shouldPoll !== isPolling) setIsPolling(shouldPoll);
	}, [agent, workflow, isPolling, namespace]);

	return { toolInfo, setToolInfo };
}
