import type { QueryClient, UseQueryOptions } from "react-query";
import { api } from "api/api";
import type { AppearanceConfig } from "api/typesGenerated";
import { getMetadataAsJSON } from "utils/metadata";

const initialAppearanceData = getMetadataAsJSON<AppearanceConfig>("appearance");
const appearanceConfigKey = ["appearance"] as const;

export const appearance = (): UseQueryOptions<AppearanceConfig> => {
  return {
    queryKey: ["appearance"],
    initialData: initialAppearanceData,
    queryFn: () => api.getAppearance(),
  };
};

export const updateAppearance = (queryClient: QueryClient) => {
  return {
    mutationFn: api.updateAppearance,
    onSuccess: (newConfig: AppearanceConfig) => {
      queryClient.setQueryData(appearanceConfigKey, newConfig);
    },
  };
};
