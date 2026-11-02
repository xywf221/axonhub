import { createFileRoute } from '@tanstack/react-router';
import PromptsManagement from '@/features/prompts';

export const Route = createFileRoute('/_authenticated/prompts/')({
  component: PromptsManagement,
});
