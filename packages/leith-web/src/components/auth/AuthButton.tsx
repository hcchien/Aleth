"use client";

import React, { useState } from 'react';
import { User, USER_LEVELS } from '../../types/auth';

interface AuthButtonProps {
    user: User | null;
    isConnecting: boolean;
    onLoginAsGuest: () => void;
    onLoginWithPasskey: () => void;
    onDisconnect: () => void;
    onUpgradeLevel: (level: 0 | 1 | 2 | 3 | 4) => void;
    shortenAddress: (address: string) => string;
}

export function AuthButton({
    user,
    isConnecting,
    onLoginAsGuest,
    onLoginWithPasskey,
    onDisconnect,
    onUpgradeLevel,
    shortenAddress,
}: AuthButtonProps) {
    const [showLoginDialog, setShowLoginDialog] = useState(false);
    const [showDropdown, setShowDropdown] = useState(false);

    if (!user) {
        return (
            <div className="relative">
                <button
                    type="button"
                    onClick={() => setShowLoginDialog(true)}
                    disabled={isConnecting}
                    className="flex items-center gap-2 border border-[#d97706]/50 text-[#d97706] hover:bg-[#d97706]/10 hover:border-[#d97706] transition-all h-9 rounded-md px-4 text-sm disabled:opacity-50"
                >
                    {isConnecting ? (
                        <span className="animate-spin inline-block w-4 h-4 border-2 border-current border-t-transparent rounded-full" />
                    ) : (
                        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M2.458 12C3.732 7.943 7.522 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" /></svg>
                    )}
                    {isConnecting ? '連線中...' : '登入'}
                </button>

                {showLoginDialog && (
                    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4">
                        <div className="bg-neutral-900 border border-neutral-800 rounded-xl p-6 w-full max-w-md shadow-2xl relative">
                            <button
                                onClick={() => setShowLoginDialog(false)}
                                className="absolute top-4 right-4 text-neutral-400 hover:text-white"
                            >
                                ✕
                            </button>

                            <h2 className="text-xl font-bold text-white mb-6">選擇登入方式</h2>

                            <div className="space-y-4">
                                {/* L0 - Google */}
                                <button
                                    type="button"
                                    onClick={(e) => {
                                        e.preventDefault();
                                        e.stopPropagation();
                                        console.log("Clicked L0 Google Login");
                                        onLoginAsGuest();
                                        setShowLoginDialog(false);
                                    }}
                                    className="w-full flex items-center gap-4 p-4 rounded-lg border border-neutral-800 hover:bg-neutral-800 transition-colors text-left"
                                >
                                    <div className="w-10 h-10 rounded-full bg-white flex items-center justify-center text-xl font-bold text-black">G</div>
                                    <div>
                                        <h3 className="text-white font-medium flex items-center gap-2">
                                            Google 登入
                                            <span className="text-[10px] bg-neutral-800 text-neutral-400 px-1.5 py-0.5 rounded">L0</span>
                                        </h3>
                                        <p className="text-xs text-neutral-400 mt-1">OAuth 快速瀏覽，可觀看與低權重互動</p>
                                    </div>
                                </button>

                                {/* L0 - Facebook */}
                                <button
                                    type="button"
                                    onClick={(e) => {
                                        e.preventDefault();
                                        e.stopPropagation();
                                        console.log("Clicked L0 Facebook Login");
                                        onLoginAsGuest();
                                        setShowLoginDialog(false);
                                    }}
                                    className="w-full flex items-center gap-4 p-4 rounded-lg border border-neutral-800 hover:bg-neutral-800 transition-colors text-left"
                                >
                                    <div className="w-10 h-10 rounded-full bg-[#1877F2] flex items-center justify-center text-xl font-bold text-white">f</div>
                                    <div>
                                        <h3 className="text-white font-medium flex items-center gap-2">
                                            Facebook 登入
                                            <span className="text-[10px] bg-neutral-800 text-neutral-400 px-1.5 py-0.5 rounded">L0</span>
                                        </h3>
                                        <p className="text-xs text-neutral-400 mt-1">OAuth 快速瀏覽，可觀看與低權重互動</p>
                                    </div>
                                </button>

                                {/* L1 */}
                                <button
                                    type="button"
                                    onClick={(e) => { e.preventDefault(); e.stopPropagation(); onLoginWithPasskey(); setShowLoginDialog(false); }}
                                    className="w-full flex items-center gap-4 p-4 rounded-lg border border-[#ea580c]/30 hover:bg-[#ea580c]/10 transition-colors text-left"
                                >
                                    <div className="w-10 h-10 rounded-full bg-[#ea580c]/20 text-[#ea580c] flex items-center justify-center text-xl">🔑</div>
                                    <div>
                                        <h3 className="text-white font-medium flex items-center gap-2">
                                            Passkey 註冊 / 登入
                                            <span className="text-[10px] bg-[#ea580c]/20 text-[#ea580c] px-1.5 py-0.5 rounded">L1</span>
                                        </h3>
                                        <p className="text-xs text-neutral-400 mt-1">WebAuthn Ed25519，可簽署發文</p>
                                    </div>
                                </button>
                            </div>

                            {/* L2-L4 info */}
                            <div className="mt-6 pt-6 border-t border-neutral-800">
                                <p className="text-xs text-neutral-500 mb-3">進階等級需透過社群升級：</p>
                                <div className="space-y-2">
                                    {[2, 3, 4].map(l => {
                                        const info = USER_LEVELS[l as 2 | 3 | 4];
                                        return (
                                            <div key={l} className="flex items-center justify-between text-xs">
                                                <span className="text-neutral-400 flex items-center gap-2">
                                                    <span className="w-1.5 h-1.5 rounded-full bg-neutral-700" />
                                                    {info.label}
                                                </span>
                                                <span className="text-neutral-500 font-mono">權重 {info.weight}</span>
                                            </div>
                                        );
                                    })}
                                </div>
                            </div>
                        </div>
                    </div>
                )}
            </div>
        );
    }

    // Logged in rendering
    return (
        <div className="relative">
            <button
                onClick={() => setShowDropdown(!showDropdown)}
                className="flex items-center gap-2 bg-neutral-900 hover:bg-neutral-800 border border-neutral-800 rounded-full pl-1 pr-3 py-1 transition-colors"
            >
                <div className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold ${user.level === 4 ? 'bg-gradient-to-tr from-purple-600 to-red-500 text-white' :
                    user.level === 0 ? 'bg-neutral-800 text-neutral-400 border border-neutral-700' :
                        'bg-gradient-to-tr from-blue-600 to-teal-500 text-white'
                    }`}>
                    {user.displayName?.charAt(0) || 'U'}
                </div>
                <span className="text-sm font-medium text-neutral-200">{shortenAddress(user.address)}</span>
            </button>

            {showDropdown && (
                <>
                    <div className="fixed inset-0 z-40" onClick={() => setShowDropdown(false)} />
                    <div className="absolute right-0 mt-2 w-64 bg-neutral-900 border border-neutral-800 rounded-xl shadow-xl z-50 overflow-hidden">
                        <div className="p-4 border-b border-neutral-800 flex flex-col items-center text-center">
                            <div className="font-bold text-white mb-1">{user.displayName}</div>
                            <div className="text-xs text-neutral-400 font-mono mb-3" title={user.address}>{shortenAddress(user.address)}</div>
                            <div className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded bg-neutral-800 border border-neutral-700 text-xs font-medium text-neutral-300">
                                L{user.level} {USER_LEVELS[user.level].label}
                                <span className="text-neutral-500 ml-1">權重 {USER_LEVELS[user.level].weight}</span>
                            </div>
                        </div>

                        {user.level < 4 && (
                            <div className="p-2 border-b border-neutral-800">
                                <div className="px-3 py-2 text-xs font-bold text-neutral-500 uppercase tracking-widest">模擬升級</div>
                                {([1, 2, 3, 4] as const).filter(l => l > user.level).map(l => (
                                    <button
                                        type="button"
                                        key={l}
                                        onClick={(e) => { e.preventDefault(); e.stopPropagation(); onUpgradeLevel(l); setShowDropdown(false); }}
                                        className="w-full text-left px-3 py-2 text-sm text-neutral-300 hover:bg-neutral-800 hover:text-white rounded-md transition-colors flex items-center justify-between"
                                    >
                                        <span>升級至 L{l}</span>
                                        <span className="text-xs text-neutral-500">{USER_LEVELS[l].label}</span>
                                    </button>
                                ))}
                            </div>
                        )}

                        <div className="p-2">
                            <button
                                onClick={() => { onDisconnect(); setShowDropdown(false); }}
                                className="w-full text-left px-3 py-2 text-sm text-red-400 hover:bg-red-400/10 rounded-md transition-colors"
                            >
                                登出
                            </button>
                        </div>
                    </div>
                </>
            )}
        </div>
    );
}
