export interface AuditLog {
  id: number
  request_id: string
  actor_id: number
  actor_name: string
  actor_role: string
  entity_type: string
  entity_id: number
  action: string
  before_snapshot: string
  after_snapshot: string
  input_hash?: string
  algorithm_version?: string
  duration_ms?: number
  result_summary?: string
  created_at: string
}
