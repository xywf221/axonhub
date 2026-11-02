import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { Prompt, PromptConnection, CreatePromptInput, UpdatePromptInput, promptConnectionSchema, promptSchema } from './schema';

const PROMPTS_QUERY = `
  query GetPrompts(
    $first: Int
    $after: Cursor
    $last: Int
    $before: Cursor
    $where: PromptWhereInput
    $orderBy: PromptOrder
  ) {
    prompts(first: $first, after: $after, last: $last, before: $before, where: $where, orderBy: $orderBy) {
      edges {
        node {
          id
          createdAt
          updatedAt
          name
          description
          role
          content
          status
          order
          settings {
            action {
              type
            }
            conditions {
              conditions {
                type
                modelId
                modelPattern
                apiKeyId
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
      totalCount
    }
  }
`;

const CREATE_PROMPT_MUTATION = `
  mutation CreatePrompt($input: CreatePromptInput!) {
    createPrompt(input: $input) {
      id
      createdAt
      updatedAt
      name
      description
      role
      content
      status
      order
      settings {
        action {
          type
        }
        conditions {
          conditions {
            type
            modelId
            modelPattern
            apiKeyId
          }
        }
      }
    }
  }
`;

const UPDATE_PROMPT_MUTATION = `
  mutation UpdatePrompt($id: ID!, $input: UpdatePromptInput!) {
    updatePrompt(id: $id, input: $input) {
      id
      createdAt
      updatedAt
      name
      description
      role
      content
      status
      order
      settings {
        action {
          type
        }
        conditions {
          conditions {
            type
            modelId
            modelPattern
            apiKeyId
          }
        }
      }
    }
  }
`;

const DELETE_PROMPT_MUTATION = `
  mutation DeletePrompt($id: ID!) {
    deletePrompt(id: $id)
  }
`;

const UPDATE_PROMPT_STATUS_MUTATION = `
  mutation UpdatePromptStatus($id: ID!, $status: PromptStatus!) {
    updatePromptStatus(id: $id, status: $status)
  }
`;

const BULK_DELETE_PROMPTS_MUTATION = `
  mutation BulkDeletePrompts($ids: [ID!]!) {
    bulkDeletePrompts(ids: $ids)
  }
`;

const BULK_DISABLE_PROMPTS_MUTATION = `
  mutation BulkDisablePrompts($ids: [ID!]!) {
    bulkDisablePrompts(ids: $ids)
  }
`;

const BULK_ENABLE_PROMPTS_MUTATION = `
  mutation BulkEnablePrompts($ids: [ID!]!) {
    bulkEnablePrompts(ids: $ids)
  }
`;

interface QueryPromptsArgs {
  first?: number;
  after?: string;
  last?: number;
  before?: string;
  where?: Record<string, any>;
  orderBy?: {
    field: 'CREATED_AT' | 'UPDATED_AT' | 'ORDER';
    direction: 'ASC' | 'DESC';
  };
}

export function useQueryPrompts(args: QueryPromptsArgs) {
  return useQuery({
    queryKey: ['prompts', args],
    queryFn: async () => {
      const data = await graphqlRequest<{ prompts: PromptConnection }>(PROMPTS_QUERY, args);
      return promptConnectionSchema.parse(data.prompts);
    },
  });
}

export function useCreatePrompt() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (input: CreatePromptInput) => {
      const data = await graphqlRequest<{ createPrompt: Prompt }>(CREATE_PROMPT_MUTATION, { input });
      return promptSchema.parse(data.createPrompt);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompts'] });
      toast.success(t('prompts.messages.createSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: t('prompts.dialogs.create.title') });
    },
  });
}

export function useUpdatePrompt() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdatePromptInput }) => {
      const data = await graphqlRequest<{ updatePrompt: Prompt }>(UPDATE_PROMPT_MUTATION, { id, input });
      return promptSchema.parse(data.updatePrompt);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompts'] });
      toast.success(t('prompts.messages.updateSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: t('prompts.dialogs.edit.title') });
    },
  });
}

export function useDeletePrompt() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      await graphqlRequest(DELETE_PROMPT_MUTATION, { id });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompts'] });
      toast.success(t('prompts.messages.deleteSuccess'));
    },
    onError: () => {
      toast.error(t('common.errors.internalServerError'));
    },
  });
}

export function useUpdatePromptStatus() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({ id, status }: { id: string; status: 'enabled' | 'disabled' }) => {
      await graphqlRequest(UPDATE_PROMPT_STATUS_MUTATION, { id, status });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['prompts'] });
      toast.success(t('prompts.messages.statusUpdateSuccess'));
    },
    onError: () => {
      toast.error(t('common.errors.internalServerError'));
    },
  });
}

export function useBulkDeletePrompts() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const data = await graphqlRequest<{ bulkDeletePrompts: boolean }>(BULK_DELETE_PROMPTS_MUTATION, { ids });
      return data.bulkDeletePrompts;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['prompts'] });
      toast.success(t('prompts.messages.bulkDeleteSuccess', { count: variables.length }));
    },
    onError: () => {
      toast.error(t('common.errors.internalServerError'));
    },
  });
}

export function useBulkDisablePrompts() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      const data = await graphqlRequest<{ bulkDisablePrompts: boolean }>(BULK_DISABLE_PROMPTS_MUTATION, { ids });
      return data.bulkDisablePrompts;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['prompts'] });
      toast.success(t('prompts.messages.bulkDisableSuccess', { count: variables.length }));
    },
    onError: () => {
      toast.error(t('common.errors.internalServerError'));
    },
  });
}

export function useBulkEnablePrompts() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (ids: string[]) => {
      try {
        const data = await graphqlRequest<{ bulkEnablePrompts: boolean }>(BULK_ENABLE_PROMPTS_MUTATION, { ids });
        return data.bulkEnablePrompts;
      } catch (error) {
        handleError(error, { context: 'Bulk Enable Prompts' });
        throw error;
      }
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['prompts'] });
      toast.success(t('prompts.messages.bulkEnableSuccess', { count: variables.length }));
    },
  });
}
