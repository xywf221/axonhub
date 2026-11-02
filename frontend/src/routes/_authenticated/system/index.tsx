import { createFileRoute } from '@tanstack/react-router';
import SystemManagement from '@/features/system';
import type { SystemTabKey } from '@/features/system/components/tabs';

function SystemRoute() {
  const search = Route.useSearch();

  return <SystemManagement initialTab={search.tab as SystemTabKey | undefined} />;
}

export const Route = createFileRoute('/_authenticated/system/')({
  component: SystemRoute,
  validateSearch: (search: { tab?: SystemTabKey }) => search,
});
