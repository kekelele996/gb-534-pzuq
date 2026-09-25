export interface ApiEnvelope<T> {
  code: string
  message: string
  data: T
  request_id: string
}

export interface ApiErrorBody {
  code?: string
  message?: string
  request_id?: string
}

export interface Page<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface ListQuery {
  search?: string
  page?: number
  page_size?: number
}
