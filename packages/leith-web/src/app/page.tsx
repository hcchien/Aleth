"use client";

import React, { useState, useEffect } from 'react';
import VaultTransition from '@/components/VaultTransition';
import VerifierConsole from '@/components/admin/VerifierConsole';
import MediaLab from '@/components/media/MediaLab';
import CryptoManager from '@/components/auth/CryptoManager';
import { AuthButton } from '@/components/auth/AuthButton';
import { useAuth } from '@/hooks/useAuth';
import { fetchPosts, Post } from '@/lib/api';

export default function PublicBoard() {
  const [posts, setPosts] = useState<Post[]>([]);
  const [isLoaded, setIsLoaded] = useState(false);
  const auth = useAuth();

  // Load posts from API on mount
  useEffect(() => {
    async function load() {
      try {
        const fetchedPosts = await fetchPosts();
        setPosts(fetchedPosts);
      } catch (e) {
        console.error("Failed to load posts from API", e);
      } finally {
        setIsLoaded(true);
      }
    }
    load();
  }, []);

  const handlePostCreate = (newPost: Post) => {
    setPosts(prev => [newPost, ...prev]);
  };

  // Prevent hydration mismatch
  if (!isLoaded || !auth.isLoaded) {
    return (
      <div className="min-h-svh w-full bg-[#09090b] flex items-center justify-center">
        <div className="w-8 h-8 rounded-full border-2 border-[#ea580c] border-t-transparent animate-spin"></div>
      </div>
    );
  }

  return (
    <div className="group/sidebar-wrapper flex min-h-svh w-full bg-[#0a0a0a] text-neutral-200 font-sans">

      {/* Sidebar */}
      <aside className="fixed inset-y-0 z-10 hidden w-[260px] md:flex left-0 border-r border-neutral-800 bg-[#0a0a0a]">
        <div className="flex h-full w-full flex-col py-6 px-4">
          <div className="flex items-center justify-between mb-8 px-2">
            <h1 className="text-xl font-bold text-neutral-200 tracking-wide">追蹤</h1>
            <svg className="w-5 h-5 text-neutral-400 cursor-pointer hover:text-white transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
          </div>
          
          <nav className="space-y-1 mb-8">
            <button className="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg bg-[#b45309]/15 text-[#d97706] font-medium transition-colors">
              <svg className="w-5 h-5 opacity-80" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" /></svg>
              全部動態
            </button>
          </nav>

          <div className="mb-8">
            <h3 className="px-3 text-xs font-semibold text-neutral-500 mb-3 flex items-center gap-2">
              <svg className="w-4 h-4 opacity-70" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" /></svg>
              追蹤的人
            </h3>
            <div className="space-y-1 text-sm">
               <div className="w-full flex items-center justify-between px-3 py-2 hover:bg-[#1a1a1a] rounded-lg cursor-pointer group transition-colors">
                  <div className="flex items-center gap-3">
                      <div className="w-7 h-7 rounded-full bg-neutral-600 flex items-center justify-center text-xs font-bold text-neutral-300">F</div>
                      <span className="text-neutral-300 group-hover:text-white transition-colors">Forum_Admin</span>
                  </div>
                  <span className="text-[9px] px-1.5 py-0.5 rounded-full border border-purple-500/30 text-purple-400 flex items-center gap-1 font-mono tracking-wider">👑 L4</span>
               </div>
               <div className="w-full flex items-center justify-between px-3 py-2 hover:bg-[#1a1a1a] rounded-lg cursor-pointer group transition-colors">
                  <div className="flex items-center gap-3">
                      <div className="w-7 h-7 rounded-full bg-neutral-600 flex items-center justify-center text-xs font-bold text-neutral-300">H</div>
                      <span className="text-neutral-300 group-hover:text-white transition-colors">Helper_001</span>
                  </div>
                  <span className="text-[9px] px-1.5 py-0.5 rounded-full border border-green-500/30 text-green-400 flex items-center gap-1 font-mono tracking-wider">🛡 L2</span>
               </div>
               <div className="w-full flex items-center justify-between px-3 py-2 hover:bg-[#1a1a1a] rounded-lg cursor-pointer group transition-colors">
                  <div className="flex items-center gap-3">
                      <div className="w-7 h-7 rounded-full bg-neutral-600 flex items-center justify-center text-xs font-bold text-neutral-300">T</div>
                      <span className="text-neutral-300 group-hover:text-white transition-colors">Tech_Guru</span>
                  </div>
                  <span className="text-[9px] px-1.5 py-0.5 rounded-full border border-orange-500/30 text-orange-500 flex items-center gap-1 font-mono tracking-wider">🛡 L3</span>
               </div>
            </div>
          </div>

          <div className="mb-4">
            <div className="flex items-center justify-between px-3 mb-3">
              <h3 className="text-xs font-semibold text-neutral-500 flex items-center gap-2">
                <svg className="w-4 h-4 opacity-70" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5.882V19.24a1.76 1.76 0 01-3.417.592l-2.147-6.15M18 13a3 3 0 100-6M5.436 13.683A4.001 4.001 0 017 6h1.832c4.1 0 7.625-1.234 9.168-3v14c-1.543-1.766-5.067-3-9.168-3H7a3.988 3.988 0 01-1.564-.317z" /></svg>
                粉絲專頁
              </h3>
              <svg className="w-5 h-5 text-neutral-500 cursor-pointer hover:text-white transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M12 4v16m8-8H4" /></svg>
            </div>
            
            <div className="space-y-1 text-sm">
               <div className="w-full flex items-center justify-between px-3 py-2 hover:bg-[#1a1a1a] rounded-lg cursor-pointer group transition-colors">
                  <div className="flex items-center gap-3 text-neutral-400 group-hover:text-white transition-colors">
                      <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M11 5.882V19.24a1.76 1.76 0 01-3.417.592l-2.147-6.15M18 13a3 3 0 100-6M5.436 13.683A4.001 4.001 0 017 6h1.832c4.1 0 7.625-1.234 9.168-3v14c-1.543-1.766-5.067-3-9.168-3H7a3.988 3.988 0 01-1.564-.317z" /></svg>
                      <span className="text-neutral-300">Ansible 官方</span>
                  </div>
                  <span className="text-xs text-neutral-500 font-mono">1523</span>
               </div>
               <div className="w-full flex items-center justify-between px-3 py-2 hover:bg-[#1a1a1a] rounded-lg cursor-pointer group transition-colors">
                  <div className="flex items-center gap-3 text-neutral-400 group-hover:text-white transition-colors">
                      <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" /></svg>
                      <span className="text-neutral-300">Web3 開發筆記</span>
                  </div>
                  <span className="text-xs text-neutral-500 font-mono">892</span>
               </div>
               <div className="w-full flex items-center justify-between px-3 py-2 hover:bg-[#1a1a1a] rounded-lg cursor-pointer group transition-colors">
                  <div className="flex items-center gap-3 text-neutral-400 group-hover:text-white transition-colors">
                      <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" /></svg>
                      <span className="text-neutral-300">DeFi 觀察站</span>
                  </div>
                  <span className="text-xs text-neutral-500 font-mono">2341</span>
               </div>
            </div>
          </div>

          <div className="mt-auto pt-4 border-t border-neutral-800">
             <button className="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-neutral-400 hover:text-white hover:bg-[#1a1a1a] transition-colors">
               <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
               <span className="text-sm font-medium">管理追蹤</span>
             </button>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <div className="flex-1 md:ml-[260px] flex flex-col min-w-0 bg-[#0a0a0a]">
        <header className="sticky top-0 z-40 bg-[#0a0a0a]/90 backdrop-blur-md border-b border-neutral-800">
          <div className="w-full px-6 h-16 flex items-center justify-between">
            <div className="flex items-center gap-12">
              <h1 className="font-serif text-2xl font-bold text-[#d97706] tracking-tight ml-2">Ansible</h1>
              
              <nav className="hidden md:flex items-center h-16 gap-8">
                <div className="h-full flex items-center border-b-2 border-[#d97706] px-1 translate-y-[1px]">
                    <span className="text-xs font-bold text-[#d97706] tracking-[0.15em] uppercase">ARTICLES</span>
                </div>
                <div className="h-full flex items-center border-b-2 border-transparent hover:border-neutral-700 px-1 cursor-pointer transition-colors">
                    <span className="text-xs font-bold text-neutral-400 hover:text-neutral-200 tracking-[0.15em] uppercase transition-colors">NOTES</span>
                </div>
              </nav>
            </div>

            <div className="flex items-center gap-6">
              <div className="hidden md:flex items-center gap-5 mr-2">
                 <div className="flex items-center gap-2">
                    <div className={`w-2 h-2 rounded-full ${auth.isConnected ? 'bg-green-500 shadow-[0_0_8px_#22c55e]' : 'bg-neutral-600'} `}></div>
                    <span className="text-sm text-neutral-400">{auth.isConnected ? "已連線" : "未連線"}</span>
                 </div>
                 <svg className="w-5 h-5 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" /></svg>
                 <svg className="w-5 h-5 text-neutral-400 hover:text-neutral-200 cursor-pointer transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" /></svg>
              </div>
              
              <AuthButton
                user={auth.user}
                isConnecting={auth.isConnecting}
                onLoginAsGuest={auth.loginAsGuest}
                onLoginWithPasskey={auth.loginWithPasskey}
                onDisconnect={auth.disconnect}
                onUpgradeLevel={auth.upgradeLevel}
                shortenAddress={auth.shortenAddress}
              />
            </div>
          </div>
        </header>

        <main className="flex-1 max-w-3xl mx-auto w-full px-6 py-10">
          <div className="mb-10">
            <h1 className="font-serif text-[28px] font-medium text-neutral-100 mb-8 tracking-wide">全部動態</h1>
            
            <div className="bg-transparent border border-[#d97706]/30 rounded-lg p-3.5 cursor-pointer hover:bg-[#d97706]/5 transition-colors flex items-center justify-center text-[#d97706] text-sm group">
                <svg className="w-4 h-4 mr-2 group-hover:scale-110 transition-transform" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" /></svg>
                登入 L1 以上等級以發表文章
            </div>
            
            {auth.isConnected && auth.user?.level && auth.user?.level > 0 ? (
              <div className="mt-8">
                <MediaLab onPostCreate={handlePostCreate} user={auth.user} />
              </div>
            ) : null}
          </div>

          <div className="space-y-0">
            {posts.map((post) => (
              <VaultTransition key={post.id} post={post} />
            ))}
          </div>
        </main>
      </div>

      <VerifierConsole />
    </div>
  );
}
