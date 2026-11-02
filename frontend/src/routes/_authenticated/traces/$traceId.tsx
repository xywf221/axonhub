import { createFileRoute } from '@tanstack/react-router';
import TraceDetailPage from '@/features/traces/components/trace-detail-page';

export const Route = createFileRoute('/_authenticated/traces/$traceId')({
  component: TraceDetailPage,
});
