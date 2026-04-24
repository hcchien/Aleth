import { useState, useCallback, useEffect } from 'react';
import { User } from '../types/auth';
import {
    beginPasskeyLogin,
    beginPasskeyRegistration,
    finishPasskeyLogin,
    finishPasskeyRegistration,
    loginAsGuestOAuth,
} from '../lib/api';
import { credentialToJSON, parseCreationOptions, parseRequestOptions } from '../lib/webauthn';

const USER_STORAGE_KEY = 'leith_mock_user';
const LAST_PASSKEY_DID_KEY = 'leith_last_passkey_did';

export function useAuth() {
    const [user, setUserState] = useState<User | null>(null);
    const [isConnecting, setIsConnecting] = useState(false);
    const [isLoaded, setIsLoaded] = useState(false);

    // Load from localStorage on mount
    useEffect(() => {
        try {
            const saved = localStorage.getItem(USER_STORAGE_KEY);
            if (saved) {
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
                    localStorage.setItem(USER_STORAGE_KEY, JSON.stringify(resolvedUser));
                    if (resolvedUser.authMethod === 'passkey') {
                        localStorage.setItem(LAST_PASSKEY_DID_KEY, resolvedUser.address);
                    }
                } else {
                    localStorage.removeItem(USER_STORAGE_KEY);
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
        try {
            const token = Math.random().toString(36).slice(2, 10);
            const auth = await loginAsGuestOAuth("google", token);
            setUser({
                address: auth.did,
                displayName: `訪客_${token.slice(0, 4)}`,
                joinedAt: Date.now(),
                level: 0,
                authMethod: 'oauth',
                oauthToken: token,
            });
        } finally {
            setIsConnecting(false);
        }
    }, [setUser]);

    // L1: Passkey
    const loginWithPasskey = useCallback(async () => {
        setIsConnecting(true);
        try {
            if (typeof window === 'undefined' || !window.PublicKeyCredential || !navigator.credentials) {
                throw new Error('This browser does not support passkeys');
            }

            const rememberedDid = localStorage.getItem(LAST_PASSKEY_DID_KEY);
            let auth: { did: string; trustTier: number };

            if (rememberedDid) {
                try {
                    const options = await beginPasskeyLogin(rememberedDid);
                    const credential = await navigator.credentials.get(parseRequestOptions(options.publicKey));
                    if (!(credential instanceof PublicKeyCredential)) {
                        throw new Error('Passkey login was cancelled');
                    }
                    auth = await finishPasskeyLogin({
                        sessionId: options.sessionId,
                        credential: credentialToJSON(credential),
                    });
                } catch (error) {
                    console.warn('Passkey login failed, falling back to registration', error);
                    localStorage.removeItem(LAST_PASSKEY_DID_KEY);
                    const options = await beginPasskeyRegistration();
                    const credential = await navigator.credentials.create(parseCreationOptions(options.publicKey));
                    if (!(credential instanceof PublicKeyCredential)) {
                        throw new Error('Passkey registration was cancelled');
                    }
                    auth = await finishPasskeyRegistration({
                        sessionId: options.sessionId,
                        credential: credentialToJSON(credential),
                    });
                }
            } else {
                const options = await beginPasskeyRegistration();
                const credential = await navigator.credentials.create(parseCreationOptions(options.publicKey));
                if (!(credential instanceof PublicKeyCredential)) {
                    throw new Error('Passkey registration was cancelled');
                }
                auth = await finishPasskeyRegistration({
                    sessionId: options.sessionId,
                    credential: credentialToJSON(credential),
                });
            }

            setUser({
                address: auth.did,
                displayName: `User_${auth.did.slice(-6)}`,
                joinedAt: Date.now(),
                verified: true,
                level: 1,
                authMethod: 'passkey',
            });
        } finally {
            setIsConnecting(false);
        }
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
