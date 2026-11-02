import { z } from 'zod';

export const formSchemaFactory = (t: (key: string) => string) =>
  z.object({
    name: z.string().min(1, t('apikeys.templates.templateNameRequired')),
    description: z.string().optional(),
    profile: z.object({
      name: z.string().min(1, t('apikeys.validation.profileNameRequired')),
      modelMappings: z.array(
        z.object({
          from: z.string().min(1, t('apikeys.validation.sourceModelRequired')),
          to: z.string().min(1, t('apikeys.validation.targetModelRequired')),
        })
      ),
      channelIDs: z.array(z.number()).optional().nullable(),
      channelTags: z.array(z.string()).optional().nullable(),
      channelTagsMatchMode: z.enum(['any', 'all', 'none']),
      modelIDs: z.array(z.string()).optional().nullable(),
      loadBalanceStrategy: z.string().optional().nullable(),
      traceStickyMode: z.string().optional().nullable(),
    }),
  });

export type FormValues = z.infer<ReturnType<typeof formSchemaFactory>>;
