import type { UseQueryResult } from '@tanstack/react-query';
import { LoadingSpinner } from './LoadingSpinner';
import { ErrorState } from './ErrorState';
import { EmptyState } from './EmptyState';

interface QueryBoundaryProps<T> {
  query: UseQueryResult<T>;
  skeleton?: React.ReactNode;
  children: (data: NonNullable<T>) => React.ReactNode;
  emptyCheck?: (data: T) => boolean;
  emptyState?: React.ReactNode;
}

/**
 * Wraps a React Query result and handles loading, error, and empty states.
 * Usage:
 *   <QueryBoundary query={myQuery} skeleton={<Skeleton />}>
 *     {(data) => <MyComponent data={data} />}
 *   </QueryBoundary>
 */
export function QueryBoundary<T>({
  query,
  skeleton,
  children,
  emptyCheck,
  emptyState,
}: QueryBoundaryProps<T>) {
  if (query.isLoading) {
    return skeleton ? (
      <>{skeleton}</>
    ) : (
      <div className="flex-1 flex items-center justify-center p-12">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  if (query.isError) {
    return (
      <ErrorState
        message={
          (query.error as { message?: string })?.message ??
          'Failed to load data'
        }
        onRetry={() => query.refetch()}
      />
    );
  }

  if (!query.data) {
    return emptyState ? <>{emptyState}</> : <EmptyState />;
  }

  if (emptyCheck && emptyCheck(query.data)) {
    return emptyState ? <>{emptyState}</> : <EmptyState />;
  }

  return <>{children(query.data as NonNullable<T>)}</>;
}
