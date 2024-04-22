import { api } from "api/api";

export const roles = () => {
  return {
    queryKey: ["roles"],
    queryFn: api.getRoles,
  };
};
