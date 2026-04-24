"use client";

import { useEffect, useMemo, useState } from "react";

import { AuthButton } from "@/components/auth/AuthButton";
import VerifierConsole from "@/components/admin/VerifierConsole";
import WalletVerificationPanel from "@/components/trust/WalletVerificationPanel";
import { useAuth } from "@/hooks/useAuth";
import {
  type CapabilitySnapshot,
  type ContentItem,
  type ContentMode,
  type CreateDiscussionNodeRequest,
  type DiscussionNode,
  type ModerationActionType,
  type ParticipationPolicy,
  type TransformationJob,
  type TransformationProviderType,
  createDiscussionFork,
  fetchMe,
  fetchContentItems,
  createContentItem,
  createModerationAction,
  fetchDiscussionNodes,
  createDiscussionNode,
  createProjection,
  createTransformationJob,
  createVerificationCase,
  publishTransformationJob,
} from "@/lib/api";
import { firstLine, formatAuthor, formatDate, trustRequirementLabel, truncate } from "@/lib/ui-helpers";

type ModeConfig = {
  id: ContentMode;
  label: string;
  subtitle: string;
  accent: string;
  empty: string;
  placeholder: string;
  titleHint: string;
};

const MODE_CONFIG: ModeConfig[] = [
  {
    id: "murmur",
    label: "呢喃",
    subtitle: "低壓力、短句、尚未定型的觀察",
    accent: "bg-amber-500",
    empty: "還沒有呢喃，先丟下一個直覺或問題。",
    placeholder: "寫下一句觀察、疑問、直覺，讓它先存在。",
    titleHint: "可留空",
  },
  {
    id: "idea",
    label: "想法",
    subtitle: "作者整理過的觀點，保有個人所有權",
    accent: "bg-emerald-500",
    empty: "還沒有想法草稿，從幾條呢喃開始整理也可以。",
    placeholder: "把一個觀點寫完整：主張、背景、推理、保留。",
    titleHint: "建議加標題",
  },
  {
    id: "discussion",
    label: "討論",
    subtitle: "公共辯論層，適合被回應、反駁與 fork",
    accent: "bg-sky-500",
    empty: "還沒有公共討論，試著投出一個值得辯論的主張。",
    placeholder: "提出一個可以被公共討論的命題或立場。",
    titleHint: "建議加標題",
  },
];

const IDEA_HERO_IMAGE =
  "https://lh3.googleusercontent.com/aida-public/AB6AXuCgxXkAP9KbIj8RHZFxDylwLU7tTn_sty-k70VZKZfs06LpT5MY5BxfINcimh8x40eEuAaLLttklMRhVWi_IyN4HjVaLLxeQgnIEOieRjitRCRcsfUuTmfXObn87R869Ll0zaNp2xRiO2G6aV8dQ21VU4wHnL8os4KtJSfxWPj6n2qNz8NVpRQxJckIG9PXX6fWVy_4dYVduNl9YsVxJeeevTjh9P5vW4dXf12nxp-_JkKSE90Yl4n-pgbmxgRoA_xPeJv5SSH9P3Q";

const IDEA_INSIGHTS = [
  "基於你的近期呢喃，這篇文章還能補進『本地 AI 與公共論辯分工』的段落。",
  "這裡的『數位意志』與先前 murmur 裡的治理框架高度一致，適合補一段定義。",
];

const MURMUR_KEYWORDS = ["#建築", "#倫理", "#極簡生活", "#加密藝術", "#虛擬"];

const DISCUSSION_BOARDS = [
  "Digital Sovereignty",
  "AI Ethics",
  "Governance",
  "WebGPU Devs",
];

export default function HomePage() {
  const auth = useAuth();
  const [activeMode, setActiveMode] = useState<ContentMode>("discussion");
  const [contentByMode, setContentByMode] = useState<Record<ContentMode, ContentItem[]>>({
    murmur: [],
    idea: [],
    discussion: [],
  });
  const [sessionName, setSessionName] = useState("訪客");
  const [sessionTier, setSessionTier] = useState(0);
  const [capabilities, setCapabilities] = useState<CapabilitySnapshot | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isCreating, setIsCreating] = useState(false);
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [selectedDiscussion, setSelectedDiscussion] = useState<ContentItem | null>(null);
  const [selectedIdeaId, setSelectedIdeaId] = useState<string | null>(null);
  const [murmurView, setMurmurView] = useState<"sanctuary" | "whispers">("sanctuary");
  const [discussionNodes, setDiscussionNodes] = useState<DiscussionNode[]>([]);
  const [isLoadingNodes, setIsLoadingNodes] = useState(false);
  const [projectionIdea, setProjectionIdea] = useState<ContentItem | null>(null);
  const [projectionExcerpt, setProjectionExcerpt] = useState("");
  const [projectionPolicy, setProjectionPolicy] = useState<ParticipationPolicy>("debate");
  const [isProjecting, setIsProjecting] = useState(false);
  const [transformSource, setTransformSource] = useState<ContentItem | null>(null);
  const [transformProvider, setTransformProvider] = useState<TransformationProviderType>("local_llm");
  const [transformProfile, setTransformProfile] = useState("researcher");
  const [transformJob, setTransformJob] = useState<TransformationJob | null>(null);
  const [isTransforming, setIsTransforming] = useState(false);
  const [isPublishingTransform, setIsPublishingTransform] = useState(false);
  const [forkReason, setForkReason] = useState("");
  const [isForkDialogOpen, setIsForkDialogOpen] = useState(false);
  const [isForking, setIsForking] = useState(false);
  const [moderationReason, setModerationReason] = useState("");
  const [moderationType, setModerationType] = useState<ModerationActionType>("flag");
  const [isModerationDialogOpen, setIsModerationDialogOpen] = useState(false);
  const [isModerating, setIsModerating] = useState(false);
  const [isRequestingVerification, setIsRequestingVerification] = useState(false);
  const [replyBody, setReplyBody] = useState("");
  const [replyType, setReplyType] = useState<CreateDiscussionNodeRequest["nodeType"]>("rebuttal");
  const [replyStance, setReplyStance] = useState<CreateDiscussionNodeRequest["stance"]>("clarify");

  const activeConfig = useMemo(
    () => MODE_CONFIG.find((config) => config.id === activeMode) ?? MODE_CONFIG[0],
    [activeMode],
  );

  useEffect(() => {
    if (!auth.isLoaded) {
      return;
    }
    let cancelled = false;

    async function load() {
      setIsLoading(true);
      try {
        const [me, murmurs, ideas, discussions] = await Promise.all([
          fetchMe(auth.user),
          fetchContentItems({ mode: "murmur" }, auth.user).catch(() => []),
          fetchContentItems({ mode: "idea" }, auth.user).catch(() => []),
          fetchContentItems({ mode: "discussion", visibility: "public" }, auth.user).catch(() => []),
        ]);
        if (cancelled) {
          return;
        }
        setSessionName(me.identity.displayName);
        setSessionTier(me.identity.trustTier);
        setCapabilities(me.capabilities);
        setContentByMode({
          murmur: murmurs,
          idea: ideas,
          discussion: discussions,
        });
        if (discussions.length > 0) {
          setSelectedDiscussion((prev) => prev ?? discussions[0]);
        }
      } catch (error) {
        console.error("Failed to load v2 content", error);
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    }

    void load();
    return () => {
      cancelled = true;
    };
  }, [auth.isLoaded, auth.user]);

  useEffect(() => {
    if (activeMode !== "discussion") {
      return;
    }
    const discussions = contentByMode.discussion;
    if (!selectedDiscussion && discussions.length > 0) {
      setSelectedDiscussion(discussions[0]);
    }
  }, [activeMode, contentByMode.discussion, selectedDiscussion]);

  useEffect(() => {
    if (activeMode !== "idea") {
      return;
    }
    const ideas = contentByMode.idea;
    if (ideas.length === 0) {
      setSelectedIdeaId(null);
      return;
    }
    if (!selectedIdeaId || !ideas.some((item) => item.id === selectedIdeaId)) {
      setSelectedIdeaId(ideas[0].id);
    }
  }, [activeMode, contentByMode.idea, selectedIdeaId]);

  useEffect(() => {
    if (!selectedDiscussion) {
      setDiscussionNodes([]);
      return;
    }
    const discussion = selectedDiscussion;
    let cancelled = false;

    async function loadNodes() {
      setIsLoadingNodes(true);
      try {
        const response = await fetchDiscussionNodes(discussion.id, auth.user);
        if (!cancelled) {
          setDiscussionNodes(response.nodes);
        }
      } catch (error) {
        console.error("Failed to load discussion nodes", error);
        if (!cancelled) {
          setDiscussionNodes([]);
        }
      } finally {
        if (!cancelled) {
          setIsLoadingNodes(false);
        }
      }
    }

    void loadNodes();
    return () => {
      cancelled = true;
    };
  }, [selectedDiscussion, auth.user]);

  async function handleCreate() {
    if (!body.trim() || isCreating) {
      return;
    }
    setIsCreating(true);
    try {
      const created = await createContentItem(
        {
          mode: activeMode,
          title: title.trim() || undefined,
          body: body.trim(),
          visibility: activeMode === "discussion" ? "public" : "private",
          participationPolicy: activeMode === "discussion" ? "debate" : undefined,
          discussionShape: activeMode === "discussion" ? "thread" : undefined,
        },
        auth.user,
      );
      setContentByMode((prev) => ({
        ...prev,
        [activeMode]: [created, ...prev[activeMode]],
      }));
      if (created.mode === "discussion") {
        setSelectedDiscussion(created);
      }
      setTitle("");
      setBody("");
    } catch (error) {
      console.error("Failed to create content item", error);
      alert("建立內容失敗，請確認已登入並且 API 已啟動。");
    } finally {
      setIsCreating(false);
    }
  }

  async function handleReply() {
    if (!selectedDiscussion || !replyBody.trim()) {
      return;
    }
    try {
      const created = await createDiscussionNode(
        selectedDiscussion.id,
        {
          body: replyBody.trim(),
          nodeType: replyType,
          stance: replyStance,
        },
        auth.user,
      );
      setDiscussionNodes((prev) => [...prev, created]);
      setReplyBody("");
    } catch (error) {
      console.error("Failed to create discussion node", error);
      alert("建立討論節點失敗，請先登入。");
    }
  }

  async function handleForkDiscussion() {
    if (!selectedDiscussion || !forkReason.trim()) {
      return;
    }
    setIsForking(true);
    try {
      const fork = await createDiscussionFork(
        selectedDiscussion.id,
        { reason: forkReason.trim() },
        auth.user,
      );
      const forked = await fetchContentItems({ mode: "discussion", visibility: "public" }, auth.user)
        .then((items) => items.find((item) => item.id === fork.forkDiscussionId));
      if (forked) {
        setContentByMode((prev) => ({
          ...prev,
          discussion: [forked, ...prev.discussion.filter((item) => item.id !== forked.id)],
        }));
        setSelectedDiscussion(forked);
      }
      setForkReason("");
      setIsForkDialogOpen(false);
    } catch (error) {
      console.error("Failed to fork discussion", error);
      alert("建立 fork 失敗，通常代表信任等級不足。");
    } finally {
      setIsForking(false);
    }
  }

  async function handleModerationAction() {
    if (!selectedDiscussion || !moderationReason.trim()) {
      return;
    }
    setIsModerating(true);
    try {
      await createModerationAction(
        {
          targetContentId: selectedDiscussion.id,
          actionType: moderationType,
          reason: moderationReason.trim(),
        },
        auth.user,
      );
      setModerationReason("");
      setIsModerationDialogOpen(false);
      alert("治理動作已建立。");
    } catch (error) {
      console.error("Failed to create moderation action", error);
      alert("建立治理動作失敗，通常代表信任等級不足。");
    } finally {
      setIsModerating(false);
    }
  }

  async function handleVerificationRequest(requestedTier: 2 | 3) {
    setIsRequestingVerification(true);
    try {
      await createVerificationCase(
        {
          requestedTier,
          credentialType: requestedTier === 2 ? "social_vouch" : "contribution_history",
          evidenceJson: JSON.stringify({
            displayName: sessionName,
            did: auth.user?.address,
            requestedAt: new Date().toISOString(),
            note:
              requestedTier === 2
                ? "User requests L2 social trust review from the prototype UI."
                : "User requests L3 contribution review from the prototype UI.",
          }),
        },
        auth.user,
      );
      alert(`L${requestedTier} verification case 已送出，等待 L4 verifier review。`);
    } catch (error) {
      console.error("Failed to create verification case", error);
      alert("送出 verification case 失敗，請確認已使用 passkey 登入。");
    } finally {
      setIsRequestingVerification(false);
    }
  }

  function openProjection(item: ContentItem) {
    setProjectionIdea(item);
    setProjectionExcerpt(item.body);
    setProjectionPolicy("debate");
  }

  async function handleProjectIdea() {
    if (!projectionIdea || !projectionExcerpt.trim()) {
      return;
    }
    setIsProjecting(true);
    try {
      const projection = await createProjection(
        {
          sourceIdeaId: projectionIdea.id,
          projectedExcerpt: projectionExcerpt.trim(),
          participationPolicy: projectionPolicy,
          ownershipTransferAcknowledged: true,
          discussionShape: "thread",
        },
        auth.user,
      );
      const createdDiscussion = await fetchContentItems(
        { mode: "discussion", visibility: "public" },
        auth.user,
      ).then((items) => items.find((item) => item.id === projection.targetDiscussionId));
      if (createdDiscussion) {
        setContentByMode((prev) => ({
          ...prev,
          discussion: [createdDiscussion, ...prev.discussion.filter((item) => item.id !== createdDiscussion.id)],
        }));
        setSelectedDiscussion(createdDiscussion);
        setActiveMode("discussion");
      }
      setProjectionIdea(null);
    } catch (error) {
      console.error("Failed to project idea", error);
      alert("投射到公共討論失敗，請確認已登入且為想法作者。");
    } finally {
      setIsProjecting(false);
    }
  }

  function openTransform(item: ContentItem) {
    setTransformSource(item);
    setTransformProvider("local_llm");
    setTransformProfile("researcher");
    setTransformJob(null);
  }

  async function handleCreateTransformation() {
    if (!transformSource) {
      return;
    }
    setIsTransforming(true);
    try {
      const job = await createTransformationJob(
        {
          sourceContentIds: [transformSource.id],
          targetMode: "idea",
          providerType: transformProvider,
          promptProfile: transformProfile,
        },
        auth.user,
      );
      setTransformJob(job);
    } catch (error) {
      console.error("Failed to create transformation job", error);
      alert("建立轉譯草稿失敗，請確認已登入。");
    } finally {
      setIsTransforming(false);
    }
  }

  async function handlePublishTransformation() {
    if (!transformJob) {
      return;
    }
    setIsPublishingTransform(true);
    try {
      const created = await publishTransformationJob(transformJob.id, auth.user);
      setContentByMode((prev) => ({
        ...prev,
        [created.mode]: [created, ...prev[created.mode]],
      }));
      setActiveMode(created.mode);
      setTransformSource(null);
      setTransformJob(null);
    } catch (error) {
      console.error("Failed to publish transformation job", error);
      alert("發布轉譯草稿失敗。");
    } finally {
      setIsPublishingTransform(false);
    }
  }

  const activeItems = contentByMode[activeMode];
  const selectedIdea =
    contentByMode.idea.find((item) => item.id === selectedIdeaId) ?? contentByMode.idea[0] ?? null;
  const relatedIdeas = contentByMode.idea.slice(0, 2);
  const canFork = capabilities?.canForkDiscussion ?? false;
  const canFlag = capabilities?.canFlagContent ?? false;
  const canSlash = capabilities?.canSlashContent ?? false;
  const canRequestVerification = capabilities?.canRequestVerification ?? false;
  const canReviewVerificationCases = capabilities?.canReviewVerificationCases ?? false;
  const isIdeaMode = activeMode === "idea";
  const isMurmurMode = activeMode === "murmur";
  const isSanctuaryView = murmurView === "sanctuary";

  return (
    <div
      className={
        isIdeaMode
          ? "min-h-svh bg-[#f6f4ef] text-[#1a1c1c]"
          : isMurmurMode
            ? "min-h-svh bg-[#0e0e0e] text-[#e7e5e5]"
            : "min-h-svh bg-[#f9f9f9] text-[#1a1c1c]"
      }
    >
      <div className="mx-auto flex min-h-svh max-w-[1680px] flex-col">
        <header
          className={
            isIdeaMode
              ? "sticky top-0 z-40 border-b border-[#d9d2e3] bg-[#fbfaf7]/95 px-8 py-4 backdrop-blur"
              : isMurmurMode
                ? "sticky top-0 z-40 border-b border-[#2a2b2b] bg-[#0e0e0e]/90 px-6 py-4 backdrop-blur-3xl"
              : "sticky top-0 z-40 border-b border-[#e6e0ef] bg-white/75 px-6 py-4 backdrop-blur-xl"
          }
        >
          <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <div className="mb-2 flex items-center gap-3">
                <div
                  className={
                    isIdeaMode
                      ? "rounded-full border border-violet-200 bg-violet-50 px-3 py-1 text-xs tracking-[0.24em] text-violet-700"
                      : isMurmurMode
                        ? "rounded-full border border-[#2c3f3b] bg-[#191f1e] px-3 py-1 text-xs tracking-[0.24em] text-[#73d9b5]"
                      : "rounded-full border border-violet-200 bg-violet-50 px-3 py-1 text-xs tracking-[0.24em] text-violet-700"
                  }
                >
                  ALETH
                </div>
                <div
                  className={
                    isIdeaMode
                      ? "text-xs uppercase tracking-[0.24em] text-[#6b6480]"
                      : isMurmurMode
                        ? "text-xs uppercase tracking-[0.24em] text-[#8da29d]"
                      : "text-xs uppercase tracking-[0.24em] text-[#7b7487]"
                  }
                >
                  {isIdeaMode ? "Editorial Studio / Idea Layer" : isMurmurMode ? "Murmur Layer / Local Encrypted" : "Discourse / Board Listing"}
                </div>
              </div>
              <h1
                className={
                  isIdeaMode
                    ? "text-3xl font-semibold tracking-tight text-[#17161d]"
                    : isMurmurMode
                      ? "text-3xl font-semibold tracking-tight text-[#e7e5e5]"
                    : "text-3xl font-semibold tracking-tight text-[#5a18cb]"
                }
                style={isIdeaMode || isMurmurMode ? { fontFamily: "var(--font-manrope)" } : undefined}
              >
                {isIdeaMode ? "The Fluid Privacy Framework" : isMurmurMode ? (isSanctuaryView ? "迷霧之境" : "好友呢喃") : "Discourse"}
              </h1>
              <p
                className={
                  isIdeaMode
                    ? "mt-2 max-w-3xl text-sm leading-6 text-[#625a73]"
                    : isMurmurMode
                      ? "mt-2 max-w-3xl text-sm leading-6 text-[#a6abaa]"
                    : "mt-2 max-w-3xl text-sm leading-6 text-[#5b5668]"
                }
              >
                {isIdeaMode
                  ? "Idea 視角現在是作者主場：長文、文脈、AI 洞察與投射設定，都在同一張 editorial surface 裡發生。"
                  : isMurmurMode
                    ? isSanctuaryView
                      ? "這裡是完全屬於你的本地 murmur 金庫。所有片段都先留在個人領地，再決定是否要整理、轉譯或投射。"
                      : "這裡只展示來自加密圈子與可信好友的 murmur。內容更輕、更私密，也更適合被轉譯成下一篇想法草稿。"
                  : "Board 是公共辯論的舞台。身份可信度、AI 共識摘要與社群治理在這一層共同決定討論品質。"}
              </p>
            </div>
            <div className="flex items-center gap-4 self-start lg:self-end">
              <div
                className={
                  isIdeaMode
                    ? "rounded-2xl border border-[#d9d2e3] bg-white px-4 py-3 shadow-sm"
                    : isMurmurMode
                      ? "rounded-2xl border border-[#2f3232] bg-[#171818] px-4 py-3"
                    : "rounded-2xl border border-[#e6e0ef] bg-white px-4 py-3 shadow-sm"
                }
              >
                <div className={isIdeaMode ? "text-xs tracking-[0.18em] text-[#81778f]" : isMurmurMode ? "text-xs tracking-[0.18em] text-[#8da29d]" : "text-xs tracking-[0.18em] text-[#7b7487]"}>SESSION</div>
                <div className={isIdeaMode ? "mt-1 text-sm font-medium text-[#17161d]" : isMurmurMode ? "mt-1 text-sm font-medium text-white" : "mt-1 text-sm font-medium text-[#1a1c1c]"}>{sessionName}</div>
                <div className={isIdeaMode ? "text-xs text-[#81778f]" : isMurmurMode ? "text-xs text-[#8da29d]" : "text-xs text-[#7b7487]"}>Trust Tier L{sessionTier}</div>
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

        <div
          className={
            isIdeaMode
              ? "grid flex-1 grid-cols-1 lg:grid-cols-[256px_minmax(0,1fr)]"
              : isMurmurMode
                ? "grid flex-1 grid-cols-1 lg:grid-cols-[256px_minmax(0,1fr)_320px]"
                : "grid flex-1 grid-cols-1 gap-0 lg:grid-cols-[256px_minmax(0,1fr)_320px]"
          }
        >
          <aside
            className={
              isIdeaMode
                ? "border-r border-[#ddd6e7] bg-[#f8f6f1] px-5 py-6"
                : isMurmurMode
                  ? "border-r border-[#1f2020] bg-[#0e0e0e] px-4 py-6"
                  : "border-r border-[#ece7f4] bg-[#f8f8f8] px-4 py-6"
            }
          >
            <div className="mb-6">
              <div className={isIdeaMode ? "text-xs uppercase tracking-[0.22em] text-[#81778f]" : isMurmurMode ? "px-4 text-xs uppercase tracking-[0.22em] text-[#8da29d]" : "text-xs uppercase tracking-[0.22em] text-slate-500"}>
                {isIdeaMode ? "Editorial Studio" : isMurmurMode ? "Murmur Layer" : "Perspectives"}
              </div>
              <div className="mt-4 space-y-2">
                {MODE_CONFIG.map((mode) => {
                  const isActive = activeMode === mode.id;
                  return (
                    <button
                      key={mode.id}
                      type="button"
                      onClick={() => setActiveMode(mode.id)}
                      className={
                        isIdeaMode
                          ? `w-full rounded-2xl border px-4 py-4 text-left transition ${
                              isActive
                                ? "border-[#d5c7ef] bg-white text-violet-700 shadow-sm"
                                : "border-transparent bg-transparent text-[#6f6780] hover:bg-white/80"
                            }`
                          : isMurmurMode
                            ? `w-full rounded-full px-4 py-3 text-left transition ${
                                isActive
                                  ? "bg-[#2c3f3b] text-[#73d9b5]"
                                  : "text-[#8da29d] hover:bg-[#191a1a]"
                              }`
                          : `w-full rounded-2xl border px-4 py-4 text-left transition ${
                              isActive
                                ? "border-white/20 bg-white/10"
                                : "border-white/5 bg-white/[0.03] hover:border-white/10 hover:bg-white/[0.06]"
                            }`
                      }
                    >
                      <div className="flex items-center gap-3">
                        <div className={`h-2.5 w-2.5 rounded-full ${mode.accent}`} />
                        <div className={isIdeaMode ? "text-base font-medium text-current" : isMurmurMode ? "text-base font-medium text-current" : "text-base font-medium text-white"}>{mode.label}</div>
                      </div>
                      <div className={isIdeaMode ? "mt-2 text-sm leading-6 text-[#81778f]" : isMurmurMode ? "mt-2 text-sm leading-6 text-[#7f918d]" : "mt-2 text-sm leading-6 text-slate-400"}>{mode.subtitle}</div>
                    </button>
                  );
                })}
              </div>
            </div>

            {isIdeaMode ? (
              <>
                <div className="mb-8 rounded-3xl border border-[#ddd6e7] bg-white p-5 shadow-sm">
                  <div className="text-[11px] uppercase tracking-[0.24em] text-[#81778f]">Library</div>
                  <div className="mt-4 space-y-2">
                    {contentByMode.idea.length === 0 ? (
                      <div className="rounded-2xl bg-[#f6f2fb] px-4 py-5 text-sm leading-6 text-[#6a617c]">
                        {activeConfig.empty}
                      </div>
                    ) : (
                      contentByMode.idea.map((item) => (
                        <button
                          key={item.id}
                          type="button"
                          onClick={() => setSelectedIdeaId(item.id)}
                          className={`w-full rounded-2xl px-4 py-4 text-left transition ${
                            selectedIdea?.id === item.id
                              ? "bg-[#f3edff] text-[#5327b5]"
                              : "bg-[#faf8fd] text-[#625a73] hover:bg-[#f4f0fb]"
                          }`}
                        >
                          <div className="text-[11px] uppercase tracking-[0.24em] text-[#90869f]">
                            {formatDate(item.createdAt)}
                          </div>
                          <div className="mt-2 font-medium">{item.title?.trim() || firstLine(item.body)}</div>
                          <div className="mt-2 text-sm leading-6 text-[#7a718a]">{truncate(item.body, 72)}</div>
                        </button>
                      ))
                    )}
                  </div>
                </div>

                <div className="rounded-3xl bg-[#6424d8] px-5 py-4 text-white shadow-[0_14px_40px_rgba(100,36,216,0.18)]">
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-[11px] uppercase tracking-[0.24em] text-violet-200">New Manuscript</div>
                      <div className="mt-2 text-sm text-violet-100">把近期論點整理成一篇可投射的想法文章。</div>
                    </div>
                    <div className="text-2xl">+</div>
                  </div>
                </div>

                <div className="mt-8 rounded-3xl border border-[#ddd6e7] bg-white p-5 shadow-sm">
                  <div className="text-[11px] uppercase tracking-[0.24em] text-[#81778f]">Compose</div>
                  <div className="mt-4 space-y-3">
                    <input
                      value={title}
                      onChange={(event) => setTitle(event.target.value)}
                      placeholder="標題 建議加標題"
                      className="w-full rounded-2xl border border-[#ddd6e7] bg-[#fcfbf8] px-4 py-3 text-sm text-[#17161d] outline-none placeholder:text-[#978ea7]"
                    />
                    <textarea
                      value={body}
                      onChange={(event) => setBody(event.target.value)}
                      placeholder={activeConfig.placeholder}
                      className="min-h-44 w-full rounded-2xl border border-[#ddd6e7] bg-[#fcfbf8] px-4 py-3 text-sm leading-7 text-[#17161d] outline-none placeholder:text-[#978ea7]"
                    />
                    <button
                      type="button"
                      onClick={() => void handleCreate()}
                      disabled={isCreating || !body.trim()}
                      className="w-full rounded-2xl bg-[#6424d8] px-4 py-3 text-sm font-semibold text-white transition hover:bg-[#5620b7] disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {isCreating ? "建立中..." : "建立想法"}
                    </button>
                  </div>
                </div>
              </>
            ) : isMurmurMode ? (
              <>
                <div className="mt-8 rounded-[28px] border border-[#252626] bg-[#131313] p-5">
                  <div className="text-[11px] uppercase tracking-[0.24em] text-[#8da29d]">Sanctuary</div>
                  <p className="mt-3 text-sm leading-6 text-[#a5aaa9]">
                    你的 murmur 只在加密圈與可信節點之間流動。這一層偏輕、偏私密，也最適合做下一步 AI 轉譯。
                  </p>
                  <div className="mt-4 rounded-full border border-[#2c3f3b] bg-[#161d1b] px-4 py-2 text-[11px] font-semibold uppercase tracking-[0.24em] text-[#73d9b5]">
                    Encryption: AES-256 Active
                  </div>
                  <div className="mt-5 flex gap-2">
                    <button
                      type="button"
                      onClick={() => setMurmurView("sanctuary")}
                      className={`rounded-full px-4 py-2 text-[11px] font-semibold uppercase tracking-[0.22em] transition ${
                        isSanctuaryView ? "bg-[#73d9b5] text-[#004a37]" : "bg-[#191a1a] text-[#8da29d] hover:text-[#e7e5e5]"
                      }`}
                    >
                      Sanctuary
                    </button>
                    <button
                      type="button"
                      onClick={() => setMurmurView("whispers")}
                      className={`rounded-full px-4 py-2 text-[11px] font-semibold uppercase tracking-[0.22em] transition ${
                        !isSanctuaryView ? "bg-[#73d9b5] text-[#004a37]" : "bg-[#191a1a] text-[#8da29d] hover:text-[#e7e5e5]"
                      }`}
                    >
                      Whispers
                    </button>
                  </div>
                </div>

                <div className="mt-8 rounded-[28px] border border-[#252626] bg-[#131313] p-5">
                  <div className="text-[11px] uppercase tracking-[0.24em] text-[#8da29d]">New Murmur</div>
                  <div className="mt-4 space-y-3">
                    <input
                      value={title}
                      onChange={(event) => setTitle(event.target.value)}
                      placeholder="標題 可留空"
                      className="w-full rounded-full border border-[#252626] bg-[#0e0e0e] px-4 py-3 text-sm text-white outline-none placeholder:text-[#767575]"
                    />
                    <textarea
                      value={body}
                      onChange={(event) => setBody(event.target.value)}
                      placeholder={activeConfig.placeholder}
                      className="min-h-40 w-full rounded-[24px] border border-[#252626] bg-[#0e0e0e] px-4 py-4 text-sm leading-7 text-white outline-none placeholder:text-[#767575]"
                    />
                    <button
                      type="button"
                      onClick={() => void handleCreate()}
                      disabled={isCreating || !body.trim()}
                      className="w-full rounded-full bg-[#73d9b5] px-4 py-3 text-sm font-bold text-[#004a37] transition hover:bg-[#81e7c3] disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {isCreating ? "建立中..." : "New Murmur"}
                    </button>
                  </div>
                </div>
              </>
            ) : (
              <>
                <div className="mb-8 flex items-center gap-3 px-2 py-4">
                  <div className="flex h-10 w-10 items-center justify-center rounded-full bg-[#7c3aed] text-white shadow-lg shadow-[#7c3aed]/15">
                    ✦
                  </div>
                  <div>
                    <div className="text-sm font-bold text-[#1a1c1c]" style={{ fontFamily: "var(--font-manrope)" }}>
                      The Agent
                    </div>
                    <div className="text-xs text-[#7b7487]">Discourse Guide</div>
                  </div>
                </div>

                <div className="space-y-1">
                  <button
                    type="button"
                    className="flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-sm font-medium text-[#5d5c74] transition hover:bg-[#efedf4]"
                  >
                    <span>⌂</span>
                    <span>Home</span>
                  </button>
                  <button
                    type="button"
                    className="flex w-full items-center gap-3 rounded-xl bg-[#f0e9ff] px-3 py-2 text-left text-sm font-medium text-[#630ed4]"
                  >
                    <span>▦</span>
                    <span>All Boards</span>
                  </button>
                </div>

                <div className="mt-6 px-3 text-[10px] font-bold uppercase tracking-[0.24em] text-[#9a92aa]">
                  Active Boards
                </div>
                <div className="mt-2 space-y-1">
                  {DISCUSSION_BOARDS.map((board) => {
                    const isActiveBoard = board === "Digital Sovereignty";
                    return (
                      <button
                        key={board}
                        type="button"
                        className={`flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-sm font-medium transition ${
                          isActiveBoard
                            ? "bg-[#f4f0ff] text-[#630ed4]"
                            : "text-[#5d5c74] hover:bg-[#efedf4]"
                        }`}
                      >
                        <span className={isActiveBoard ? "text-[#7c3aed]" : "text-[#a79fb4]"}>•</span>
                        <span>{board}</span>
                      </button>
                    );
                  })}
                </div>

                <button
                  type="button"
                  className="mt-8 w-full rounded-2xl bg-[#630ed4] px-4 py-3 text-sm font-bold text-white shadow-lg shadow-[#630ed4]/20 transition hover:scale-[0.99]"
                  style={{ fontFamily: "var(--font-manrope)" }}
                >
                  Create New Board
                </button>

                <div className="mt-auto pt-6">
                  <div className="space-y-1">
                    <button
                      type="button"
                      className="flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-sm text-[#5d5c74] transition hover:bg-[#efedf4]"
                    >
                      <span>⚙</span>
                      <span>Settings</span>
                    </button>
                    <button
                      type="button"
                      className="flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-sm text-[#5d5c74] transition hover:bg-[#efedf4]"
                    >
                      <span>🛡</span>
                      <span>Trust Protocol</span>
                    </button>
                  </div>
                </div>
              </>
            )}
          </aside>

          {isIdeaMode ? (
            <main className="px-8 py-8 lg:px-12">
              {isLoading ? (
                <div className="rounded-[28px] border border-[#ddd6e7] bg-white p-8 text-sm text-[#6a617c] shadow-sm">
                  正在載入 idea articles…
                </div>
              ) : !selectedIdea ? (
                <div className="rounded-[28px] border border-dashed border-[#ddd6e7] bg-white p-8 text-sm leading-6 text-[#6a617c] shadow-sm">
                  {activeConfig.empty}
                </div>
              ) : (
                <div className="mx-auto grid max-w-7xl grid-cols-1 gap-10 xl:grid-cols-[minmax(0,1fr)_360px]">
                  <article className="overflow-hidden rounded-[30px] border border-[#ddd6e7] bg-white shadow-[0_18px_60px_rgba(31,21,52,0.08)]">
                    <div className="h-80 w-full overflow-hidden bg-[#ece8f4]">
                      <img src={IDEA_HERO_IMAGE} alt="Idea hero" className="h-full w-full object-cover grayscale-[0.15]" />
                    </div>
                    <div className="px-8 py-8 lg:px-12 lg:py-10">
                      <header className="border-b border-[#ebe5f3] pb-8">
                        <div className="mb-5 flex flex-wrap items-center gap-3">
                          <span className="rounded-full bg-[#f0e9ff] px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.24em] text-[#6424d8]">Idea Layer</span>
                          <span className="text-xs font-medium uppercase tracking-[0.24em] text-[#81778f]">{selectedIdea.status}</span>
                          <span className="text-xs font-medium uppercase tracking-[0.24em] text-[#81778f]">Reading Time · {Math.max(3, Math.ceil(selectedIdea.body.length / 420))} min</span>
                        </div>
                        <h2
                          className="max-w-4xl text-4xl font-extrabold leading-tight text-[#18151f] lg:text-5xl"
                          style={{ fontFamily: "var(--font-manrope)" }}
                        >
                          {selectedIdea.title?.trim() || firstLine(selectedIdea.body)}
                        </h2>
                        <div className="mt-8 flex flex-wrap items-center justify-between gap-4">
                          <div className="flex items-center gap-4">
                            <div className="flex h-12 w-12 items-center justify-center rounded-full bg-[#ece8f5] text-sm font-bold text-[#5b536b]">
                              {formatAuthor(selectedIdea.authorDid).slice(-2).toUpperCase()}
                            </div>
                            <div>
                              <div className="flex items-center gap-2">
                                <span className="font-semibold text-[#18151f]">{formatAuthor(selectedIdea.authorDid)}</span>
                                <span className="rounded-full bg-[#6424d8] px-2 py-0.5 text-[10px] font-bold text-white">LVL {selectedIdea.trustTier ?? 0}</span>
                              </div>
                              <div className="text-sm text-[#81778f]">{formatDate(selectedIdea.createdAt)}</div>
                            </div>
                          </div>
                          <div className="flex items-center gap-2 text-[#8d83a0]">
                            <button type="button" className="rounded-full border border-[#ddd6e7] px-3 py-2 text-xs uppercase tracking-[0.18em] transition hover:bg-[#f6f3fb]">Share</button>
                            <button type="button" className="rounded-full border border-[#ddd6e7] px-3 py-2 text-xs uppercase tracking-[0.18em] transition hover:bg-[#f6f3fb]">More</button>
                          </div>
                        </div>
                      </header>

                      <section
                        className="mx-auto mt-10 max-w-3xl whitespace-pre-wrap text-[1.24rem] leading-[2.15rem] text-[#2a2433]"
                        style={{ fontFamily: "var(--font-newsreader)" }}
                      >
                        {selectedIdea.body}
                      </section>
                    </div>
                  </article>

                  <aside className="space-y-6">
                    <section className="overflow-hidden rounded-[28px] bg-[#f2eff8] p-6 shadow-sm">
                      <div className="mb-5 flex items-center gap-3">
                        <div className="flex h-8 w-8 items-center justify-center rounded-full bg-[#e5d8ff] text-sm text-[#6424d8]">✦</div>
                        <div className="text-xs font-semibold uppercase tracking-[0.24em] text-[#6f6780]">AI 寫作助手</div>
                      </div>
                      <div className="space-y-4">
                        {IDEA_INSIGHTS.map((insight) => (
                          <div key={insight} className="rounded-2xl border border-[#ddd6e7] bg-white p-4 shadow-sm">
                            <p className="text-sm leading-6 text-[#373142]" style={{ fontFamily: "var(--font-newsreader)" }}>
                              {insight}
                            </p>
                            <div className="mt-3 text-[11px] font-semibold uppercase tracking-[0.2em] text-[#6424d8]">
                              AI insight
                            </div>
                          </div>
                        ))}
                      </div>
                    </section>

                    <section className="rounded-[28px] border border-[#ddd6e7] bg-white p-6 shadow-sm">
                      <div className="text-sm font-semibold text-[#18151f]">內容投影設置</div>
                      <div className="mt-5 space-y-5">
                        <div className="flex items-start justify-between gap-4">
                          <div>
                            <div className="font-medium text-[#18151f]">投射至公共論辯層</div>
                            <div className="mt-1 text-sm leading-6 text-[#81778f]">允許這篇 idea 被轉成可辯論的 public discussion surface。</div>
                          </div>
                          <button
                            type="button"
                            onClick={() => openProjection(selectedIdea)}
                            className="rounded-full bg-[#6424d8] px-4 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-white transition hover:bg-[#5620b7]"
                          >
                            Project
                          </button>
                        </div>

                        <div className="rounded-2xl bg-[#f7f4fb] p-4">
                          <div className="text-[11px] uppercase tracking-[0.22em] text-[#81778f]">Projection Status</div>
                          <div className="mt-3 grid grid-cols-3 gap-2 text-[11px] font-semibold uppercase tracking-[0.16em]">
                            <div className="rounded-xl border border-[#cdbaf2] bg-[#efe7ff] px-3 py-2 text-center text-[#6424d8]">完全公開</div>
                            <div className="rounded-xl bg-white px-3 py-2 text-center text-[#81778f]">同好圈</div>
                            <div className="rounded-xl bg-white px-3 py-2 text-center text-[#81778f]">僅個人</div>
                          </div>
                        </div>
                      </div>
                    </section>

                    <section className="rounded-[28px] border border-[#ddd6e7] bg-white p-6 shadow-sm">
                      <div className="text-[11px] font-black uppercase tracking-[0.24em] text-[#81778f]">文檔元數據</div>
                      <div className="mt-4 space-y-3 text-sm">
                        <div className="flex items-center justify-between text-[#81778f]">
                          <span>修改時間</span>
                          <span className="font-medium text-[#18151f]">{formatDate(selectedIdea.updatedAt)}</span>
                        </div>
                        <div className="flex items-center justify-between text-[#81778f]">
                          <span>隱私等級</span>
                          <span className="font-medium text-[#6424d8]">{selectedIdea.visibility === "public" ? "公開" : "Local-only"}</span>
                        </div>
                        <div className="flex items-center justify-between text-[#81778f]">
                          <span>投影狀態</span>
                          <span className="font-medium text-[#ba4a34]">{selectedIdea.visibility === "public" ? "已發布" : "未發布"}</span>
                        </div>
                      </div>
                    </section>
                  </aside>
                </div>
              )}
            </main>
          ) : isMurmurMode ? (
            <>
              <main className="px-6 pb-24 pt-8 md:px-10">
                <div className="mx-auto max-w-2xl space-y-10">
                  <div>
                    <div className="text-[11px] uppercase tracking-[0.24em] text-[#8da29d]">{isSanctuaryView ? "Sanctuary" : "Whispers"}</div>
                    <h2
                      className="mt-3 text-4xl font-extrabold tracking-tight text-[#e7e5e5]"
                      style={{ fontFamily: "var(--font-manrope)" }}
                    >
                      {isSanctuaryView ? "迷霧之境" : "好友呢喃"}
                    </h2>
                    <p className="mt-2 text-sm font-medium text-[#acabaa]">
                      {isSanctuaryView ? "只屬於你的本地 murmur 與語音片段" : "來自您的加密圈子之私密動態"}
                    </p>
                  </div>

                  {isLoading ? (
                    <div className="rounded-[28px] border border-[#252626] bg-[#131313] p-8 text-sm text-[#acabaa]">
                      {isSanctuaryView ? "正在讀取你的本地 murmur…" : "正在讀取密友動態…"}
                    </div>
                  ) : activeItems.length === 0 ? (
                    <div className="rounded-[28px] border border-dashed border-[#252626] bg-[#131313] p-8 text-sm leading-6 text-[#acabaa]">
                      {isSanctuaryView ? "還沒有個人 murmur。先寫下一句只留在金庫裡的片段。" : "還沒有 murmur。先寫下一句只給圈內人看的觀察。"}
                    </div>
                  ) : (
                    activeItems.map((item, index) => (
                      <article
                        key={item.id}
                        className="group rounded-[28px] border border-[#252626] bg-[#131313] p-8 transition duration-300 hover:border-[#2c3f3b]"
                      >
                        <div className="mb-6 flex items-start justify-between gap-4">
                          <div className="flex items-center gap-4">
                            <div className="relative flex h-12 w-12 items-center justify-center rounded-full bg-[#252626] text-sm font-bold text-[#e7e5e5]">
                              {isSanctuaryView ? "我" : formatAuthor(item.authorDid).slice(-2).toUpperCase()}
                              {isSanctuaryView ? null : (
                                <div className="absolute -bottom-1 -right-1 rounded-full border-2 border-[#131313] bg-[#73d9b5] p-0.5 text-[10px] text-[#004a37]">
                                  ✓
                                </div>
                              )}
                            </div>
                            <div>
                              <div className="flex items-center gap-2">
                                <span className="font-bold text-[#e7e5e5]">{isSanctuaryView ? sessionName || "You" : formatAuthor(item.authorDid)}</span>
                                <span className="rounded-full bg-[#2c3f3b] px-2 py-0.5 text-[9px] font-black uppercase tracking-[0.2em] text-[#73d9b5]">
                                  {isSanctuaryView ? "Local-only" : `L${item.trustTier ?? 1} Curator`}
                                </span>
                              </div>
                              <span className="text-xs text-[#acabaa]">{formatDate(item.createdAt)}</span>
                            </div>
                          </div>
                          <span className="text-[#73d9b5]/60">{isSanctuaryView ? "vault" : "key"}</span>
                        </div>

                        {isSanctuaryView && index % 3 === 2 ? (
                          <div className="flex items-start gap-4">
                            <span className="text-[#73d9b5]">graphic_eq</span>
                            <div className="flex-1">
                              <p className="whitespace-pre-wrap text-lg font-light leading-relaxed text-[#e7e5e5]">
                                語音備忘錄：{truncate(item.body, 42)}
                              </p>
                            </div>
                          </div>
                        ) : (
                          <p className="whitespace-pre-wrap text-lg font-light leading-relaxed text-[#e7e5e5]">
                            {item.body}
                          </p>
                        )}

                        <div className="mt-6 flex flex-wrap gap-2">
                          <span className="rounded-full bg-[#1f2020] px-3 py-1 text-[10px] font-bold text-[#8da29d]">
                            #{item.mode}
                          </span>
                          <span className="rounded-full bg-[#1f2020] px-3 py-1 text-[10px] font-bold text-[#8da29d]">
                            {item.visibility}
                          </span>
                          {item.title?.trim() ? (
                            <span className="rounded-full bg-[#1f2020] px-3 py-1 text-[10px] font-bold text-[#8da29d]">
                              {item.title}
                            </span>
                          ) : null}
                        </div>

                        <div className="mt-6 flex items-center justify-between gap-4">
                          {isSanctuaryView ? (
                            <div className="text-[10px] font-medium text-[#767575]">
                              {new Date(item.createdAt).toLocaleDateString("zh-TW").replace(/\//g, ".")}
                            </div>
                          ) : (
                            <div className="flex items-center gap-6 text-[#8da29d]">
                              <div className="flex items-center gap-2 text-xs">
                                <span>favorite</span>
                                <span>{12 + index}</span>
                              </div>
                              <div className="flex items-center gap-2 text-xs">
                                <span>bubble_chart</span>
                                <span>{Math.max(2, Math.ceil(item.body.length / 80))} 共鳴</span>
                              </div>
                            </div>
                          )}
                          <button
                            type="button"
                            onClick={() => openTransform(item)}
                            className="rounded-full border border-[#2c3f3b] bg-[#191f1e] px-4 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-[#73d9b5] transition hover:bg-[#22302c]"
                          >
                            {isSanctuaryView ? "整理成想法" : "轉譯成想法"}
                          </button>
                        </div>
                      </article>
                    ))
                  )}
                </div>
              </main>

              <aside className="hidden border-l border-[#1f2020] bg-[#131313] p-6 lg:flex lg:flex-col">
                <div>
                  <h3 className="text-sm font-black uppercase tracking-[0.24em] text-[#e7e5e5]">{isSanctuaryView ? "AI 感應浮現" : "AI 共鳴分析"}</h3>
                  <div className="mt-6 rounded-[28px] border border-[#252626] bg-[#191a1a] p-6">
                    <div className="flex items-center gap-3 text-[#73d9b5]">
                      <span>{isSanctuaryView ? "sensors" : "neurology"}</span>
                      <span className="text-xs font-bold uppercase tracking-[0.24em]">{isSanctuaryView ? "感應主題" : "集體意識流"}</span>
                    </div>
                    <p className="mt-4 text-sm leading-7 text-[#acabaa]">
                      {isSanctuaryView ? (
                        <>
                          偵測到你最近頻繁提到 <span className="font-bold text-[#e7e5e5]">本地</span>、<span className="font-bold text-[#e7e5e5]">加密</span> 與
                          <span className="font-bold text-[#e7e5e5]"> 邊界</span>。這組合反映了你近期對數位主權的高度專注。
                        </>
                      ) : (
                        <>
                          本週你的圈子裡，關於 <span className="font-bold text-[#e7e5e5]">數位隱私</span> 與
                          <span className="font-bold text-[#e7e5e5]"> 空間美學</span> 的討論重合度達到 84%。
                        </>
                      )}
                    </p>
                    <div className="mt-4 h-1 w-full overflow-hidden rounded-full bg-[#252626]">
                      <div className={`h-full bg-[#73d9b5] ${isSanctuaryView ? "w-[100%]" : "w-[84%]"}`} />
                    </div>
                  </div>
                </div>

                <div className="mt-8">
                  <h3 className="text-[10px] font-black uppercase tracking-[0.24em] text-[#8da29d]">{isSanctuaryView ? "浮現語彙" : "熱門關鍵字"}</h3>
                  <div className="mt-4 flex flex-wrap gap-2">
                    {MURMUR_KEYWORDS.map((keyword) => (
                      <span
                        key={keyword}
                        className="rounded-full border border-[#252626] bg-[#191a1a] px-4 py-2 text-xs text-[#e7e5e5]"
                      >
                        {keyword}
                      </span>
                    ))}
                  </div>
                </div>

                <div className="mt-auto rounded-[28px] border border-[#252626] bg-[#191a1a] p-4">
                  <div className="mb-3 flex items-center justify-between">
                    <span className="text-[10px] font-bold uppercase tracking-[0.22em] text-[#8da29d]">系統安全狀態</span>
                    <span className="text-[10px] font-bold uppercase tracking-[0.22em] text-[#73d9b5]">Secure</span>
                  </div>
                  <div className="flex items-end justify-between">
                    <div className="flex items-end gap-1">
                      <div className="h-3 w-1 rounded-full bg-[#73d9b5]" />
                      <div className="h-5 w-1 rounded-full bg-[#73d9b5]" />
                      <div className="h-4 w-1 rounded-full bg-[#73d9b5]" />
                      <div className="h-6 w-1 rounded-full bg-[#73d9b5]" />
                      <div className="h-3 w-1 rounded-full bg-[#73d9b5]" />
                    </div>
                    <span className="text-[10px] text-[#acabaa]">Ping: 24ms</span>
                  </div>
                </div>
              </aside>
            </>
          ) : (
            <>
              <main className="px-8 py-8">
                <section className="mb-10">
                  <div className="mb-6 flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
                    <div>
                      <h2
                        className="text-4xl font-extrabold tracking-tight text-[#1a1c1c]"
                        style={{ fontFamily: "var(--font-manrope)" }}
                      >
                        Digital Sovereignty
                      </h2>
                      <p
                        className="mt-2 max-w-2xl text-xl italic text-[#5d5c74]"
                        style={{ fontFamily: "var(--font-newsreader)" }}
                      >
                        Architecting the future of local-first web persistence.
                      </p>
                    </div>
                    <button
                      type="button"
                      className="rounded-2xl bg-[#630ed4] px-8 py-3 text-sm font-bold text-white shadow-xl shadow-[#630ed4]/10 transition hover:bg-[#732ee4]"
                      style={{ fontFamily: "var(--font-manrope)" }}
                    >
                      Join Board
                    </button>
                  </div>

                  <div className="grid gap-4 rounded-[28px] bg-[#f3f3f3] p-6 md:grid-cols-3">
                    <div>
                      <div className="text-[10px] font-bold uppercase tracking-[0.22em] text-[#7b7487]">Active Members</div>
                      <div className="mt-2 text-2xl font-bold text-[#1a1c1c]" style={{ fontFamily: "var(--font-manrope)" }}>12.4k</div>
                    </div>
                    <div>
                      <div className="text-[10px] font-bold uppercase tracking-[0.22em] text-[#7b7487]">Daily Murmurs</div>
                      <div className="mt-2 text-2xl font-bold text-[#1a1c1c]" style={{ fontFamily: "var(--font-manrope)" }}>842</div>
                    </div>
                    <div>
                      <div className="text-[10px] font-bold uppercase tracking-[0.22em] text-[#7b7487]">Verification Tier</div>
                      <div className="mt-2 text-2xl font-bold text-[#630ed4]" style={{ fontFamily: "var(--font-manrope)" }}>L3 Protocols ✓</div>
                    </div>
                  </div>
                </section>

                <section className="mb-8 rounded-[28px] border border-[#ece7f4] bg-white p-6 shadow-sm">
                  <div className="mb-4 flex items-end justify-between gap-4">
                    <div>
                      <div className="text-[10px] font-bold uppercase tracking-[0.22em] text-[#7b7487]">Surface a thread</div>
                      <div className="mt-2 text-xl font-bold text-[#1a1c1c]" style={{ fontFamily: "var(--font-manrope)" }}>
                        發起新的 public discussion
                      </div>
                    </div>
                    <div className="text-xs text-[#7b7487]">{activeItems.length} threads</div>
                  </div>
                  <div className="space-y-3">
                    <input
                      value={title}
                      onChange={(event) => setTitle(event.target.value)}
                      placeholder={`標題 ${activeConfig.titleHint}`}
                      className="w-full rounded-2xl border border-[#e5e0ec] bg-[#faf9fc] px-4 py-3 text-sm text-[#1a1c1c] outline-none placeholder:text-[#8e879b]"
                    />
                    <textarea
                      value={body}
                      onChange={(event) => setBody(event.target.value)}
                      placeholder={activeConfig.placeholder}
                      className="min-h-36 w-full rounded-2xl border border-[#e5e0ec] bg-[#faf9fc] px-4 py-3 text-sm leading-7 text-[#1a1c1c] outline-none placeholder:text-[#8e879b]"
                    />
                    <button
                      type="button"
                      onClick={() => void handleCreate()}
                      disabled={isCreating || !body.trim()}
                      className="rounded-2xl bg-[#630ed4] px-5 py-3 text-sm font-semibold text-white transition hover:bg-[#732ee4] disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {isCreating ? "建立中..." : "Publish to Discourse"}
                    </button>
                  </div>
                </section>

                <section className="space-y-6">
                  {isLoading ? (
                    <div className="rounded-[28px] border border-[#ece7f4] bg-white p-8 text-sm text-[#7b7487] shadow-sm">
                      正在載入 board threads…
                    </div>
                  ) : activeItems.length === 0 ? (
                    <div className="rounded-[28px] border border-dashed border-[#d7d0e3] bg-white p-8 text-sm leading-6 text-[#7b7487] shadow-sm">
                      {activeConfig.empty}
                    </div>
                  ) : (
                    activeItems.map((item, index) => (
                      <button
                        key={item.id}
                        type="button"
                        onClick={() => setSelectedDiscussion(item)}
                        className={`w-full rounded-[28px] border p-6 text-left shadow-sm transition ${
                          selectedDiscussion?.id === item.id
                            ? "border-[#d7c6ff] bg-[#fcfbff]"
                            : "border-[#ece7f4] bg-white hover:bg-[#fcfbff]"
                        }`}
                      >
                        <div className="mb-4 flex items-start justify-between gap-4">
                          <div className="flex items-center gap-3">
                            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-[#efebf7] text-sm font-bold text-[#5c5667]">
                              {formatAuthor(item.authorDid).slice(-2).toUpperCase()}
                            </div>
                            <div>
                              <div className="flex flex-wrap items-center gap-2">
                                <span className="text-sm font-bold text-[#1a1c1c]" style={{ fontFamily: "var(--font-manrope)" }}>
                                  {formatAuthor(item.authorDid)}
                                </span>
                                <span className="rounded-full bg-[#f0e9ff] px-2 py-0.5 text-[10px] font-bold text-[#630ed4]">
                                  {item.trustTier && item.trustTier >= 3 ? "VERIFIED RESIDENT" : `L${item.trustTier ?? 0} PARTICIPANT`}
                                </span>
                              </div>
                              <div className="text-[10px] text-[#9a92aa]">Posted {formatDate(item.createdAt)}</div>
                            </div>
                          </div>
                          <span className="text-[#b9b0c9]">⋯</span>
                        </div>

                        <h3
                          className="text-2xl font-bold tracking-tight text-[#1a1c1c]"
                          style={{ fontFamily: "var(--font-manrope)" }}
                        >
                          {item.title?.trim() || firstLine(item.body)}
                        </h3>

                        <div className="relative mt-5 overflow-hidden rounded-2xl bg-[#f6f0ff] p-4">
                          <div className="absolute left-0 top-0 h-full w-1 bg-[#7c3aed]" />
                          <div className="flex items-start gap-3">
                            <span className="text-[#630ed4]">✦</span>
                            <div>
                              <div className="mb-1 text-[10px] font-bold uppercase tracking-[0.22em] text-[#630ed4]">
                                AI Agent Consensus Summary
                              </div>
                              <p
                                className="text-sm italic leading-6 text-[#4a4455]"
                                style={{ fontFamily: "var(--font-newsreader)" }}
                              >
                                {truncate(
                                  item.body,
                                  170,
                                ) || "The board is still clustering around the strongest verified arguments in this thread."
                                }
                              </p>
                            </div>
                          </div>
                        </div>

                        <div className="mt-5 flex items-center justify-between gap-4">
                          <div className="flex flex-wrap gap-6 text-sm font-bold text-[#6a6479]">
                            <div>{Math.max(1, discussionNodes.length || Math.ceil(item.body.length / 90))} Replies</div>
                            <div>{12 + index * 3} Supports</div>
                          </div>
                          <div className="rounded-full border border-[#e5deef] bg-[#faf8fd] px-3 py-1 text-[10px] font-bold uppercase tracking-[0.18em] text-[#7b7487]">
                            {item.participationPolicy ?? "debate"}
                          </div>
                        </div>
                      </button>
                    ))
                  )}
                </section>

                {selectedDiscussion ? (
                  <section className="mt-8 rounded-[28px] border border-[#ece7f4] bg-white p-6 shadow-sm">
                    <div className="mb-5 flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                      <div>
                        <div className="text-[10px] font-bold uppercase tracking-[0.22em] text-[#7b7487]">Thread Surface</div>
                        <h3
                          className="mt-2 text-2xl font-bold text-[#1a1c1c]"
                          style={{ fontFamily: "var(--font-manrope)" }}
                        >
                          {selectedDiscussion.title?.trim() || firstLine(selectedDiscussion.body)}
                        </h3>
                        <p className="mt-3 whitespace-pre-wrap text-sm leading-7 text-[#4a4455]">
                          {selectedDiscussion.body}
                        </p>
                      </div>
                      <div className="flex flex-wrap gap-3">
                        <button
                          type="button"
                          onClick={() => setIsForkDialogOpen(true)}
                          disabled={!canFork}
                          className="rounded-full border border-[#d7c6ff] bg-[#f5efff] px-4 py-2 text-xs font-semibold text-[#630ed4] transition hover:bg-[#efe5ff] disabled:cursor-not-allowed disabled:opacity-40"
                        >
                          Fork 討論
                        </button>
                        <button
                          type="button"
                          onClick={() => {
                            setModerationType(canSlash ? "slash" : "flag");
                            setIsModerationDialogOpen(true);
                          }}
                          disabled={!canFlag && !canSlash}
                          className="rounded-full border border-[#f2d1cf] bg-[#fff2f1] px-4 py-2 text-xs font-semibold text-[#ba1a1a] transition hover:bg-[#ffe8e6] disabled:cursor-not-allowed disabled:opacity-40"
                        >
                          治理動作
                        </button>
                      </div>
                    </div>

                    <div className="mb-5 space-y-1 text-xs text-[#7b7487]">
                      {!canFork ? <div>{trustRequirementLabel(2)} 才能 fork 公共討論。</div> : null}
                      {!canFlag ? <div>{trustRequirementLabel(2)} 才能提出 flag。</div> : null}
                      {!canSlash ? <div>{trustRequirementLabel(4)} 才能提出 slash / lock / hide。</div> : null}
                    </div>

                    <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_320px]">
                      <div className="rounded-[24px] bg-[#f7f5fb] p-5">
                        <div className="mb-4 flex items-center justify-between">
                          <div className="text-sm font-semibold text-[#1a1c1c]">Nodes</div>
                          <div className="text-xs text-[#7b7487]">{discussionNodes.length} entries</div>
                        </div>
                        {isLoadingNodes ? (
                          <div className="text-sm text-[#7b7487]">正在讀取節點…</div>
                        ) : discussionNodes.length === 0 ? (
                          <div className="text-sm leading-6 text-[#7b7487]">
                            這個討論還沒有節點，第一個回應可以是 claim、question、evidence 或 rebuttal。
                          </div>
                        ) : (
                          <div className="space-y-3">
                            {discussionNodes.map((node) => (
                              <div key={node.id} className="rounded-[20px] border border-[#e7e1ef] bg-white p-4">
                                <div className="flex flex-wrap items-center gap-2 text-xs uppercase tracking-[0.18em] text-[#7b7487]">
                                  <span>{node.nodeType}</span>
                                  <span>•</span>
                                  <span>{node.stance}</span>
                                  <span>•</span>
                                  <span>{formatAuthor(node.authorDid)}</span>
                                </div>
                                <div className="mt-3 whitespace-pre-wrap text-sm leading-6 text-[#36303f]">
                                  {node.body}
                                </div>
                              </div>
                            ))}
                          </div>
                        )}
                      </div>

                      <div className="rounded-[24px] bg-[#f7f5fb] p-5">
                        <div className="text-sm font-semibold text-[#1a1c1c]">新增節點</div>
                        <div className="mt-4 grid grid-cols-2 gap-3">
                          <select
                            value={replyType}
                            onChange={(event) => setReplyType(event.target.value as CreateDiscussionNodeRequest["nodeType"])}
                            className="rounded-2xl border border-[#e5deef] bg-white px-4 py-3 text-sm text-[#1a1c1c] outline-none"
                          >
                            <option value="claim">claim</option>
                            <option value="question">question</option>
                            <option value="evidence">evidence</option>
                            <option value="rebuttal">rebuttal</option>
                            <option value="summary">summary</option>
                          </select>
                          <select
                            value={replyStance}
                            onChange={(event) => setReplyStance(event.target.value as CreateDiscussionNodeRequest["stance"])}
                            className="rounded-2xl border border-[#e5deef] bg-white px-4 py-3 text-sm text-[#1a1c1c] outline-none"
                          >
                            <option value="support">support</option>
                            <option value="oppose">oppose</option>
                            <option value="clarify">clarify</option>
                            <option value="neutral">neutral</option>
                          </select>
                        </div>
                        <textarea
                          value={replyBody}
                          onChange={(event) => setReplyBody(event.target.value)}
                          placeholder="用一個清楚的節點回應這場討論。"
                          className="mt-3 min-h-32 w-full rounded-2xl border border-[#e5deef] bg-white px-4 py-3 text-sm leading-6 text-[#1a1c1c] outline-none placeholder:text-[#8e879b]"
                        />
                        <button
                          type="button"
                          onClick={() => void handleReply()}
                          disabled={!replyBody.trim()}
                          className="mt-3 w-full rounded-2xl bg-[#630ed4] px-4 py-3 text-sm font-semibold text-white transition hover:bg-[#732ee4] disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          送出節點
                        </button>
                      </div>
                    </div>
                  </section>
                ) : null}
              </main>

              <aside className="border-l border-[#ece7f4] bg-[#fafafa] p-6">
                <section>
                  <h4 className="mb-4 text-[10px] font-bold uppercase tracking-[0.24em] text-[#9a92aa]">Board Intelligence</h4>
                  <div className="rounded-[24px] bg-white p-5 shadow-sm">
                    <div className="flex items-center gap-3">
                      <span className="text-[#630ed4]">⚖</span>
                      <span className="text-sm font-bold text-[#1a1c1c]" style={{ fontFamily: "var(--font-manrope)" }}>
                        Truth Anchoring
                      </span>
                    </div>
                    <p className="mt-4 text-xs leading-6 text-[#5d5c74]">
                      All claims must be backed by cryptographic proof of authorship or public record. No algorithmic manipulation of visibility.
                    </p>
                  </div>
                </section>

                <section className="mt-8">
                  <div className="mb-4 flex items-center justify-between">
                    <h4 className="text-[10px] font-bold uppercase tracking-[0.24em] text-[#9a92aa]">Trust Protocol</h4>
                    <span className="text-[#630ed4]">🛡</span>
                  </div>
                  <div className="space-y-3">
                    <div className="flex items-center justify-between rounded-2xl border border-[#ece7f4] bg-white p-3 text-xs">
                      <span className="font-medium text-[#1a1c1c]">Sybil Resistance</span>
                      <span className="rounded-full bg-[#edf8ef] px-2 py-0.5 font-bold text-[#2f8b57]">ACTIVE</span>
                    </div>
                    <div className="flex items-center justify-between rounded-2xl border border-[#ece7f4] bg-white p-3 text-xs">
                      <span className="font-medium text-[#1a1c1c]">Reputation Weighting</span>
                      <span className="rounded-full bg-[#f0e9ff] px-2 py-0.5 font-bold text-[#630ed4]">
                        {canSlash ? "L4 ENABLED" : `L${sessionTier} ENABLED`}
                      </span>
                    </div>
                    <div className="flex items-center justify-between rounded-2xl border border-[#ece7f4] bg-white p-3 text-xs">
                      <span className="font-medium text-[#1a1c1c]">ZKP Verification</span>
                      <span className="rounded-full bg-[#f5f5f5] px-2 py-0.5 font-bold text-[#7b7487]">OPTIONAL</span>
                    </div>
                  </div>
                  {canRequestVerification ? (
                    <div className="mt-4 rounded-2xl border border-[#ece7f4] bg-white p-4">
                      <div className="text-[10px] font-bold uppercase tracking-[0.2em] text-[#9a92aa]">Verification Request</div>
                      <p className="mt-2 text-xs leading-5 text-[#5d5c74]">
                        送出 L2/L3 case 後，只有已登記的 L4 verifier 可以審核並發出 assessment 或 credential。
                      </p>
                      <div className="mt-3 grid grid-cols-2 gap-2">
                        <button
                          type="button"
                          disabled={isRequestingVerification || sessionTier >= 2}
                          onClick={() => void handleVerificationRequest(2)}
                          className="rounded-xl bg-[#f0e9ff] px-3 py-2 text-xs font-bold text-[#630ed4] transition hover:bg-[#e6dcff] disabled:opacity-40"
                        >
                          申請 L2
                        </button>
                        <button
                          type="button"
                          disabled={isRequestingVerification || sessionTier >= 3}
                          onClick={() => void handleVerificationRequest(3)}
                          className="rounded-xl bg-[#630ed4] px-3 py-2 text-xs font-bold text-white transition hover:bg-[#5210aa] disabled:opacity-40"
                        >
                          申請 L3
                        </button>
                      </div>
                    </div>
                  ) : null}
                  {canRequestVerification ? <WalletVerificationPanel user={auth.user} sessionTier={sessionTier} /> : null}
                </section>

                <section className="mt-8">
                  <h4 className="mb-4 text-[10px] font-bold uppercase tracking-[0.24em] text-[#9a92aa]">Your Related Ideas</h4>
                  <div className="space-y-4">
                    {relatedIdeas.length > 0 ? (
                      relatedIdeas.map((idea, index) => (
                        <div
                          key={idea.id}
                          className={
                            index === 0
                              ? "relative overflow-hidden rounded-[24px] bg-[#630ed4] p-4 text-white shadow-lg shadow-violet-200"
                              : "rounded-[24px] border border-[#e9e2f2] bg-white p-4"
                          }
                        >
                          {index === 0 ? (
                            <>
                              <div className="absolute -right-4 -top-4 h-16 w-16 rounded-full bg-white/10 blur-xl" />
                              <div className="text-[10px] font-bold uppercase text-violet-200">From your Idea Layer</div>
                              <p className="mt-2 text-sm italic" style={{ fontFamily: "var(--font-newsreader)" }}>
                                {truncate(idea.body, 100)}
                              </p>
                              <button
                                type="button"
                                onClick={() => openProjection(idea)}
                                className="mt-3 rounded-full bg-white/20 px-3 py-1 text-[10px] font-bold uppercase tracking-[0.16em] transition hover:bg-white/30"
                              >
                                Surface to Board
                              </button>
                            </>
                          ) : (
                            <>
                              <div className="text-[10px] font-bold uppercase text-[#630ed4]">Private Murmur</div>
                              <p className="mt-2 text-xs leading-6 text-[#5d5c74]">{truncate(idea.body, 96)}</p>
                              <button
                                type="button"
                                onClick={() => setActiveMode("idea")}
                                className="mt-3 text-[10px] font-bold uppercase tracking-[0.16em] text-[#630ed4] hover:underline"
                              >
                                Open Idea Editor
                              </button>
                            </>
                          )}
                        </div>
                      ))
                    ) : (
                      <div className="rounded-[24px] border border-[#e9e2f2] bg-white p-4 text-xs leading-6 text-[#5d5c74]">
                        還沒有相關 idea。你可以先在 Idea Layer 整理一篇觀點，再把其中一段投射到這個 board。
                      </div>
                    )}
                  </div>
                </section>
              </aside>
            </>
          )}
        </div>
      </div>

      {projectionIdea ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm">
          <div className="w-full max-w-2xl rounded-3xl border border-white/10 bg-[#0b1623] p-6 shadow-2xl">
            <div className="text-xs uppercase tracking-[0.22em] text-slate-500">Ownership Transfer</div>
            <h3 className="mt-3 text-2xl font-semibold text-white">將想法投射到公共討論</h3>
            <p className="mt-3 text-sm leading-7 text-slate-300">
              這個動作會把你選擇的段落轉成 public discussion。進入公共層後，內容可以被回應、反駁與延伸，
              刪除與完全控制權也會明顯下降。
            </p>
            <div className="mt-5 rounded-3xl border border-amber-400/20 bg-amber-400/10 p-4 text-sm leading-6 text-amber-100">
              你仍然保有原始 idea，但這次投射產生的 discussion 會成為社群辯論表面，原作者不再擁有與私人草稿相同的控制權。
            </div>
            <div className="mt-5 grid gap-4">
              <textarea
                value={projectionExcerpt}
                onChange={(event) => setProjectionExcerpt(event.target.value)}
                className="min-h-40 w-full rounded-2xl border border-white/10 bg-[#08111b] px-4 py-3 text-sm leading-6 text-white outline-none"
              />
              <select
                value={projectionPolicy}
                onChange={(event) => setProjectionPolicy(event.target.value as ParticipationPolicy)}
                className="rounded-2xl border border-white/10 bg-[#08111b] px-4 py-3 text-sm text-white outline-none"
              >
                <option value="read_only">唯讀</option>
                <option value="comment">允許評論</option>
                <option value="debate">允許辯論</option>
              </select>
            </div>
            <div className="mt-6 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setProjectionIdea(null)}
                className="rounded-2xl border border-white/10 px-4 py-3 text-sm text-slate-300 transition hover:bg-white/5"
              >
                取消
              </button>
              <button
                type="button"
                onClick={() => void handleProjectIdea()}
                disabled={isProjecting || !projectionExcerpt.trim()}
                className="rounded-2xl bg-white px-4 py-3 text-sm font-semibold text-slate-950 transition hover:bg-slate-200 disabled:opacity-50"
              >
                {isProjecting ? "投射中..." : "確認投射並移交部分控制權"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {transformSource ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm">
          <div className="w-full max-w-3xl rounded-3xl border border-white/10 bg-[#0b1623] p-6 shadow-2xl">
            <div className="text-xs uppercase tracking-[0.22em] text-slate-500">Transformation Review</div>
            <h3 className="mt-3 text-2xl font-semibold text-white">把呢喃轉成想法草稿</h3>
            <p className="mt-3 text-sm leading-7 text-slate-300">
              這一步模擬 LLM 轉譯流程。你可以先選 provider 與人格，再查看生成草稿，最後決定是否接受成新的 idea。
            </p>
            <div className="mt-5 grid gap-4 md:grid-cols-2">
              <select
                value={transformProvider}
                onChange={(event) => setTransformProvider(event.target.value as TransformationProviderType)}
                className="rounded-2xl border border-white/10 bg-[#08111b] px-4 py-3 text-sm text-white outline-none"
              >
                <option value="local_llm">Local LLM</option>
                <option value="byok">BYOK</option>
                <option value="system_llm">System LLM</option>
              </select>
              <select
                value={transformProfile}
                onChange={(event) => setTransformProfile(event.target.value)}
                className="rounded-2xl border border-white/10 bg-[#08111b] px-4 py-3 text-sm text-white outline-none"
              >
                <option value="researcher">Researcher</option>
                <option value="blogger">Blogger</option>
                <option value="moderator">Moderator</option>
              </select>
            </div>
            <div className="mt-5 rounded-3xl border border-white/10 bg-[#08111b] p-4">
              <div className="text-xs uppercase tracking-[0.18em] text-slate-500">Source Murmur</div>
              <div className="mt-3 whitespace-pre-wrap text-sm leading-7 text-slate-200">
                {transformSource.body}
              </div>
            </div>
            {transformJob ? (
              <div className="mt-5 rounded-3xl border border-emerald-400/20 bg-emerald-400/10 p-4">
                <div className="text-xs uppercase tracking-[0.18em] text-emerald-200">Generated Idea Draft</div>
                <div className="mt-3 text-lg font-semibold text-white">{transformJob.outputTitle || "Untitled Draft"}</div>
                <div className="mt-3 whitespace-pre-wrap text-sm leading-7 text-slate-100">
                  {transformJob.outputBody}
                </div>
              </div>
            ) : null}
            <div className="mt-6 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => {
                  setTransformSource(null);
                  setTransformJob(null);
                }}
                className="rounded-2xl border border-white/10 px-4 py-3 text-sm text-slate-300 transition hover:bg-white/5"
              >
                關閉
              </button>
              {!transformJob ? (
                <button
                  type="button"
                  onClick={() => void handleCreateTransformation()}
                  disabled={isTransforming}
                  className="rounded-2xl bg-white px-4 py-3 text-sm font-semibold text-slate-950 transition hover:bg-slate-200 disabled:opacity-50"
                >
                  {isTransforming ? "轉譯中..." : "生成草稿"}
                </button>
              ) : (
                <button
                  type="button"
                  onClick={() => void handlePublishTransformation()}
                  disabled={isPublishingTransform}
                  className="rounded-2xl bg-white px-4 py-3 text-sm font-semibold text-slate-950 transition hover:bg-slate-200 disabled:opacity-50"
                >
                  {isPublishingTransform ? "發布中..." : "接受並建立想法"}
                </button>
              )}
            </div>
          </div>
        </div>
      ) : null}

      {isForkDialogOpen && selectedDiscussion ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm">
          <div className="w-full max-w-xl rounded-3xl border border-white/10 bg-[#0b1623] p-6 shadow-2xl">
            <div className="text-xs uppercase tracking-[0.22em] text-slate-500">Fork Discussion</div>
            <h3 className="mt-3 text-2xl font-semibold text-white">建立衍生討論</h3>
            <p className="mt-3 text-sm leading-7 text-slate-300">
              Fork 會把現有公共討論分岔成另一條社群擁有的辯論支線，原作者無法刪除你的推進脈絡。
            </p>
            <textarea
              value={forkReason}
              onChange={(event) => setForkReason(event.target.value)}
              placeholder="說明你為什麼要 fork 這場討論。"
              className="mt-5 min-h-32 w-full rounded-2xl border border-white/10 bg-[#08111b] px-4 py-3 text-sm leading-6 text-white outline-none"
            />
            <div className="mt-6 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setIsForkDialogOpen(false)}
                className="rounded-2xl border border-white/10 px-4 py-3 text-sm text-slate-300 transition hover:bg-white/5"
              >
                取消
              </button>
              <button
                type="button"
                onClick={() => void handleForkDiscussion()}
                disabled={isForking || !forkReason.trim() || !canFork}
                className="rounded-2xl bg-white px-4 py-3 text-sm font-semibold text-slate-950 transition hover:bg-slate-200 disabled:opacity-50"
              >
                {isForking ? "建立中..." : "確認 Fork"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {isModerationDialogOpen && selectedDiscussion ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm">
          <div className="w-full max-w-xl rounded-3xl border border-white/10 bg-[#0b1623] p-6 shadow-2xl">
            <div className="text-xs uppercase tracking-[0.22em] text-slate-500">Governance</div>
            <h3 className="mt-3 text-2xl font-semibold text-white">建立治理動作</h3>
            <p className="mt-3 text-sm leading-7 text-slate-300">
              L2 可以建立 flag，L4 才能提出 slash、lock 或 hide。這些動作應該只用在明確需要社群治理的內容。
            </p>
            <select
              value={moderationType}
              onChange={(event) => setModerationType(event.target.value as ModerationActionType)}
              className="mt-5 w-full rounded-2xl border border-white/10 bg-[#08111b] px-4 py-3 text-sm text-white outline-none"
            >
              <option value="flag">flag</option>
              <option value="hide" disabled={!canSlash}>hide</option>
              <option value="lock" disabled={!canSlash}>lock</option>
              <option value="slash" disabled={!canSlash}>slash</option>
            </select>
            <textarea
              value={moderationReason}
              onChange={(event) => setModerationReason(event.target.value)}
              placeholder="說明為何需要這個治理動作。"
              className="mt-4 min-h-32 w-full rounded-2xl border border-white/10 bg-[#08111b] px-4 py-3 text-sm leading-6 text-white outline-none"
            />
            <div className="mt-6 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setIsModerationDialogOpen(false)}
                className="rounded-2xl border border-white/10 px-4 py-3 text-sm text-slate-300 transition hover:bg-white/5"
              >
                取消
              </button>
              <button
                type="button"
                onClick={() => void handleModerationAction()}
                disabled={isModerating || !moderationReason.trim() || (!canFlag && !canSlash)}
                className="rounded-2xl bg-white px-4 py-3 text-sm font-semibold text-slate-950 transition hover:bg-slate-200 disabled:opacity-50"
              >
                {isModerating ? "建立中..." : "送出治理動作"}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {canReviewVerificationCases ? <VerifierConsole user={auth.user} /> : null}
    </div>
  );
}
