import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { graphqlRequest } from '@/gql/graphql';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { UsageLog, UsageLogConnection, usageLogConnectionSchema, usageLogSchema } from './usage-logs-schema';

// Dynamic GraphQL query builder
function buildUsageLogsQuery() {
  return `
    query GetUsageLogs($first: Int, $after: Cursor, $orderBy: UsageLogOrder, $where: UsageLogWhereInput) {
      usageLogs(first: $first, after: $after, orderBy: $orderBy, where: $where) {
        edges {
          node {
            id
            createdAt
            updatedAt
            requestID
            channel {
              id
              name
              type
            }
            modelID
            promptTokens
            completionTokens
            totalTokens
            promptAudioTokens
            promptCachedTokens
            promptWriteCachedTokens
            completionAudioTokens
            completionReasoningTokens
            completionAcceptedPredictionTokens
            completionRejectedPredictionTokens
            source
            format
            totalCost
            costPriceReferenceID
            costItems {
              itemCode
              quantity
              subtotal
            }
          }
          cursor
        }
        pageInfo {
          hasNextPage
          hasPreviousPage
          startCursor
          endCursor
        }
        totalCount
      }
    }
  `;
}

function buildUsageLogDetailQuery() {
  return `
    query GetUsageLog($id: ID!) {
      node(id: $id) {
        ... on UsageLog {
          id
          createdAt
          updatedAt
          requestID
          channel {
            id
            name
            type
          }
          modelID
          promptTokens
          completionTokens
          totalTokens
          promptAudioTokens
          promptCachedTokens
          promptWriteCachedTokens
          completionAudioTokens
          completionReasoningTokens
          completionAcceptedPredictionTokens
          completionRejectedPredictionTokens
          source
          format
          totalCost
          costPriceReferenceID
          costItems {
            itemCode
            quantity
            subtotal
          }
        }
      }
    }
  `;
}

// Query hooks
export function useUsageLogs(variables?: {
  first?: number;
  after?: string;
  orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' };
  where?: {
    source?: string;
    modelID?: string;
    channelID?: string;
    requestID?: string;
    [key: string]: any;
  };
}, options?: { enabled?: boolean }) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();
  const enabled = options?.enabled ?? true;

  return useQuery({
    queryKey: ['usageLogs', variables],
    queryFn: async () => {
      try {
        const query = buildUsageLogsQuery();
        const data = await graphqlRequest<{ usageLogs: UsageLogConnection }>(query, variables);
        return usageLogConnectionSchema.parse(data?.usageLogs);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled,
  });
}

export function useUsageLog(id: string) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();

  return useQuery({
    queryKey: ['usageLog', id],
    queryFn: async () => {
      try {
        const query = buildUsageLogDetailQuery();
        const data = await graphqlRequest<{ node: UsageLog }>(query, { id });
        if (!data.node) {
          throw new Error('Usage log not found');
        }
        return usageLogSchema.parse(data.node);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: !!id,
  });
}
