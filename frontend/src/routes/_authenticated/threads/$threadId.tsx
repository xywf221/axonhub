import { createFileRoute } from '@tanstack/react-router';
import { ThreadDetailPage } from '@/features/threads/components';

export const Route = createFileRoute('/_authenticated/threads/$threadId')({
  component: ThreadDetailPage,
});
