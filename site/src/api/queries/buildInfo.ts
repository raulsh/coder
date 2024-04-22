import type { UseQueryOptions } from "react-query";
import { api } from "api/api";
import type { BuildInfoResponse } from "api/typesGenerated";
import { getMetadataAsJSON } from "utils/metadata";

const initialBuildInfoData = getMetadataAsJSON<BuildInfoResponse>("build-info");
const buildInfoKey = ["buildInfo"] as const;

export const buildInfo = (): UseQueryOptions<BuildInfoResponse> => {
  return {
    queryKey: buildInfoKey,
    initialData: initialBuildInfoData,
    queryFn: () => api.getBuildInfo(),
  };
};
