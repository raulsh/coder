import type { QueryClient, QueryOptions } from "react-query";
import { api } from "api/api";
import type {
  UpdateUserQuietHoursScheduleRequest,
  UserQuietHoursScheduleResponse,
} from "api/typesGenerated";

export const userQuietHoursScheduleKey = (userId: string) => [
  "settings",
  userId,
  "quietHours",
];

export const userQuietHoursSchedule = (
  userId: string,
): QueryOptions<UserQuietHoursScheduleResponse> => {
  return {
    queryKey: userQuietHoursScheduleKey(userId),
    queryFn: () => api.getUserQuietHoursSchedule(userId),
  };
};

export const updateUserQuietHoursSchedule = (
  userId: string,
  queryClient: QueryClient,
) => {
  return {
    mutationFn: (request: UpdateUserQuietHoursScheduleRequest) =>
      api.updateUserQuietHoursSchedule(userId, request),
    onSuccess: async () => {
      await queryClient.invalidateQueries(userQuietHoursScheduleKey(userId));
    },
  };
};
