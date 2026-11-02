import { createFileRoute } from '@tanstack/react-router';
import UsageStatisticsPage from '@/features/usage-statistics';

export const Route = createFileRoute('/_authenticated/usage-stats/')({
  component: UsageStatisticsPage,
});
