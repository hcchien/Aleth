"use client";

import React, { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';

type FlaggedCase = {
    id: string;
    targetId: string; // Post ID or UID
    reporterDid: string;
    reason: string;
    trustTier: string;
    signaturesRemaining: number;
};

// Mock data representing flagged content routed by the API Gateway to L4s
const caseQueue: FlaggedCase[] = [
    {
        id: "case_901x",
        targetId: "post_ab12",
        reporterDid: "did:vflow:q1w2...",
        reason: "A.I. Generated Deepfake - Incorrect Cryptographic Sourcing",
        trustTier: "L4_REQUIRED",
        signaturesRemaining: 2, // Needs 3 total consensus sigs
    },
    {
        id: "case_882y",
        targetId: "post_xc99",
        reporterDid: "did:vflow:v3b4...",
        reason: "Phishing Link inside Vault Payload",
        trustTier: "L4_REQUIRED",
        signaturesRemaining: 3,
    }
];

export default function VerifierConsole() {
    const [isOpen, setIsOpen] = useState(false);
    const [activeTab, setActiveTab] = useState<'slashing' | 'vcs'>('slashing');

    return (
        <>
            {/* 
        This trigger button only renders in the main layout 
        if the Auth middleware confirms the active DID is L4. 
      */}
            <button
                onClick={() => setIsOpen(true)}
                className="fixed bottom-6 right-6 bg-red-900/80 hover:bg-red-800 backdrop-blur-md text-red-100 px-6 py-3 rounded-full font-bold shadow-2xl flex items-center gap-2 border border-red-500/30 z-40 transition-all hover:scale-105"
            >
                <div className="w-2 h-2 rounded-full bg-red-400 animate-pulse" />
                L4 Console
            </button>

            <AnimatePresence>
                {isOpen && (
                    <motion.div
                        initial={{ opacity: 0, y: 50 }}
                        animate={{ opacity: 1, y: 0 }}
                        exit={{ opacity: 0, y: 50 }}
                        className="fixed inset-0 z-50 flex items-end justify-center sm:items-center p-4 bg-black/60 backdrop-blur-sm"
                        onClick={() => setIsOpen(false)}
                    >
                        <motion.div
                            className="bg-neutral-950 border border-neutral-800 w-full max-w-5xl h-[85vh] rounded-3xl overflow-hidden shadow-2xl flex flex-col"
                            onClick={(e) => e.stopPropagation()}
                        >
                            <header className="px-8 py-6 border-b border-neutral-800 flex justify-between items-center bg-black/50">
                                <div>
                                    <h2 className="text-2xl font-bold text-white flex items-center gap-3">
                                        <span className="text-red-500 text-3xl">⚖️</span>
                                        VeriFlow Console
                                    </h2>
                                    <p className="text-sm text-neutral-400 font-mono mt-1">Authority Level: L4 • Active Signer: did:vflow:admin...</p>
                                </div>

                                <button
                                    className="w-10 h-10 rounded-full bg-neutral-900 hover:bg-neutral-800 text-neutral-400 flex items-center justify-center transition-colors"
                                    onClick={() => setIsOpen(false)}
                                >
                                    ✕
                                </button>
                            </header>

                            <div className="flex border-b border-neutral-800">
                                <button
                                    className={`flex-1 py-4 font-bold text-sm uppercase tracking-wider ${activeTab === 'slashing' ? 'text-red-400 border-b-2 border-red-500 bg-red-500/5' : 'text-neutral-500 hover:text-neutral-300'}`}
                                    onClick={() => setActiveTab('slashing')}
                                >
                                    Slashing Consensus (Cases)
                                </button>
                                <button
                                    className={`flex-1 py-4 font-bold text-sm uppercase tracking-wider ${activeTab === 'vcs' ? 'text-blue-400 border-b-2 border-blue-500 bg-blue-500/5' : 'text-neutral-500 hover:text-neutral-300'}`}
                                    onClick={() => setActiveTab('vcs')}
                                >
                                    VCs Issuance
                                </button>
                            </div>

                            <div className="flex-1 overflow-y-auto p-8 bg-neutral-950">
                                {activeTab === 'slashing' && (
                                    <div className="space-y-6">
                                        <div className="bg-red-950/20 border border-red-900/30 rounded-xl p-4 mb-8">
                                            <h4 className="text-red-400 font-bold mb-1">Active Cases Requiring Consensus</h4>
                                            <p className="text-sm text-neutral-400">Slashing a node requires 3 independent L4 cryptographic signatures. Proceed strictly based on fact.</p>
                                        </div>

                                        {caseQueue.map(c => (
                                            <div key={c.id} className="bg-neutral-900 border border-neutral-800 rounded-2xl p-6">
                                                <div className="flex justify-between items-start mb-6">
                                                    <div>
                                                        <div className="text-xs text-neutral-500 font-mono mb-1">Case ID: {c.id}</div>
                                                        <h3 className="text-lg font-bold text-white">{c.reason}</h3>
                                                        <div className="text-sm text-neutral-400 mt-1">Target: <span className="font-mono text-blue-400">{c.targetId}</span></div>
                                                    </div>

                                                    <div className="flex gap-2">
                                                        <div className="bg-neutral-950 border border-neutral-800 px-3 py-1.5 rounded-lg text-xs font-bold text-white flex items-center gap-2">
                                                            Signatures Req: <span className="text-red-400">{c.signaturesRemaining}</span>
                                                        </div>
                                                    </div>
                                                </div>

                                                <div className="flex gap-4">
                                                    <button className="flex-1 bg-red-600 hover:bg-red-500 text-white font-medium py-3 rounded-lg transition-colors">
                                                        Sign Strike (Slash)
                                                    </button>
                                                    <button className="flex-1 bg-neutral-800 hover:bg-neutral-700 text-neutral-200 font-medium py-3 rounded-lg transition-colors">
                                                        Dismiss Case
                                                    </button>
                                                </div>
                                            </div>
                                        ))}
                                    </div>
                                )}

                                {activeTab === 'vcs' && (
                                    <div className="flex items-center justify-center h-full text-neutral-500 text-sm">
                                        Verifiable Credentials Integration (e.g. Identity Oracles) will be connected here.
                                    </div>
                                )}
                            </div>
                        </motion.div>
                    </motion.div>
                )}
            </AnimatePresence>
        </>
    );
}
