import { authHandlers } from './auth.handlers';
import { dashboardHandlers } from './dashboard.handlers';
import { cveHandlers } from './cve.handlers';
import { findingHandlers } from './finding.handlers';
import { scanHandlers } from './scan.handlers';
import { assetHandlers } from './asset.handlers';
import { adminHandlers } from './admin.handlers';
import { productHandlers } from './product.handlers';
import { aiHandlers } from './ai.handlers';
import { reportHandlers } from './report.handlers';
import { notificationHandlers } from './notification.handlers';
import { integrationHandlers } from './integration.handlers';
import { profileHandlers } from './profile.handlers';
import { capecHandlers } from './capec.handlers';
import { searchHandlers } from './search.handlers';

export const handlers = [
  ...authHandlers,
  ...dashboardHandlers,
  ...cveHandlers,
  ...findingHandlers,
  ...scanHandlers,
  ...assetHandlers,
  ...adminHandlers,
  ...productHandlers,
  ...aiHandlers,
  ...reportHandlers,
  ...notificationHandlers,
  ...integrationHandlers,
  ...profileHandlers,
  ...capecHandlers,
  ...searchHandlers,
];

