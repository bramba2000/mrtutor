const API_BASE_PATH = '/api/v1'

export const UNAUTHENTICATED_CODE = 'unauthenticated'
export const INVALID_CREDENTIALS_CODE = 'invalidCredentials'
export const PRINCIPAL_CONFLICT_CODE = 'principal.conflict'

export type FieldError = {
  field: string
  message: string
}

export class ApiError extends Error {
  code: string
  status: number
  fields?: Array<FieldError>

  constructor(
    code: string,
    message: string,
    status: number,
    fields?: Array<FieldError>,
  ) {
    super(message)
    this.code = code
    this.status = status
    this.fields = fields
  }

  /** Field name -> message, for direct use as e.g. a Mantine form's `errors`. */
  fieldErrors(): Record<string, string> {
    return Object.fromEntries(
      (this.fields ?? []).map((f) => [f.field, f.message]),
    )
  }
}

export async function apiFetch<T = void>(
  url: string,
  init?: RequestInit,
): Promise<T> {
  const response = await fetch(`${API_BASE_PATH}${url}`, {
    ...init,
    credentials: 'include',
  })
  if (!response.ok) {
    throw await toApiError(response)
  }
  if (response.status === 204) {
    return undefined as T
  }
  if (response.headers.get('Content-Type')?.includes('application/json')) {
    return (await response.json()) as T
  }
  const text = await response.text()
  return (text === '' ? undefined : text) as T
}

export async function post<T = void>(
  url: string,
  body: unknown,
  init?: RequestInit,
): Promise<T> {
  return apiFetch<T>(url, {
    ...init,
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers || {}),
    },
    body: JSON.stringify(body),
  })
}

async function toApiError(resp: Response): Promise<ApiError> {
  try {
    const data = await resp.json()
    if (data && data.code && data.message) {
      return new ApiError(data.code, data.message, resp.status, data.fields)
    }
  } catch {
    // fall through: body wasn't valid JSON (e.g. a plain-text 503)
  }
  return new ApiError(
    'internal',
    `API request failed with status ${resp.status}`,
    resp.status,
  )
}
