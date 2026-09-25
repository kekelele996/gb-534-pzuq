import { api, query } from './client'
import type { Page, ListQuery } from '../types/common'
import type { AuditLog } from '../types/audit'
export const listAuditLogs = (params: ListQuery & { entity_type?: string; request_id?: string; action?: string } = {}) =>
  api<Page<AuditLog>>(`/audit-logs${query(params)}`)
