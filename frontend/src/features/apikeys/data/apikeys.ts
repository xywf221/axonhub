import { z } from 'zod';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient, keepPreviousData } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useErrorHandler } from '@/hooks/use-error-handler';
import type {
  ApiKey,
  ApiKeyConnection,
  ApiKeyProfileTemplate,
  ApiKeyTokenUsageStats,
  CreateApiKeyInput,
  CreateApiKeyProfileTemplateInput,
  UpdateApiKeyInput,
  UpdateApiKeyProfileTemplateInput,
  UpdateApiKeyProfilesInput,
} from './schema';
import {
  apiKeyConnectionSchema,
  apiKeyStatusSchema,
  apiKeyProfileTemplateSchema,
  apiKeySchema,
  apiKeyTokenUsageStatsSchema,
} from './schema';

const NOAUTH_API_KEY_TYPE = 'noauth';

// Dynamic GraphQL query builders
function buildApiKeysQuery() {
  return `
    query GetApiKeys($first: Int, $after: Cursor, $orderBy: APIKeyOrder, $where: APIKeyWhereInput) {
      apiKeys(first: $first, after: $after, orderBy: $orderBy, where: $where) {
        edges {
          node {
            id
            createdAt
            updatedAt
            user {
              id
              firstName
              lastName
            }
            key
            name
            type
            status
            scopes
            profiles {
              activeProfile
              profiles {
                name
                templateID
                templateName
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
        totalCount
      }
    }
  `;
}

function buildApiKeyQuery() {
  return `
    query GetApiKey($id: ID!) {
      node(id: $id) {
        ... on APIKey {
        id
        createdAt
        updatedAt
        user {
          id
          firstName
          lastName
        }
        key
        name
        type
        status
        scopes
        profiles {
          activeProfile
          profiles {
            name
            templateID
            templateName
            modelMappings { from to }
            channelIDs
            channelTags
            channelTagsMatchMode
            modelIDs
            loadBalanceStrategy
            traceStickyMode
          }
        }
      }
    }
}
  `;
}

function buildCreateApiKeyMutation() {
  return `
    mutation CreateAPIKey($input: CreateAPIKeyInput!) {
      createAPIKey(input: $input) {
        id
        createdAt
        updatedAt
        user {
          id
          firstName
          lastName
        }
        key
        name
        type
        status
        scopes
      }
    }
  `;
}

function buildUpdateApiKeyMutation() {
  return `
    mutation UpdateAPIKey($id: ID!, $input: UpdateAPIKeyInput!) {
      updateAPIKey(id: $id, input: $input) {
        id
        createdAt
        updatedAt
        user {
          id
          firstName
          lastName
        }
        key
        name
        type
        status
        scopes
      }
    }
  `;
}

const UPDATE_APIKEY_STATUS_MUTATION = `
  mutation UpdateAPIKeyStatus($id: ID!, $status: APIKeyStatus!) {
    updateAPIKeyStatus(id: $id, status: $status) {
      id
      status
    }
  }
`;

const UPDATE_APIKEY_PROFILES_MUTATION = `
  mutation UpdateAPIKeyProfiles($id: ID!, $input: UpdateAPIKeyProfilesInput!) {
    updateAPIKeyProfiles(id: $id, input: $input) {
      id
      name
      status
      profiles {
        activeProfile
        profiles {
          name
          templateID
          templateName
          modelMappings {
            from
            to
          }
          channelIDs
          channelTags
          channelTagsMatchMode
          modelIDs
          loadBalanceStrategy
          traceStickyMode
        }
      }
    }
  }
`;

const BULK_DISABLE_APIKEYS_MUTATION = `
  mutation BulkDisableAPIKeys($ids: [ID!]!) {
    bulkDisableAPIKeys(ids: $ids)
  }
`;

const BULK_ENABLE_APIKEYS_MUTATION = `
  mutation BulkEnableAPIKeys($ids: [ID!]!) {
    bulkEnableAPIKeys(ids: $ids)
  }
`;

const BULK_ARCHIVE_APIKEYS_MUTATION = `
  mutation BulkArchiveAPIKeys($ids: [ID!]!) {
    bulkArchiveAPIKeys(ids: $ids)
  }
`;

const ROTATE_APIKEY_MUTATION = `
  mutation RotateAPIKey($id: ID!) {
    rotateAPIKey(id: $id) {
      id
      key
      name
      type
      status
      scopes
      createdAt
      updatedAt
    }
  }
`;

const APIKEY_TOKEN_USAGE_STATS_QUERY = `
  query APIKeyTokenUsageStats($input: APIKeyTokenUsageStatsInput) {
    apiKeyTokenUsageStats(input: $input) {
      apiKeyId
      inputTokens
      outputTokens
      cachedTokens
      reasoningTokens
      topModels {
        modelId
        inputTokens
        outputTokens
        cachedTokens
        reasoningTokens
      }
    }
  }
`;

const APIKEY_PROFILE_TEMPLATES_QUERY = `
  query ApiKeyProfileTemplates($where: APIKeyProfileTemplateWhereInput) {
    apiKeyProfileTemplates(first: 100, where: $where) {
      edges {
        node {
          id
          createdAt
          updatedAt
          name
          description
          linkedProfilesCount
          profile {
            name
            modelMappings { from to }
            channelIDs
            channelTags
            channelTagsMatchMode
            modelIDs
            loadBalanceStrategy
            traceStickyMode
          }
        }
      }
      totalCount
    }
  }
`;

const CREATE_APIKEY_PROFILE_TEMPLATE_MUTATION = `
  mutation CreateApiKeyProfileTemplate($input: CreateAPIKeyProfileTemplateInput!, $profile: APIKeyProfileInput!) {
    createApiKeyProfileTemplate(input: $input, profile: $profile) {
      id
      createdAt
      updatedAt
      name
      description
      linkedProfilesCount
      profile {
        name
      }
    }
  }
`;

const UPDATE_APIKEY_PROFILE_TEMPLATE_MUTATION = `
  mutation UpdateApiKeyProfileTemplate($id: ID!, $input: UpdateAPIKeyProfileTemplateInput!, $profile: APIKeyProfileInput) {
    updateApiKeyProfileTemplate(id: $id, input: $input, profile: $profile) {
      id
      createdAt
      updatedAt
      name
      description
      linkedProfilesCount
      profile {
        name
      }
    }
  }
`;

const DELETE_APIKEY_PROFILE_TEMPLATE_MUTATION = `
  mutation DeleteApiKeyProfileTemplate($id: ID!) {
    deleteApiKeyProfileTemplate(id: $id) {
      id
      name
    }
  }
`;

const LOAD_APIKEY_PROFILE_TEMPLATE_MUTATION = `
  mutation LoadApiKeyProfileTemplate($input: LoadApiKeyProfileTemplateInput!) {
    loadApiKeyProfileTemplate(input: $input) {
      id
      name
      status
      profiles {
        activeProfile
        profiles {
          name
          templateID
          templateName
          modelMappings { from to }
          channelIDs
          channelTags
          channelTagsMatchMode
          modelIDs
          loadBalanceStrategy
          traceStickyMode
        }
      }
    }
  }
`;

const API_KEY_OPTIONS_QUERY = `
  query GetAPIKeyOptions($first: Int!, $after: Cursor, $orderBy: APIKeyOrder, $where: APIKeyWhereInput) {
    apiKeys(first: $first, after: $after, orderBy: $orderBy, where: $where) {
      edges {
        node {
          id
          name
          status
        }
      }
      pageInfo {
        hasNextPage
        endCursor
      }
    }
  }
`;

const apiKeyOptionConnectionSchema = z.object({
  edges: z.array(
    z.object({
      node: z.object({
        id: z.string(),
        name: z.string(),
        status: apiKeyStatusSchema,
      }),
    })
  ),
  pageInfo: z.object({
    hasNextPage: z.boolean(),
    endCursor: z.string().optional().nullable(),
  }),
});

async function fetchAPIKeyOptions(
  variables: { first: number; after?: string; where: Record<string, unknown> }
) {
  const data = await graphqlRequest<{ apiKeys: unknown }>(
    API_KEY_OPTIONS_QUERY,
    {
      ...variables,
      orderBy: { field: 'CREATED_AT', direction: 'DESC' },
    }
  );
  return apiKeyOptionConnectionSchema.parse(data?.apiKeys);
}

// React Query hooks
export function useApiKeyOptions(options?: { search?: string; includeArchived?: boolean; enabled?: boolean }) {
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();
  const search = options?.search?.trim();
  const includeArchived = options?.includeArchived ?? false;

  return useInfiniteQuery({
    queryKey: ['apiKeys', 'options', includeArchived, search],
    queryFn: async ({ pageParam }) => {
      try {
        return await fetchAPIKeyOptions({
          first: 100,
          after: pageParam,
          where: {
            typeNotIn: [NOAUTH_API_KEY_TYPE],
            statusIn: includeArchived ? ['enabled', 'disabled', 'archived'] : ['enabled', 'disabled'],
            ...(search ? { nameContainsFold: search } : {}),
          },
        });
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => (lastPage.pageInfo.hasNextPage ? (lastPage.pageInfo.endCursor ?? undefined) : undefined),
    enabled: options?.enabled !== false,
  });
}

export function useApiKeyOptionsByIDs(ids: string[] | undefined, options?: { enabled?: boolean }) {
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['apiKeys', 'options', 'selected', ids],
    queryFn: async () => {
      try {
        return await fetchAPIKeyOptions({
          first: Math.min(ids?.length ?? 1, 1000),
          where: {
            typeNotIn: [NOAUTH_API_KEY_TYPE],
            statusIn: ['enabled', 'disabled', 'archived'],
            idIn: ids,
          },
        });
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: options?.enabled !== false && !!ids?.length,
  });
}

export function useApiKeys(
  variables?: {
    first?: number;
    after?: string;
    orderBy?: { field: 'NAME' | 'CREATED_AT' | 'UPDATED_AT'; direction: 'ASC' | 'DESC' };
    where?: {
      nameContainsFold?: string;
      status?: string;
      userID?: string;
      [key: string]: unknown;
    };
  },
  options?: {
    disableAutoFetch?: boolean;
  }
) {
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['apiKeys', variables],
    queryFn: async () => {
      try {
        const query = buildApiKeysQuery();
        const mergedVariables = {
          ...variables,
          where: {
            ...variables?.where,
            typeNotIn: [NOAUTH_API_KEY_TYPE],
          },
        };
        const data = await graphqlRequest<{ apiKeys: ApiKeyConnection }>(query, mergedVariables);
        return apiKeyConnectionSchema.parse(data?.apiKeys);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: !options?.disableAutoFetch,
  });
}

export function useApiKey(id: string) {
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['apiKey', id],
    queryFn: async () => {
      try {
        const query = buildApiKeyQuery();
        const data = await graphqlRequest<{ node: ApiKey }>(query, { id });
        return apiKeySchema.parse(data.node);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: !!id,
  });
}

export function useApiKeyTokenUsageStats(
  variables?: {
    apiKeyIds?: string[];
    createdAtGTE?: string;
    createdAtLTE?: string;
  },
  options?: {
    enabled?: boolean;
  }
) {
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['apiKeyTokenUsageStats', variables],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ apiKeyTokenUsageStats: ApiKeyTokenUsageStats[] }>(
          APIKEY_TOKEN_USAGE_STATS_QUERY,
          { input: variables && Object.keys(variables).length > 0 ? variables : undefined }
        );
        return apiKeyTokenUsageStatsSchema.array().parse(data.apiKeyTokenUsageStats);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: options?.enabled ?? true,
    placeholderData: keepPreviousData,
    staleTime: 30000, // Consider data fresh for 30 seconds
  });
}

export function useCreateApiKey() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: (input: CreateApiKeyInput) => {
      const mutation = buildCreateApiKeyMutation();
      return graphqlRequest<{ createAPIKey: ApiKey }>(mutation, { input });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      toast.success(t('apikeys.messages.createSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: t('apikeys.dialogs.create.title') });
    },
  });
}

export function useUpdateApiKey() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateApiKeyInput }) => {
      const mutation = buildUpdateApiKeyMutation();
      return graphqlRequest<{ updateAPIKey: ApiKey }>(mutation, { id, input });
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      queryClient.invalidateQueries({ queryKey: ['apiKey', variables.id] });
      toast.success(t('apikeys.messages.updateSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: t('apikeys.dialogs.edit.title') });
    },
  });
}

export function useUpdateApiKeyStatus() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: 'enabled' | 'disabled' | 'archived' }) => {
      return graphqlRequest<{ updateAPIKeyStatus: ApiKey }>(UPDATE_APIKEY_STATUS_MUTATION, { id, status });
    },
    onSuccess: (data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      queryClient.invalidateQueries({ queryKey: ['apiKey', variables.id] });
      const statusText =
        data.updateAPIKeyStatus.status === 'enabled'
          ? t('apikeys.status.enabled')
          : data.updateAPIKeyStatus.status === 'disabled'
            ? t('apikeys.status.disabled')
            : t('apikeys.status.archived');
      toast.success(t('apikeys.messages.statusUpdateSuccess', { status: statusText }));
    },
    onError: (error) => {
      handleError(error);
    },
  });
}

export function useUpdateApiKeyProfiles() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateApiKeyProfilesInput }) => {
      return graphqlRequest<{ updateAPIKeyProfiles: ApiKey }>(UPDATE_APIKEY_PROFILES_MUTATION, { id, input });
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      queryClient.invalidateQueries({ queryKey: ['apiKey', variables.id] });
      toast.success(t('apikeys.messages.profilesUpdateSuccess'));
    },
    onError: (error) => {
      handleError(error);
    },
  });
}

export function useBulkDisableApiKeys() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const data = await graphqlRequest<{ bulkDisableAPIKeys: boolean }>(BULK_DISABLE_APIKEYS_MUTATION, { ids });
      return data.bulkDisableAPIKeys;
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      toast.success(t('apikeys.messages.bulkDisableSuccess', { count: variables.length }));
    },
    onError: (error) => {
      handleError(error);
    },
  });
}

export function useBulkEnableApiKeys() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const data = await graphqlRequest<{ bulkEnableAPIKeys: boolean }>(BULK_ENABLE_APIKEYS_MUTATION, { ids });
      return data.bulkEnableAPIKeys;
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      toast.success(t('apikeys.messages.bulkEnableSuccess', { count: variables.length }));
    },
    onError: (error) => {
      handleError(error);
    },
  });
}

export function useBulkArchiveApiKeys() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const data = await graphqlRequest<{ bulkArchiveAPIKeys: boolean }>(BULK_ARCHIVE_APIKEYS_MUTATION, { ids });
      return data.bulkArchiveAPIKeys;
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      toast.success(t('apikeys.messages.bulkArchiveSuccess', { count: variables.length }));
    },
    onError: (error) => {
      handleError(error);
    },
  });
}

export function useRotateApiKey() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ rotateAPIKey: ApiKey }>(ROTATE_APIKEY_MUTATION, { id });
      return apiKeySchema.parse(data.rotateAPIKey);
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      queryClient.invalidateQueries({ queryKey: ['apiKey', variables] });
      toast.success(t('apikeys.messages.rotateSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: t('apikeys.dialogs.rotate.title') });
    },
  });
}

export function useApiKeyProfileTemplates() {
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['apiKeyProfileTemplates'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ apiKeyProfileTemplates: { edges: { node: ApiKeyProfileTemplate }[]; totalCount: number } }>(
          APIKEY_PROFILE_TEMPLATES_QUERY,
          {}
        );
        const templates = (data?.apiKeyProfileTemplates?.edges ?? []).map((e) => e.node);
        return apiKeyProfileTemplateSchema.array().parse(templates);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useCreateApiKeyProfileTemplate() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (input: CreateApiKeyProfileTemplateInput) => {
      const { profile, ...inputFields } = input;
      return graphqlRequest<{ createApiKeyProfileTemplate: ApiKeyProfileTemplate }>(
        CREATE_APIKEY_PROFILE_TEMPLATE_MUTATION,
        { input: inputFields, profile: { ...profile, name: input.name } }
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['apiKeyProfileTemplates'] });
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      queryClient.invalidateQueries({ queryKey: ['apiKey'] });
    },
  });
}

export function useUpdateApiKeyProfileTemplate() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateApiKeyProfileTemplateInput }) => {
      const { profile, ...inputFields } = input;
      const resolvedProfile = profile ? { ...profile, name: input.name ?? profile.name } : undefined;
      return graphqlRequest<{ updateApiKeyProfileTemplate: ApiKeyProfileTemplate }>(
        UPDATE_APIKEY_PROFILE_TEMPLATE_MUTATION,
        { id, input: inputFields, profile: resolvedProfile }
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['apiKeyProfileTemplates'] });
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      queryClient.invalidateQueries({ queryKey: ['apiKey'] });
    },
  });
}

export function useDeleteApiKeyProfileTemplate() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => {
      return graphqlRequest<{ deleteApiKeyProfileTemplate: { id: string; name: string } }>(
        DELETE_APIKEY_PROFILE_TEMPLATE_MUTATION,
        { id }
      );
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeyProfileTemplates'] });
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      queryClient.invalidateQueries({ queryKey: ['apiKey'] });
      toast.success(t('apikeys.templates.deleteSuccessMessage', { name: data.deleteApiKeyProfileTemplate.name }));
    },
    onError: () => {
      toast.error(t('apikeys.templates.deleteErrorMessage'));
    },
  });
}

export function useLoadApiKeyProfileTemplate() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (input: { templateID: string; apiKeyID: string }) => {
      return graphqlRequest<{ loadApiKeyProfileTemplate: ApiKey }>(LOAD_APIKEY_PROFILE_TEMPLATE_MUTATION, { input });
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['apiKeys'] });
      queryClient.invalidateQueries({ queryKey: ['apiKey', variables.apiKeyID] });
    },
  });
}
