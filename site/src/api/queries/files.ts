import { api } from "api/api";

export const uploadFile = () => {
  return {
    mutationFn: api.uploadFile,
  };
};

export const file = (fileId: string) => {
  return {
    queryKey: ["files", fileId],
    queryFn: () => api.getFile(fileId),
  };
};
