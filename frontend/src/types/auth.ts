export type UserRole = 'admin' | 'process_scientist' | 'data_analyst' | 'reviewer' | 'auditor'

export interface SessionUser {
  id: number
  username: string
  display_name: string
  role: UserRole
}

export interface LoginRequest { username: string; password: string }
export interface LoginResponse {
  token: string
  token_type: string
  expires_at: string
  user: SessionUser
}
