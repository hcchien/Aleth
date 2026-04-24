"use client";

import React, { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';

import { UserLevelBadge } from '@/components/auth/UserLevelBadge';

import { Post } from '@/lib/api';

type PostWithTrustTier = Post & { trustTier?: number };

function normalizeTrustTier(value?: number): 0 | 1 | 2 | 3 | 4 {
    switch (value) {
        case 0:
        case 1:
        case 2:
        case 3:
        case 4:
            return value;
        default:
            return 1;
    }
}

function formatRelativeTime(ts: number) {
    const diff = Date.now() - ts; // Assuming ts from API is epoch ms, if seconds then ts * 1000
    const diffSecs = Math.floor(diff / 1000);
    if (diffSecs < 60) return `${diffSecs} seconds ago`;
    const diffMins = Math.floor(diffSecs / 60);
    if (diffMins < 60) return `${diffMins} mins ago`;
    const diffHours = Math.floor(diffMins / 60);
    if (diffHours < 24) return `${diffHours} hours ago`;
    return `${Math.floor(diffHours / 24)} days ago`;
}

function formatFullDate(ts: number) {
    const date = new Date(ts);
    return date.toLocaleString('zh-TW', { year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit' });
}

// VaultTransition demonstrates the "Flip-Note" shared element transition
// from the Public Board card into the full Personal Vault BBS view.
export default function VaultTransition({ post }: { post: Post }) {
    const [isOpen, setIsOpen] = useState(false);
    const postWithTrustTier = post as PostWithTrustTier;
    const trustTier = normalizeTrustTier(postWithTrustTier.trustTier);

    return (
        <>
            {/* 1. Public Board Card (The Trigger) */}
            <motion.div
                layoutId={`card-container-${post.id}`}
                className="group/card py-5 px-4 -mx-4 border-b border-neutral-900/50 transition-all duration-300 hover:bg-[#1a1a1a] cursor-pointer"
                onClick={() => setIsOpen(true)}
            >
                <div className="flex items-start justify-between mb-3">
                    <div className="flex items-center gap-3">
                        <div className="w-8 h-8 rounded-full bg-neutral-800 flex items-center justify-center text-sm font-bold text-neutral-300">
                            {post.authorDid.replace('did:vflow:', '').charAt(0).toUpperCase()}
                        </div>
                        <div className="flex items-center gap-2">
                            <span className="font-medium text-neutral-200 text-sm">{post.authorDid.replace('did:vflow:', '')}</span>
                            <svg className="w-3.5 h-3.5 text-[#d97706]" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" /></svg>
                            <UserLevelBadge level={trustTier} />
                            <span className="text-neutral-500 text-xs mx-1">·</span>
                            <span className="text-neutral-500 text-xs">{formatRelativeTime(post.timestamp)}</span>
                        </div>
                    </div>
                    <div>
                        <svg className="w-4 h-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" /></svg>
                    </div>
                </div>

                <motion.h2 layoutId={`card-title-${post.id}`} className="font-serif text-[22px] font-medium text-neutral-100 mb-2 tracking-wide group-hover/card:text-[#d97706] transition-colors">
                    {post.body.split('\n')[0].substring(0, 50)}...
                </motion.h2>

                <motion.p layoutId={`card-content-${post.id}`} className="text-sm text-neutral-400 line-clamp-2 mb-4 leading-relaxed tracking-wide">
                    {post.body.split('\n').slice(1).join('\n') || post.body}
                </motion.p>

                {/* Tags */}
                <div className="flex items-center gap-2 mb-5">
                    <span className="text-xs px-2.5 py-1 rounded-sm bg-neutral-800 text-neutral-400 font-medium">公告</span>
                    <span className="text-xs px-2.5 py-1 rounded-sm bg-neutral-800 text-neutral-400 font-medium">入門</span>
                </div>

                {/* Interactions */}
                <div className="flex items-center justify-between border-t border-neutral-900/50 pt-4">
                    <div className="flex items-center gap-2">
                        <button className="flex items-center gap-1.5 px-3 py-1 rounded-full border border-neutral-800 bg-neutral-900/50 hover:bg-neutral-800 hover:border-neutral-700 transition-colors text-xs text-neutral-400">
                            <span>👍</span> {Math.floor((post.visibilityScore || 0) * 10) || 2}
                        </button>
                        <button className="flex items-center gap-1.5 px-3 py-1 rounded-full border border-neutral-800 bg-neutral-900/50 hover:bg-neutral-800 hover:border-neutral-700 transition-colors text-xs text-neutral-400">
                            <span>❤️</span> {Math.floor((post.visibilityScore || 0) * 5) || 1}
                        </button>
                    </div>
                    <div className="flex items-center gap-5 text-neutral-500">
                        <button className="flex items-center gap-1.5 hover:text-neutral-300 transition-colors">
                            <svg className="w-[18px] h-[18px]" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" /></svg>
                            <span className="text-xs">1</span>
                        </button>
                        <button className="hover:text-neutral-300 transition-colors">
                            <svg className="w-[18px] h-[18px]" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M5 5a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 21V5z" /></svg>
                        </button>
                    </div>
                </div>
            </motion.div>

            {/* 2. Personal Vault Modal (The Expanded State) */}
            <AnimatePresence>
                {isOpen && (
                    <motion.div
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        className="fixed inset-0 z-50 flex items-center justify-center p-4 md:p-12 bg-black/90 backdrop-blur-sm overflow-y-auto"
                        onClick={() => setIsOpen(false)}
                    >
                        <motion.div
                            layoutId={`card-container-${post.id}`}
                            className="bg-[#0a0a0a] border border-neutral-800 w-full max-w-4xl rounded-2xl overflow-hidden shadow-2xl flex flex-col relative cursor-default"
                            onClick={(e) => e.stopPropagation()}
                        >
                            {/* Vault / Article Content */}
                            <motion.div layoutId={`card-content-${post.id}`} className="w-full h-[85vh] overflow-y-auto px-8 py-10 flex flex-col">
                                {/* Top Nav */}
                                <div className="flex items-center gap-3 text-sm text-neutral-400 mb-8 font-medium cursor-pointer hover:text-white transition-colors w-min whitespace-nowrap" onClick={() => setIsOpen(false)}>
                                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" /></svg>
                                    返回列表
                                </div>

                                {/* Tags */}
                                <div className="flex items-center gap-2 mb-6">
                                    <span className="text-xs px-2 py-0.5 rounded text-[#d97706] font-medium tracking-wider">公告</span>
                                </div>

                                {/* Header block */}
                                <div className="flex items-start justify-between mb-8 border-b border-neutral-800/60 pb-8">
                                    <div className="flex-1">
                                        <h1 className="font-serif text-[32px] font-medium text-neutral-100 tracking-wide mb-6">
                                            歡迎來到去中心化論壇
                                        </h1>
                                        <div className="flex items-center gap-3">
                                            <div className="flex items-center gap-2 bg-neutral-900 px-2 py-1.5 rounded-lg border border-neutral-800/80">
                                                <span className="text-[#d97706] font-bold text-sm ml-1">{post.authorDid.replace('did:vflow:', '').charAt(0).toUpperCase()}</span>
                                            </div>
                                            <div className="flex flex-col justify-center">
                                                <div className="flex items-center gap-3 mb-1">
                                                    <span className="font-medium text-neutral-200 text-sm">{post.authorDid.replace('did:vflow:', '')}</span>
                                                    <UserLevelBadge level={trustTier} />
                                                    <span className="text-xs text-neutral-500 font-mono tracking-wider">0x1234...5678</span>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                    <div className="flex flex-col items-end gap-3 mt-1">
                                        <svg className="w-5 h-5 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" /></svg>
                                        <div className="flex items-center text-xs text-neutral-500 gap-2">
                                            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                                            <span>{formatFullDate(post.timestamp)}</span>
                                        </div>
                                    </div>
                                </div>

                                {/* Main Content */}
                                <div className="text-base text-neutral-300 leading-8 whitespace-pre-wrap tracking-wide mb-10">
                                    這是一個基於數位錢包身份驗證的本地優先論壇。您的資料存儲在本地設備上，並在網路連接時與伺服器同步。
                                    <br /><br />
                                    {post.body}
                                </div>

                                {/* Hide static image for now until media hashes are supported visually */}
                                {post.mediaHashes && post.mediaHashes.length > 0 && (
                                    <div className="w-full mb-10 rounded-xl overflow-hidden border border-neutral-800">
                                        <div className="w-full h-48 bg-neutral-900/50 flex items-center justify-center text-neutral-500 border border-neutral-800">
                                            [Media placeholder for hash: {post.mediaHashes[0].substring(0, 16)}...]
                                        </div>
                                    </div>
                                )}

                                {/* Reactions */}
                                <div className="flex items-center gap-2 mb-12 border-b border-neutral-800/60 pb-8">
                                    <button className="flex items-center gap-1.5 px-4 py-1.5 rounded-full border border-neutral-700 bg-neutral-800/40 hover:bg-neutral-800 hover:border-neutral-600 transition-colors text-sm text-neutral-300">
                                        <span>👍</span> {Math.floor((post.visibilityScore || 0) * 10) || 2}
                                    </button>
                                    <button className="flex items-center gap-1.5 px-4 py-1.5 rounded-full border border-neutral-700 bg-neutral-800/40 hover:bg-neutral-800 hover:border-neutral-600 transition-colors text-sm text-neutral-300">
                                        <span>❤️</span> {Math.floor((post.visibilityScore || 0) * 5) || 1}
                                    </button>
                                </div>

                                {/* Comments Section */}
                                <div className="mt-4">
                                    <h2 className="font-serif text-2xl font-bold text-neutral-100 mb-8 tracking-wide">回覆 (1)</h2>
                                    
                                    <div className="space-y-8">
                                        {/* Comment Item */}
                                        <div className="flex items-start gap-4">
                                            <div className="w-10 h-10 rounded-full bg-neutral-800 flex items-center justify-center text-sm font-bold text-[#d97706] border border-[#d97706]/20">
                                                E
                                            </div>
                                            <div className="flex-1">
                                                <div className="flex items-center justify-between mb-3">
                                                    <div className="flex items-center gap-3">
                                                        <span className="font-medium text-neutral-200 text-sm">Early_User</span>
                                                        <span className="text-[10px] px-1.5 py-0.5 rounded-full border border-teal-500/30 text-teal-400 flex items-center gap-1 font-mono tracking-wider bg-teal-500/10">🛡 L1</span>
                                                        <span className="text-neutral-500 text-xs">·</span>
                                                        <span className="text-neutral-500 text-xs">2026年3月12日 下午05:39</span>
                                                    </div>
                                                    <svg className="w-4 h-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" /></svg>
                                                </div>
                                                <div className="text-neutral-300 text-sm leading-relaxed mb-4">
                                                    很高興加入這個社群！
                                                </div>
                                                <div className="flex items-center justify-between">
                                                    <div className="flex items-center gap-2">
                                                        <button className="flex items-center gap-1.5 px-3 py-1 rounded-full border border-neutral-800 bg-neutral-900/50 hover:bg-neutral-800 hover:border-neutral-700 transition-colors text-xs text-neutral-400">
                                                            <span>👍</span> 1
                                                        </button>
                                                        <button className="flex items-center gap-1.5 px-3 py-1 rounded-full border border-neutral-800 bg-neutral-900/50 hover:bg-neutral-800 hover:border-neutral-700 transition-colors text-xs text-neutral-400">
                                                            <span>❤️</span> 1
                                                        </button>
                                                    </div>
                                                    <div className="flex items-center gap-4 text-neutral-500">
                                                        <button className="flex items-center gap-1.5 hover:text-neutral-300 transition-colors">
                                                            <svg className="w-[18px] h-[18px]" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" /></svg>
                                                            <span className="text-xs">0</span>
                                                        </button>
                                                        <button className="hover:text-neutral-300 transition-colors">
                                                            <svg className="w-[18px] h-[18px]" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M5 5a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 21V5z" /></svg>
                                                        </button>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </motion.div>
                        </motion.div>
                    </motion.div>
                )}
            </AnimatePresence>
        </>
    );
}
