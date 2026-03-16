"use client";

import {
  createContext,
  useContext,
  useEffect,
  useState,
  useCallback,
  ReactNode,
} from "react";
import { gqlClient } from "./gql-client";
import {
  getAccessToken,
  setAccessToken,
  setRefreshToken,
  clearTokens,
} from "./auth";

interface User {
  id: string;
  username: string;
  displayName: string | null;
  email: string | null;
  trustLevel: number;
  apEnabled: boolean;
}

interface AuthContextValue {
  user: User | null;
  loading: boolean;
  login: (accessToken: string, refreshToken: string, user: User) => void;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue>({
  user: null,
  loading: true,
  login: () => {},
  logout: () => {},
});

const ME_QUERY = `
  query Me {
    me {
      id
      username
      displayName
      email
      trustLevel
      apEnabled
    }
  }
`;

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const token = getAccessToken();
    if (!token) {
      setLoading(false);
      return;
    }
    gqlClient<{ me: User | null }>(ME_QUERY)
      .then((data) => setUser(data.me))
      .catch(() => clearTokens())
      .finally(() => setLoading(false));
  }, []);

  const login = useCallback(
    (accessToken: string, refreshToken: string, u: User) => {
      setAccessToken(accessToken);
      setRefreshToken(refreshToken);
      setUser(u);
    },
    []
  );

  const logout = useCallback(() => {
    clearTokens();
    setUser(null);
  }, []);

  return (
    <AuthContext.Provider value={{ user, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}
