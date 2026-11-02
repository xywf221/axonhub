import { createFileRoute } from '@tanstack/react-router';
import DataStoragesManagement from '@/features/data-storages';

export const Route = createFileRoute('/_authenticated/data-storages/')({
  component: DataStoragesManagement,
});
