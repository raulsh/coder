import { api } from "api/api";

export const updateCheck = () => {
  return {
    queryKey: ["updateCheck"],
    queryFn: () => api.getUpdateCheck(),
  };
};
