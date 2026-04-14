import { useState, useCallback, useEffect } from 'react';
import { User } from '../types/auth';

export function useAuth() {
    const [user, setUserState] = useState<User | null>(null);
    const [isConnecting, setIsConnecting] = useState(false);
    const [isLoaded, setIsLoaded] = useState(false);

    // Load from localStorage on mount
    useEffect(() => {
        try {
            const saved = localStorage.getItem('leith_mock_user');
            if (saved) {
                // eslint-disable-next-line react-hooks/set-state-in-effect
                setUserState(JSON.parse(saved));
            }
        } catch (e) {
            console.error("Failed to load user from localStorage", e);
        }
        setIsLoaded(true);
    }, []);

    const setUser = useCallback((newUser: User | null | ((prev: User | null) => User | null)) => {
        setUserState(prev => {
            const resolvedUser = typeof newUser === 'function' ? newUser(prev) : newUser;
            try {
                if (resolvedUser) {
                    localStorage.setItem('leith_mock_user', JSON.stringify(resolvedUser));
                } else {
                    localStorage.removeItem('leith_mock_user');
                }
            } catch (e) {
                console.error("Failed to save user to localStorage", e);
            }
            return resolvedUser;
        });
    }, []);

    // L0: OAuth
    const loginAsGuest = useCallback(async () => {
        setIsConnecting(true);
        await new Promise(resolve => setTimeout(resolve, 800));
        const id = Math.random().toString(36).slice(2, 10);
        setUser({
            address: `guest_${id}`,
            displayName: `訪客_${id.slice(0, 4)}`,
            joinedAt: Date.now(),
            level: 0,
        });
        setIsConnecting(false);
    }, [setUser]);

    // L1: Passkey
    const loginWithPasskey = useCallback(async () => {
        setIsConnecting(true);
        await new Promise(resolve => setTimeout(resolve, 1500));
        const mockAddress = 'did:vflow:' + Array.from({ length: 40 }, () =>
            Math.floor(Math.random() * 16).toString(16)
        ).join('').slice(0, 30);
        setUser({
            address: mockAddress,
            displayName: `User_${mockAddress.slice(10, 16)}`,
            joinedAt: Date.now(),
            verified: true,
            level: 1,
        });
        setIsConnecting(false);
    }, [setUser]);

    const upgradeLevel = useCallback((newLevel: 0 | 1 | 2 | 3 | 4) => {
        setUser(prev => prev ? { ...prev, level: newLevel } : null);
    }, [setUser]);

    const disconnect = useCallback(() => {
        setUser(null);
    }, [setUser]);

    const shortenAddress = (address: string) => {
        if (address.startsWith('guest_')) return address;
        if (address.startsWith('did:vflow:')) {
            return `did:...${address.slice(-6)}`;
        }
        return `${address.slice(0, 6)}...${address.slice(-4)}`;
    };

    return {
        user,
        isConnecting,
        isConnected: !!user,
        isLoaded,
        loginAsGuest,
        loginWithPasskey,
        disconnect,
        upgradeLevel,
        shortenAddress,
    };
}
