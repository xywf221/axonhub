import { createFileRoute } from '@tanstack/react-router';
import TracesManagement from '@/features/traces';

export const Route = createFileRoute('/_authenticated/traces/')({
  component: TracesManagement,
});
