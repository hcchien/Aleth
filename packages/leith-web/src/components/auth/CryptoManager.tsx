"use client";

import React, { useState } from 'react';

// Mock WebAuthn service wrapper
export default function CryptoManager() {
    const [identities] = useState([
        // Mock local Identity starting at L0 (OAuth Guest)
        { did: "OAuth: Google", tier: "L0", active: true }
    ]);

    const handleRegisterPasskey = async () => {
        // 1. Fetch challenge from Go API
        // 2. Local `navigator.credentials.create()`
        // 3. Send Attestation to Go API
        console.log("Mock: Registering new Passkey Identity...");
        alert("In production, this triggers FaceID/TouchID registration flow via WebAuthn API.");
    };

    const activeDid = identities.find(i => i.active);

    return (
        <div className="bg-neutral-900 border border-neutral-800 rounded-2xl p-6 shadow-xl w-full max-w-sm">
            <h2 className="text-lg font-bold text-white mb-4 flex items-center gap-2">
                <span className="text-neutral-400">🔑</span>
                Key Manager
            </h2>

            {activeDid ? (
                <div className="space-y-4">
                    <div className={`bg-black/50 border rounded-xl p-4 flex flex-col items-center text-center gap-2 ${activeDid.tier === 'L0' ? 'border-neutral-800 border-dashed' : 'border-neutral-800'}`}>
                        <div className={`w-12 h-12 rounded-full flex items-center justify-center font-bold text-lg mb-2 ${activeDid.tier === 'L4' ? 'bg-gradient-to-tr from-red-600 to-purple-600 shadow-[0_0_20px_rgba(239,68,68,0.4)]' :
                                activeDid.tier === 'L0' ? 'bg-neutral-800 text-neutral-500 border-2 border-dashed border-neutral-600' :
                                    'bg-gradient-to-tr from-blue-600 to-teal-500'
                            }`}>
                            {activeDid.tier}
                        </div>

                        <p className="text-xs text-neutral-500 font-mono tracking-wider uppercase mb-1">
                            {activeDid.tier === 'L0' ? 'Guest Identity (OAuth)' : 'Active Identity'}
                        </p>

                        {activeDid.tier === 'L0' ? (
                            <p className="text-sm font-bold text-neutral-500 truncate w-full px-2">
                                user@gmail.com
                            </p>
                        ) : (
                            <p className="text-sm font-bold text-blue-400 font-mono truncate w-full px-2" title={activeDid.did}>
                                {activeDid.did}
                            </p>
                        )}
                    </div>

                    {activeDid.tier === 'L0' ? (
                        <div className="pt-2">
                            <button
                                onClick={handleRegisterPasskey}
                                className="w-full bg-blue-600 hover:bg-blue-500 text-white font-bold py-3 rounded-lg transition-colors flex flex-col items-center justify-center gap-1 shadow-[0_0_15px_rgba(59,130,246,0.2)]"
                            >
                                <span>Upgrade to L1 (Passkey)</span>
                                <span className="text-[10px] font-normal opacity-70">Unlock posting & weighted vouching</span>
                            </button>
                        </div>
                    ) : (
                        <button className="w-full text-xs text-neutral-500 hover:text-white transition-colors border border-neutral-800 hover:border-neutral-600 rounded-lg py-2">
                            Manage Keys / Switch DID
                        </button>
                    )}
                </div>
            ) : (
                <div className="text-center py-6">
                    <p className="text-sm text-neutral-400 mb-6">No cryptographic identity found on this device.</p>
                    <button
                        onClick={handleRegisterPasskey}
                        className="w-full bg-white hover:bg-neutral-200 text-black font-bold py-3 rounded-lg transition-colors flex items-center justify-center gap-2"
                    >
                        Create Passkey (WebAuthn)
                    </button>
                </div>
            )}
        </div>
    );
}
