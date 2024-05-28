import type { QueryClient, UseQueryOptions } from "react-query";
import { API } from "api/api";
import type { CreateIntelCohortRequest, IntelCohort, IntelReport } from "api/typesGenerated";

export const intelCohortsKey = ["intel", "cohorts"];

export const intelCohorts = (
  organizationId: string,
): UseQueryOptions<IntelCohort[]> => {
  return {
    queryKey: intelCohortsKey,
    queryFn: () => API.getIntelCohorts(organizationId),
  };
};

export const intelReportKey = (): string[] => [
  "intel",
  "report",
];

export const intelReport = (organizationId: string): UseQueryOptions<IntelReport> => {
  return {
    queryKey: intelReportKey(),
    queryFn: () => API.getIntelReport(organizationId),
  };
};

export const createIntelCohort = (organizationId: string) => {
  return {
    mutationFn: async (request: CreateIntelCohortRequest) => {
      const newCohort = await API.createIntelCohort(organizationId, request);
      return newCohort;
    },
  }
}
