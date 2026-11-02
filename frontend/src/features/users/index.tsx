import React, { useState, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { Main } from '@/components/layout/main';
import { PageHeader } from '@/components/layout/page-header';
import { createColumns } from './components/users-columns';
import { UsersDialogs } from './components/users-dialogs';
import { UsersPrimaryButtons } from './components/users-primary-buttons';
import { UsersTable } from './components/users-table';
import UsersProvider from './context/users-context';
import { useUsers } from './data/users';

function UsersContent() {
  const { t } = useTranslation();
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'users-table-page-size',
  });

  // Filter states
  const [nameFilter, setNameFilter] = useState<string>('');
  const [statusFilter, setStatusFilter] = useState<string[]>([]);

  const debouncedNameFilter = useDebounce(nameFilter, 300);

  // Memoize columns to prevent infinite re-renders
  const columns = useMemo(() => createColumns(t), [t]);

  // Build where clause for API filtering
  const whereClause = (() => {
    const where: Record<string, string | string[]> = {};
    if (debouncedNameFilter) {
      where.firstNameContainsFold = debouncedNameFilter;
    }
    if (statusFilter.length > 0) {
      where.statusIn = statusFilter;
    }
    return Object.keys(where).length > 0 ? where : undefined;
  })();

  const {
    data,
    isLoading,
    error: _error,
  } = useUsers({
    ...paginationArgs,
    where: whereClause,
  });

  // Reset cursor when filters change
  React.useEffect(() => {
    resetCursor();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedNameFilter, statusFilter]);

  const handleNextPage = () => {
    if (data?.pageInfo?.hasNextPage && data?.pageInfo?.endCursor) {
      setCursors(data.pageInfo.startCursor ?? undefined, data.pageInfo.endCursor ?? undefined, 'after');
    }
  };

  const handlePreviousPage = () => {
    if (data?.pageInfo?.hasPreviousPage) {
      setCursors(data.pageInfo.startCursor ?? undefined, data.pageInfo.endCursor ?? undefined, 'before');
    }
  };

  const handlePageSizeChange = (newPageSize: number) => {
    setPageSize(newPageSize);
  };

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <UsersTable
        data={data?.edges?.map((edge) => edge.node) || []}
        columns={columns}
        loading={isLoading}
        pageInfo={data?.pageInfo}
        pageSize={pageSize}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        nameFilter={nameFilter}
        statusFilter={statusFilter}
        onNameFilterChange={setNameFilter}
        onStatusFilterChange={setStatusFilter}
      />
    </div>
  );
}

export default function UsersManagement() {
  const { t } = useTranslation();

  return (
    <UsersProvider>
      <PageHeader title={t('users.title')} description={t('users.description')} actions={<UsersPrimaryButtons />} />

      <Main fixed>
        <UsersContent />
      </Main>
      <UsersDialogs />
    </UsersProvider>
  );
}
