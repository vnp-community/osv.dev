import { Outlet, useLocation } from 'react-router';
import { Sidebar } from '@/app/components/Sidebar';
import { Topbar } from '@/app/components/Topbar';
import { GlobalSearch } from '@/app/components/GlobalSearch';

/** Derive human-readable breadcrumbs from the current pathname */
function useBreadcrumbs(): string[] {
  const { pathname } = useLocation();
  const segments = pathname.split('/').filter(Boolean);
  if (segments.length === 0) return ['Dashboard'];
  return segments.map((seg) =>
    seg.charAt(0).toUpperCase() + seg.slice(1).replace(/-/g, ' ')
  );
}

export function AppLayout() {
  const breadcrumbs = useBreadcrumbs();

  return (
    <div
      className="w-full h-screen flex flex-col overflow-hidden relative"
      style={{ background: '#0B1020', fontFamily: "'Inter', sans-serif" }}
    >
      <div className="flex flex-1 overflow-hidden">
        <Sidebar />
        <div className="flex flex-col flex-1 overflow-hidden">
          <Topbar breadcrumbs={breadcrumbs} />
          <main className="flex-1 overflow-hidden">
            <Outlet />
          </main>
        </div>
      </div>
      
      {/* Global Search Modal Overlay */}
      <GlobalSearch />
    </div>
  );
}
