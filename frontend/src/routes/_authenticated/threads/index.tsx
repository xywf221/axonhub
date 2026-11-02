import { createFileRoute } from '@tanstack/react-router';
import ThreadsManagement from '@/features/threads';

export const Route = createFileRoute('/_authenticated/threads/')({
  component: ThreadsManagement,
});
