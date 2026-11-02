'use client';

import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { passwordConfirmationSchema } from '@/lib/validation';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { User, CreateUserInput, UpdateUserInput } from '../data/schema';
import { useCreateUser, useUpdateUser } from '../data/users';

// 创建表单验证模式的工厂函数
const createFormSchema = (t: (key: string) => string) =>
  z
    .object({
      firstName: z.string().min(1, t('users.validation.firstNameRequired')),
      lastName: z.string().min(1, t('users.validation.lastNameRequired')),
      email: z.email().min(1, t('users.validation.emailRequired')),
      password: z.string().optional(),
      confirmPassword: z.string().optional(),
      isOwner: z.boolean().optional(),
    })
    .superRefine((data, ctx) => {
      // 只在创建用户且提供了密码时验证
      if (data.password || data.confirmPassword) {
        // Validate password against shared rules
        const passwordValidation = passwordConfirmationSchema(t).safeParse({
          password: data.password || '',
          confirmPassword: data.confirmPassword || '',
        });

        if (!passwordValidation.success) {
          passwordValidation.error.issues.forEach((issue) => {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              message: issue.message,
              path: issue.path,
            });
          });
        }
      }
    });

interface Props {
  currentRow?: User;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function UsersActionDialog({ currentRow, open, onOpenChange }: Props) {
  const { t } = useTranslation();
  const isEdit = !!currentRow;

  const createUser = useCreateUser();
  const updateUser = useUpdateUser();

  // 创建表单验证模式
  const formSchema = createFormSchema(t);
  type UserForm = z.infer<typeof formSchema>;

  // 根据是否为编辑模式使用不同的表单配置
  const form = useForm<UserForm>({
    resolver: zodResolver(formSchema),
    defaultValues: isEdit
      ? {
          firstName: currentRow.firstName,
          lastName: currentRow.lastName,
          email: currentRow.email,
          password: '',
          confirmPassword: '',
          isOwner: currentRow.isOwner,
        }
      : {
          firstName: '',
          lastName: '',
          email: '',
          password: '',
          confirmPassword: '',
          isOwner: false,
        },
  });

  const onSubmit = async (values: UserForm) => {
    try {
      if (isEdit && currentRow) {
        const updateInput: UpdateUserInput = {
          firstName: values.firstName,
          lastName: values.lastName,
          email: values.email,
          isOwner: values.isOwner,
        };

        await updateUser.mutateAsync({
          id: currentRow.id,
          input: updateInput,
        });
      } else {
        // 创建用户时，移除 confirmPassword 字段
        const createInput: CreateUserInput = {
          firstName: values.firstName,
          lastName: values.lastName,
          email: values.email,
          password: values.password || '',
          // 注意：不包含 confirmPassword
          isOwner: values.isOwner,
        };

        await createUser.mutateAsync(createInput);
      }

      form.reset();
      onOpenChange(false);
    } catch (error) {
      toast.error(t('common.errors.userSaveFailed'));
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (!state) {
          form.reset();
        }
        onOpenChange(state);
      }}
    >
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader className='text-left'>
          <DialogTitle>{isEdit ? t('users.dialogs.edit.title') : t('users.dialogs.add.title')}</DialogTitle>
          <DialogDescription>{isEdit ? t('users.dialogs.edit.description') : t('users.dialogs.add.description')}</DialogDescription>
        </DialogHeader>

        <div className='max-h-[60vh] overflow-y-auto px-1'>
          <Form {...form}>
            <form id='user-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
              <div className='grid grid-cols-2 gap-4'>
                <FormField
                  control={form.control}
                  name='firstName'
                  render={({ field, fieldState }) => (
                    <FormItem>
                      <FormLabel>{t('users.form.firstName')}</FormLabel>
                      <FormControl>
                        <Input placeholder='John' aria-invalid={!!fieldState.error} {...field} />
                      </FormControl>
                      <div className='min-h-[1.25rem]'>
                        <FormMessage />
                      </div>
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='lastName'
                  render={({ field, fieldState }) => (
                    <FormItem>
                      <FormLabel>{t('users.form.lastName')}</FormLabel>
                      <FormControl>
                        <Input placeholder='Doe' aria-invalid={!!fieldState.error} {...field} />
                      </FormControl>
                      <div className='min-h-[1.25rem]'>
                        <FormMessage />
                      </div>
                    </FormItem>
                  )}
                />
              </div>

              <FormField
                control={form.control}
                name='email'
                render={({ field, fieldState }) => (
                  <FormItem>
                    <FormLabel>{t('users.form.email')}</FormLabel>
                    <FormControl>
                      <Input placeholder='john.doe@example.com' aria-invalid={!!fieldState.error} {...field} />
                    </FormControl>
                    <div className='min-h-[1.25rem]'>
                      <FormMessage />
                    </div>
                  </FormItem>
                )}
              />

              {/* Password fields - only show when creating new user */}
              {!isEdit && (
                <div className='grid grid-cols-2 gap-4'>
                  <FormField
                    control={form.control}
                    name='password'
                    render={({ field, fieldState }) => (
                      <FormItem>
                        <FormLabel>{t('users.form.password')}</FormLabel>
                        <FormControl>
                          <Input type='password' aria-invalid={!!fieldState.error} {...field} />
                        </FormControl>
                        <div className='min-h-[1.25rem]'>
                          <FormMessage />
                        </div>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='confirmPassword'
                    render={({ field, fieldState }) => (
                      <FormItem>
                        <FormLabel>{t('users.form.confirmPassword')}</FormLabel>
                        <FormControl>
                          <Input type='password' aria-invalid={!!fieldState.error} {...field} />
                        </FormControl>
                        <div className='min-h-[1.25rem]'>
                          <FormMessage />
                        </div>
                      </FormItem>
                    )}
                  />
                </div>
              )}

              <FormField
                control={form.control}
                name='isOwner'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-start space-y-0 space-x-3'>
                    <FormControl>
                      <Checkbox checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                    <div className='space-y-1 leading-none'>
                      <FormLabel>{t('users.form.isOwner')}</FormLabel>
                      <p className='text-muted-foreground text-sm'>{t('users.form.ownerDescription')}</p>
                    </div>
                  </FormItem>
                )}
              />
            </form>
          </Form>
        </div>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
          >
            {t('common.buttons.cancel')}
          </Button>
          <Button type='submit' form='user-form' disabled={createUser.isPending || updateUser.isPending}>
            {createUser.isPending || updateUser.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
