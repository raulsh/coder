import type { GetJFrogXRayScanParams } from "api/api";
import { api } from "api/api";

export const xrayScan = (params: GetJFrogXRayScanParams) => {
  return {
    queryKey: ["xray", params],
    queryFn: () => api.getJFrogXRayScan(params),
  };
};
