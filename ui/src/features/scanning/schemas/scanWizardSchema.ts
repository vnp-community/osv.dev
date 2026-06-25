import { z } from 'zod';

const ipOrHostname = z.string().regex(
  /^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$|^[a-zA-Z0-9.-]+$/,
  'Must be a valid IP, CIDR, or hostname'
);

// Keep for future field-level validation
void ipOrHostname;

export const scanWizardSchema = z.object({
  type: z.enum(['nmap_full', 'nmap_discovery', 'zap', 'import']),
  name: z.string().min(3, 'Scan name must be at least 3 characters').max(100),
  targetsRaw: z
    .string()
    .min(1, 'At least one target is required'),
  scanProfile: z.enum(['discovery', 'full', 'custom']).optional(),
  portRange: z.string().optional(),
  maxDepth: z.number().min(1).max(10).optional(),
  timeout: z.number().min(30).max(3600).optional(),
  // Use optional string to avoid zodResolver type inference issues with .default()
  frequency: z.enum(['once', 'daily', 'weekly', 'custom']).optional(),
  cronExpr: z.string().optional(),
  engagementId: z.string().optional(),
});

export type ScanWizardFormData = z.infer<typeof scanWizardSchema>;


// Parse targetsRaw string → string[]
export function parseTargets(targetsRaw: string): string[] {
  return targetsRaw
    .split(/[\n,;]/)
    .map((t) => t.trim())
    .filter(Boolean);
}
