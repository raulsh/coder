import type { UseQueryOptions } from "react-query";
import { api } from "api/api";
import type { Experiments } from "api/typesGenerated";
import { getMetadataAsJSON } from "utils/metadata";

const initialExperimentsData = getMetadataAsJSON<Experiments>("experiments");
const experimentsKey = ["experiments"] as const;

export const experiments = (): UseQueryOptions<Experiments> => {
  return {
    queryKey: experimentsKey,
    initialData: initialExperimentsData,
    queryFn: () => api.getExperiments(),
  } satisfies UseQueryOptions<Experiments>;
};

export const availableExperiments = () => {
  return {
    queryKey: ["availableExperiments"],
    queryFn: async () => api.getAvailableExperiments(),
  };
};
