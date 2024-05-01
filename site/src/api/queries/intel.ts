import type { UseQueryOptions } from "react-query"
import * as API from "api/api"
import type { IntelCohort } from "api/typesGenerated"

const intelCohortsKey = ["intel", "cohorts"]

export const intelCohorts = (organizationId: string): UseQueryOptions<IntelCohort[]> => {
  return {
    queryKey: intelCohortsKey,
    queryFn: () => API.getIntelCohorts(organizationId),
  }
}
