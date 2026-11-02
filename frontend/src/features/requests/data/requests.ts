import { useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
import { useErrorHandler } from '@/hooks/use-error-handler';
import {
  Request,
  RequestConnection,
  RequestExecutionConnection,
  requestConnectionSchema,
  requestExecutionConnectionSchema,
  requestSchema,
} from './schema';

// Dynamic GraphQL query builder
function buildRequestsQuery() {
  return `
    query GetRequests(
      $first: Int
      $after: Cursor
      $last: Int
      $before: Cursor
      $orderBy: RequestOrder
      $where: RequestWhereInput
    ) {
      requests(first: $first, after: $after, last: $last, before: $before, orderBy: $orderBy, where: $where) {
        edges {
          node {
            id
            createdAt
            updatedAt
            apiKey {
              id
              name
              user {
                firstName
                lastName
              }
            }
            channel {
              id
              name
            }
            source
            modelID
            format
            reasoningEffort
            stream
            status
            clientIP
            userAgent
            metricsLatencyMs
            metricsFirstTokenLatencyMs
            metricsReasoningDurationMs
            executions(first: 10, orderBy: { field: CREATED_AT, direction: DESC }) {
              edges {
                node {
                  id
                  createdAt
                  modelID
                  upstreamModelID
                  format
                  status
                  reasoningEffort
                  passThroughApplied
                  channel {
                    id
                    name
                  }
                  channelAPIKeyIndex
                }
                cursor
              }
            }
            usageLogs(first: 1) {
              edges {
                node {
                  id
                  promptTokens
                  completionTokens
                  completionReasoningTokens
                  totalTokens
                  promptCachedTokens
                  promptWriteCachedTokens
                  totalCost
                  costPriceReferenceID
                }
              }
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
      }
    }
  `;
}

function buildRequestDetailQuery() {
  return `
    query GetRequestDetail($id: ID!) {
      node(id: $id) {
        ... on Request {
          id
          createdAt
          updatedAt
          apiKey {
            id
            name
            user {
              firstName
              lastName
            }
          }
          channel {
            id
            name
          }
          source
          modelID
          stream
          clientIP
          dataStorageID
          contentSaved
          contentStorageKey
          requestHeaders
          responseHeaders
          requestBody
          responseBody
          responseChunks
          status
          format
          metricsReasoningDurationMs
          usageLogs(first: 1) {
            edges {
              node {
                  id
                  promptTokens
                  completionTokens
                  completionReasoningTokens
                  totalTokens
                  promptCachedTokens
                  promptWriteCachedTokens
                  totalCost
                  costPriceReferenceID
                }
            }
          }
        }
      }
    }
  `;
}

function buildRequestDetailPollingQuery() {
  return `
    query GetRequestDetailPolling($id: ID!) {
      node(id: $id) {
        ... on Request {
          id
          createdAt
          updatedAt
          apiKey {
            id
            name
            user {
              firstName
              lastName
            }
          }
          channel {
            id
            name
          }
          source
          modelID
          stream
          clientIP
          dataStorageID
          contentSaved
          contentStorageKey
          responseHeaders
          status
          format
          metricsReasoningDurationMs
        }
      }
    }
  `;
}

function buildRequestExecutionsQuery() {
  return `
    query GetRequestExecutions(
      $requestID: ID!
      $first: Int
      $after: Cursor
      $orderBy: RequestExecutionOrder
      $where: RequestExecutionWhereInput
    ) {
      node(id: $requestID) {
        ... on Request {
          executions(first: $first, after: $after, orderBy: $orderBy, where: $where) {
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
                    baseURL
                }
                channelAPIKeySuffix
                channelAPIKeyIndex
                modelID
                upstreamModelID
                dataStorageID
                requestHeaders
                responseHeaders
                requestBody
                responseBody
                responseChunks
                errorMessage
                responseStatusCode
                status
                format
                reasoningEffort
                stream
                requestURL
                passThroughApplied
                metricsFirstTokenLatencyMs
                metricsReasoningDurationMs
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
      }
    }
  `;
}

// Query hooks
export function useRequests(variables?: {
  first?: number;
  after?: string;
  last?: number;
  before?: string;
  orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' };
  where?: {
    status?: string;
    source?: string;
    channelID?: string;
    channelIDIn?: string[];
    statusIn?: string[];
    sourceIn?: string[];
    [key: string]: any;
  };
}, options?: { enabled?: boolean }) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();
  const enabled = options?.enabled ?? true;

  return useQuery({
    queryKey: ['requests', variables],
    queryFn: async () => {
      try {
        const query = buildRequestsQuery();
        const data = await graphqlRequest<{ requests: RequestConnection }>(query, variables);
        return requestConnectionSchema.parse(data?.requests);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled,
    refetchOnWindowFocus: false,
  });
}

export function useRequest(
  id: string,
  options?: {
    enabled?: boolean;
    disableAutoRefresh?: boolean;
  }
) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const enabled = options?.enabled ?? true;

  const queryKey = ['request', id] as const;

  return useQuery({
    queryKey,
    queryFn: async () => {
      try {
        const previousRequest = queryClient.getQueryData<Request>(queryKey);
        const shouldUseLightweightPolling = previousRequest?.status === 'processing';

        const query = shouldUseLightweightPolling
          ? buildRequestDetailPollingQuery()
          : buildRequestDetailQuery();

        const data = await graphqlRequest<{ node: Request }>(query, { id });
        if (!data.node) {
          throw new Error('Request not found');
        }

        const parsedRequest = requestSchema.parse(data.node);

        if (!shouldUseLightweightPolling) {
          return parsedRequest;
        }

        if (parsedRequest.status !== 'processing') {
          const fullData = await graphqlRequest<{ node: Request }>(buildRequestDetailQuery(), { id });
          if (!fullData.node) {
            throw new Error('Request not found');
          }
          return requestSchema.parse(fullData.node);
        }

        return requestSchema.parse({
          ...previousRequest,
          ...parsedRequest,
          requestHeaders: previousRequest?.requestHeaders,
          requestBody: previousRequest?.requestBody,
          responseBody: previousRequest?.responseBody,
          responseChunks: previousRequest?.responseChunks,
          usageLogs: previousRequest?.usageLogs,
        });
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: enabled && !!id,
    refetchInterval: (query) => {
      if (options?.disableAutoRefresh) {
        return false;
      }

      return query.state.data?.status === 'processing' ? 2000 : false;
    },
  });
}

/**
 * Imperative (non-hook) fetch of a page of requests for drawer navigation.
 * direction 'older' fetches the page after endCursor (older in DESC order).
 * direction 'newer' fetches the page before startCursor (newer in DESC order).
 */
export async function fetchAdjacentRequestPage(params: {
  cursor: string;
  direction: 'older' | 'newer';
  pageSize: number;
  where?: Record<string, any>;
}): Promise<{ requests: Request[]; pageInfo: RequestConnection['pageInfo'] }> {
  const query = buildRequestsQuery();
  const variables =
    params.direction === 'older'
      ? { first: params.pageSize, after: params.cursor }
      : { last: params.pageSize, before: params.cursor };

  const where: Record<string, any> = { ...params.where };

  const data = await graphqlRequest<{ requests: RequestConnection }>(
    query,
    { ...variables, where: Object.keys(where).length > 0 ? where : undefined, orderBy: { field: 'CREATED_AT', direction: 'DESC' } }
  );
  const result = requestConnectionSchema.parse(data?.requests);
  return { requests: result.edges.map((e) => e.node), pageInfo: result.pageInfo };
}

export function useRequestExecutions(
  requestID: string,
  variables?: {
    first?: number;
    after?: string;
    orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' };
    where?: Record<string, any>;
  }
) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();

  return useQuery({
    queryKey: ['request-executions', requestID, variables],
    queryFn: async () => {
      try {
        const query = buildRequestExecutionsQuery();
        const finalVariables = {
          requestID,
          ...variables,
        };
        const data = await graphqlRequest<{ node: { executions: RequestExecutionConnection } }>(query, finalVariables);
        return requestExecutionConnectionSchema.parse(data?.node?.executions);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: !!requestID,
  });
}
