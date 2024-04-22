import { api } from "api/api";
import type {
  DeleteWorkspaceAgentPortShareRequest,
  UpsertWorkspaceAgentPortShareRequest,
} from "api/typesGenerated";

export const workspacePortShares = (workspaceId: string) => {
  return {
    queryKey: ["sharedPorts", workspaceId],
    queryFn: () => api.getWorkspaceAgentSharedPorts(workspaceId),
  };
};

export const upsertWorkspacePortShare = (workspaceId: string) => {
  return {
    mutationFn: async (options: UpsertWorkspaceAgentPortShareRequest) => {
      await api.upsertWorkspaceAgentSharedPort(workspaceId, options);
    },
  };
};

export const deleteWorkspacePortShare = (workspaceId: string) => {
  return {
    mutationFn: async (options: DeleteWorkspaceAgentPortShareRequest) => {
      await api.deleteWorkspaceAgentSharedPort(workspaceId, options);
    },
  };
};
