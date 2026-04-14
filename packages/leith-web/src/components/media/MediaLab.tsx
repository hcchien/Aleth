"use client";

import React, { useState } from 'react';

import { User } from '@/types/auth';
import { createPost } from '@/lib/api';

// Mock function to simulate client-side D-pHash calculation
// In production, this would use an off-screen canvas to read ImageData and apply discrete cosine transform.
const computeDPHash = async (_file: File): Promise<string> => {
    return new Promise((resolve) => {
        setTimeout(() => {
            // Mock hash generation
            const mockHash = Array.from({ length: 16 })
                .map(() => Math.floor(Math.random() * 16).toString(16))
                .join('');
            resolve(`dphash:${mockHash}`);
        }, 800); // Simulate processing time
    });
};

interface MediaLabProps {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    onPostCreate?: (post: any) => void;
    user?: User | null;
}

export default function MediaLab({ onPostCreate, user }: MediaLabProps) {
    const [file, setFile] = useState<File | null>(null);
    const [previewSize, setPreviewSize] = useState<{ w: number, h: number } | null>(null);
    const [isHashing, setIsHashing] = useState(false);
    const [isPosting, setIsPosting] = useState(false);
    const [computedHash, setComputedHash] = useState<string | null>(null);
    const [body, setBody] = useState("");

    const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        if (e.target.files && e.target.files[0]) {
            const selectedFile = e.target.files[0];
            setFile(selectedFile);
            setComputedHash(null);

            const img = new Image();
            img.onload = () => {
                setPreviewSize({ w: img.width, h: img.height });
            };
            img.src = URL.createObjectURL(selectedFile);
        }
    };

    const handleProcessMedia = async () => {
        if (!file) return;
        setIsHashing(true);
        try {
            const hash = await computeDPHash(file);
            setComputedHash(hash);
        } finally {
            setIsHashing(false);
        }
    };

    const handleSignAndPost = async () => {
        if (!computedHash || !body || isPosting) return;

        setIsPosting(true);
        try {
            const req = {
                body: body,
                mediaHashes: [computedHash],
                timestamp: Math.floor(Date.now() / 1000),
                authorDid: user?.address || "did:vflow:unknown",
                signature: "mock_signature_for_now", // Temporarily mocked until crypto signing is implemented
            };

            const newPost = await createPost(req);

            if (onPostCreate) {
                onPostCreate(newPost);
            }

            // Reset local state to allow another post
            setFile(null);
            setComputedHash(null);
            setBody("");
            setPreviewSize(null);
        } catch (e) {
            console.error("Failed to create post", e);
            alert("Failed to submit post to the network.");
        } finally {
            setIsPosting(false);
        }
    };

    return (
        <div className="bg-neutral-900 border border-neutral-800 rounded-2xl p-6 shadow-xl w-full max-w-2xl mx-auto my-8">
            <h2 className="text-xl font-bold text-white mb-2 flex items-center gap-2">
                <span className="text-blue-500">🧪</span>
                VeriFlow Media Lab
            </h2>
            <p className="text-sm text-neutral-400 mb-6">
                Generate immutable $pHash$ fingerprints locally on your device before network transmission.
            </p>

            {(!user || user.level < 1) ? (
                <div className="bg-neutral-950 border border-neutral-800 rounded-lg p-6 text-center">
                    <div className="text-3xl mb-3">🔒</div>
                    <p className="text-neutral-300 font-medium mb-1">
                        {!user ? '請先登入以發表文章' : '登入 L1 以上等級以發表文章'}
                    </p>
                    <p className="text-xs text-neutral-500">
                        Create a Passkey or connect with a higher trust tier to unlock posting capabilities.
                    </p>
                </div>
            ) : (
                <div className="space-y-6">
                    {/* Upload Area */}
                    <div className="border-2 border-dashed border-neutral-700 rounded-xl p-8 text-center hover:bg-neutral-800/50 transition-colors">
                        <input
                            type="file"
                            accept="image/*"
                            className="hidden"
                            id="media-upload"
                            onChange={handleFileChange}
                        />
                        <label htmlFor="media-upload" className="cursor-pointer text-blue-400 hover:text-blue-300 font-medium font-mono text-sm">
                            {file ? file.name : "+ Select Immutable Image Source"}
                        </label>

                        {previewSize && (
                            <div className="mt-2 text-xs text-neutral-500">
                                Source Resolution: {previewSize.w}x{previewSize.h}px
                            </div>
                        )}
                    </div>

                    {/* Processing Actions */}
                    {file && !computedHash && (
                        <button
                            onClick={handleProcessMedia}
                            disabled={isHashing}
                            className="w-full bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white font-medium py-3 rounded-lg flex justify-center items-center gap-2"
                        >
                            {isHashing ? (
                                <span className="animate-pulse">Computing Matrix / Local $pHash$...</span>
                            ) : (
                                "Generate Cryptographic Fingerprint"
                            )}
                        </button>
                    )}

                    {/* Post Composition */}
                    {computedHash && (
                        <div className="space-y-4 animate-in fade-in slide-in-from-bottom-4 duration-500">
                            <div className="bg-black/50 border border-green-500/30 p-4 rounded-lg flex items-center gap-3">
                                <div className="text-green-500">✓</div>
                                <div>
                                    <div className="text-xs text-neutral-500 uppercase font-bold tracking-wider mb-1">Generated $pHash$</div>
                                    <div className="font-mono text-green-400 text-sm">{computedHash}</div>
                                </div>
                            </div>

                            <textarea
                                value={body}
                                onChange={(e) => setBody(e.target.value)}
                                placeholder="What context does this immutable record provide?"
                                className="w-full h-32 bg-neutral-950 border border-neutral-800 rounded-lg p-4 text-white focus:outline-none focus:ring-2 focus:ring-blue-500/50 resize-none"
                            />

                            <button
                                onClick={handleSignAndPost}
                                disabled={isPosting}
                                className="w-full bg-white hover:bg-neutral-200 disabled:opacity-50 text-black font-bold py-3 rounded-lg transition-colors flex items-center justify-center gap-2"
                            >
                                {isPosting ? "Transmitting..." : "Sign & Broadcast Transaction"}
                            </button>
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}
