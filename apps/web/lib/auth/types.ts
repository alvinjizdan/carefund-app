/**
 * CareFund Authentication & Session Domain Types
 */

/**
 * Canonical frontend User shape.
 * All backend responses (register, login, me) are normalized into this interface.
 */
export interface User {
  id: string;
  name: string;
  email: string;
  roles: string[];
}

/**
 * Token pair returned from backend authentication operations.
 * Strictly internal to server-side BFF handlers.
 */
export interface TokenPair {
  accessToken: string;
  refreshToken: string;
}

/**
 * Raw response from Go backend POST /api/v1/auth/login
 */
export interface BackendLoginData {
  user: {
    id: string;
    name: string;
    email: string;
    roles: string[];
  };
  access_token: string;
  refresh_token: string;
}

/**
 * Raw response from Go backend POST /api/v1/auth/register
 * (Backend currently returns PascalCase UserResponse without roles)
 */
export interface BackendRegisterData {
  user: {
    ID: string;
    Email: string;
    Name: string;
    Phone?: string | null;
    IsActive?: boolean;
    CreatedAt?: string;
    UpdatedAt?: string;
  };
  access_token: string;
  refresh_token: string;
}

/**
 * Raw response from Go backend POST /api/v1/auth/refresh
 */
export interface BackendRefreshData {
  access_token: string;
  refresh_token: string;
}

/**
 * Raw response from Go backend GET /api/v1/me
 */
export interface BackendMeData {
  id: string;
  name: string;
  email: string;
  roles: string[];
}

/**
 * Standard Go backend error envelope
 */
export interface BackendErrorEnvelope {
  error: {
    code: string;
    message: string;
  };
  request_id?: string;
}

/**
 * Standard BFF public response envelopes
 */
export interface BffSuccessEnvelope<T> {
  data: T;
}

export interface BffErrorEnvelope {
  error: {
    code: string;
    message: string;
  };
  request_id: string;
}
