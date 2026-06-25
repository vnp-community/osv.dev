// Base API response types

export interface APIError {
  error: string;      // Machine-readable: "NOT_FOUND", "UNAUTHORIZED"
  message: string;    // Human-readable
  details?: unknown;
  traceId?: string;   // For support tracking
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  pageSize: number;
}
