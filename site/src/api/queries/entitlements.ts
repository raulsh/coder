import type { QueryClient, UseQueryOptions } from "react-query";
import { api } from "api/api";
import type { Entitlements } from "api/typesGenerated";
import { getMetadataAsJSON } from "utils/metadata";

const initialEntitlementsData = getMetadataAsJSON<Entitlements>("entitlements");
const ENTITLEMENTS_QUERY_KEY = ["entitlements"] as const;

export const entitlements = (): UseQueryOptions<Entitlements> => {
  return {
    queryKey: ENTITLEMENTS_QUERY_KEY,
    queryFn: () => api.getEntitlements(),
    initialData: initialEntitlementsData,
  };
};

export const refreshEntitlements = (queryClient: QueryClient) => {
  return {
    mutationFn: api.refreshEntitlements,
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ENTITLEMENTS_QUERY_KEY,
      });
    },
  };
};
