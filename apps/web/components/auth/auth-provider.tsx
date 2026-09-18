"use client";

import React, { createContext, useContext } from "react";
import { User } from "@/lib/auth/types";
import { SessionStatus } from "@/lib/auth/session";

export interface AuthContextValue {
  user: User | null;
  status: SessionStatus;
  isAuthenticated: boolean;
}

const AuthContext = createContext<AuthContextValue>({
  user: null,
  status: "UNAUTHENTICATED",
  isAuthenticated: false,
});

export interface AuthProviderProps {
  children: React.ReactNode;
  initialUser: User | null;
  initialStatus?: SessionStatus;
}

/**
 * Lightweight Client Component AuthProvider.
 * Derives its initial authentication state directly from server-side session resolution props.
 * Does not store tokens or use external global state libraries.
 */
export function AuthProvider({
  children,
  initialUser,
  initialStatus = initialUser ? "AUTHENTICATED" : "UNAUTHENTICATED",
}: AuthProviderProps) {
  const value: AuthContextValue = {
    user: initialUser,
    status: initialStatus,
    isAuthenticated: initialStatus === "AUTHENTICATED" && initialUser !== null,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

/**
 * Hook to consume client authentication state.
 */
export function useAuth(): AuthContextValue {
  return useContext(AuthContext);
}
