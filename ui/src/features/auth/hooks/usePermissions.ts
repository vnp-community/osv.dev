import { useAuthStore } from '../store/authStore';
import type { Permission } from '@/shared/types/auth';

export function usePermissions() {
  const { user } = useAuthStore();

  const hasPermission = (permission: Permission): boolean =>
    user?.permissions.includes(permission) ?? false;

  return {
    // Scan
    canCreateScan:    hasPermission('scan:create'),
    canReadScan:      hasPermission('scan:read'),
    // Findings
    canWriteFindings: hasPermission('finding:write'),
    canReadFindings:  hasPermission('finding:read'),
    // Assets
    canWriteAssets:   hasPermission('asset:write'),
    canReadAssets:    hasPermission('asset:read'),
    // Reports
    canDownloadReports: hasPermission('report:download'),
    // Admin
    canManageUsers:      hasPermission('user:manage'),
    canConfigureSystem:  hasPermission('system:configure'),
    // Agent
    canReportAsAgent:    hasPermission('agent:report'),
    // Derived
    isAdmin:    user?.role === 'admin',
    isReadonly: user?.role === 'readonly',
    // Raw checker
    hasPermission,
  };
}
