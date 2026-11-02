import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { usageStatsByUserSchema, type UsageStatsByUser } from '@/features/dashboard/data/dashboard';

const USAGE_STATS_BY_USER_QUERY = `
  query GetUsageStatsByUser($timeWindow: String) {
    usageStatsByUser(timeWindow: $timeWindow) {
      userId
      userName
      requestCount
      totalTokens
      totalCost
    }
  }
`;

export function useUsageStatsByUser(timeWindow?: string) {
  return useQuery({
    queryKey: ['usageStatsByUser', timeWindow],
    queryFn: async () => {
      const data = await graphqlRequest<{ usageStatsByUser: UsageStatsByUser[] }>(
        USAGE_STATS_BY_USER_QUERY,
        { timeWindow }
      );
      return data.usageStatsByUser.map((item) => usageStatsByUserSchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

