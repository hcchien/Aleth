"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter, useParams } from "next/navigation";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";
import { getAccessToken } from "@/lib/auth";
import { PagePolicyForm, type PagePolicy } from "@/app/components/page-policy-form";

const PAGE_ADMIN_QUERY = `
  query PageAdmin($slug: String!) {
    page(slug: $slug) {
      id
      slug
      name
      description
      avatarUrl
      coverUrl
      category
      apEnabled
      defaultAccess
      minTrustLevel
      commentPolicy
      minCommentTrust
      requireVcs { vcType issuer }
      requireCommentVcs { vcType issuer }
    }
    pageMembers(pageId: $slug) {
      items {
        role
        joinedAt
        user { id username displayName }
      }
    }
  }
`;

const UPDATE_PAGE_MUTATION = `
  mutation UpdatePage($pageId: ID!, $input: UpdatePageInput!) {
    updatePage(pageId: $pageId, input: $input) {
      id slug name description avatarUrl coverUrl category apEnabled
    }
  }
`;

const SET_AP_ENABLED_MUTATION = `
  mutation UpdatePage($pageId: ID!, $input: UpdatePageInput!) {
    updatePage(pageId: $pageId, input: $input) {
      id apEnabled
    }
  }
`;

const ADD_MEMBER_MUTATION = `
  mutation AddPageMember($pageId: ID!, $userId: ID!, $role: PageRole!) {
    addPageMember(pageId: $pageId, userId: $userId, role: $role)
  }
`;

const REMOVE_MEMBER_MUTATION = `
  mutation RemovePageMember($pageId: ID!, $userId: ID!) {
    removePageMember(pageId: $pageId, userId: $userId)
  }
`;

const DELETE_PAGE_MUTATION = `
  mutation DeletePage($pageId: ID!) {
    deletePage(pageId: $pageId)
  }
`;

const PAGE_SERIES_QUERY = `
  query PageSeries($pageId: ID!) {
    pageSeries(pageId: $pageId) {
      id title description articleCount
    }
  }
`;

const CREATE_SERIES_MUTATION = `
  mutation CreatePageSeries($pageId: ID!, $title: String!, $description: String) {
    createSeries(input: { pageId: $pageId, title: $title, description: $description }) {
      id title description articleCount
    }
  }
`;

const UPDATE_SERIES_MUTATION = `
  mutation UpdateSeries($id: ID!, $title: String!, $description: String) {
    updateSeries(id: $id, input: { title: $title, description: $description }) {
      id title description
    }
  }
`;

const DELETE_SERIES_MUTATION = `
  mutation DeleteSeries($id: ID!) {
    deleteSeries(id: $id)
  }
`;

const FIND_USER_QUERY = `
  query FindUser($username: String!) {
    userByUsername(username: $username) { id username displayName }
  }
`;

interface PageMemberEdge {
  role: string;
  joinedAt: string;
  user: { id: string; username: string; displayName: string | null };
}

interface PageAdminData {
  id: string;
  slug: string;
  name: string;
  description: string | null;
  avatarUrl: string | null;
  coverUrl: string | null;
  category: string;
  apEnabled: boolean;
  defaultAccess: string;
  minTrustLevel: number;
  commentPolicy: string;
  minCommentTrust: number;
  requireVcs: { vcType: string; issuer: string }[];
  requireCommentVcs: { vcType: string; issuer: string }[];
}

interface MemberConnection {
  items: PageMemberEdge[];
}

const CATEGORIES = ["general", "music", "sports", "tech", "art", "gaming", "politics", "education", "other"];

export default function PageAdminPanel() {
  const t = useTranslations("fanPage");
  const tCommon = useTranslations("common");
  const tp = useTranslations("boardPolicy");
  const { user, loading: authLoading } = useAuth();
  const router = useRouter();
  const params = useParams<{ slug: string }>();
  const slug = params.slug;

  const [page, setPage] = useState<PageAdminData | null>(null);
  const [members, setMembers] = useState<PageMemberEdge[]>([]);
  const [fetching, setFetching] = useState(true);
  const [fetchError, setFetchError] = useState<string | null>(null);

  // Edit info state
  const [editName, setEditName] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [editCategory, setEditCategory] = useState("general");
  const [editAvatarUrl, setEditAvatarUrl] = useState<string | null>(null);
  const [editCoverUrl, setEditCoverUrl] = useState<string | null>(null);
  const [uploadingAvatar, setUploadingAvatar] = useState(false);
  const [uploadingCover, setUploadingCover] = useState(false);
  const [savingInfo, setSavingInfo] = useState(false);
  const [infoSaved, setInfoSaved] = useState(false);
  const [infoError, setInfoError] = useState<string | null>(null);
  const avatarInputRef = useRef<HTMLInputElement>(null);
  const coverInputRef = useRef<HTMLInputElement>(null);

  // AP toggle
  const [apEnabled, setApEnabled] = useState(false);
  const [savingAP, setSavingAP] = useState(false);

  // Add member
  const [addUsername, setAddUsername] = useState("");
  const [addRole, setAddRole] = useState<"admin" | "editor">("editor");
  const [addingMember, setAddingMember] = useState(false);
  const [addMemberError, setAddMemberError] = useState<string | null>(null);

  // Delete page
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);

  // Series
  const [seriesList, setSeriesList] = useState<{ id: string; title: string; description: string | null; articleCount: number }[]>([]);
  const [showCreateSeries, setShowCreateSeries] = useState(false);
  const [newSeriesTitle, setNewSeriesTitle] = useState("");
  const [newSeriesDesc, setNewSeriesDesc] = useState("");
  const [creatingSeries, setCreatingSeries] = useState(false);
  const [seriesError, setSeriesError] = useState<string | null>(null);
  const [editSeries, setEditSeries] = useState<{ id: string; title: string; description: string } | null>(null);
  const [savingSeries, setSavingSeries] = useState(false);

  // Active section
  const [activeSection, setActiveSection] = useState<"info" | "policy" | "members" | "series" | "danger">("info");

  useEffect(() => {
    if (authLoading) return;
    if (!user) {
      router.replace(`/login?redirect=/p/${slug}/admin`);
      return;
    }

    gqlClient<{ page: PageAdminData | null; pageMembers: MemberConnection | null }>(
      PAGE_ADMIN_QUERY,
      { slug }
    )
      .then((data) => {
        if (!data.page) {
          router.replace(`/p/${slug}`);
          return;
        }
        setPage(data.page);
        setMembers(data.pageMembers?.items ?? []);
        setEditName(data.page.name);
        setEditDescription(data.page.description ?? "");
        setEditCategory(data.page.category);
        setEditAvatarUrl(data.page.avatarUrl);
        setEditCoverUrl(data.page.coverUrl);
        setApEnabled(data.page.apEnabled);
        // Load series for this page
        gqlClient<{ pageSeries: { id: string; title: string; description: string | null; articleCount: number }[] }>(
          PAGE_SERIES_QUERY,
          { pageId: data.page.id }
        ).then((s) => setSeriesList(s.pageSeries)).catch(() => {});
      })
      .catch((err) => {
        setFetchError(err instanceof Error ? err.message : "Failed to load page");
      })
      .finally(() => setFetching(false));
  }, [user, authLoading, slug]);

  async function uploadImage(file: File, setUploading: (v: boolean) => void, setUrl: (url: string) => void) {
    setUploading(true);
    const token = getAccessToken();
    const formData = new FormData();
    formData.append("file", file);
    try {
      const res = await fetch("/api/upload", {
        method: "POST",
        headers: token ? { authorization: `Bearer ${token}` } : {},
        body: formData,
      });
      const json = await res.json() as { url?: string; error?: string };
      if (!res.ok || !json.url) throw new Error(json.error ?? "Upload failed");
      setUrl(json.url);
    } catch (err) {
      setInfoError(err instanceof Error ? err.message : "Upload failed");
    } finally {
      setUploading(false);
    }
  }

  async function handleSaveInfo(e: React.FormEvent) {
    e.preventDefault();
    if (!page) return;
    setSavingInfo(true);
    setInfoError(null);
    setInfoSaved(false);
    try {
      const data = await gqlClient<{ updatePage: PageAdminData }>(UPDATE_PAGE_MUTATION, {
        pageId: page.id,
        input: {
          name: editName.trim() || null,
          description: editDescription.trim() || null,
          category: editCategory,
          avatarUrl: editAvatarUrl,
          coverUrl: editCoverUrl,
        },
      });
      setPage((prev) => prev ? { ...prev, ...data.updatePage } : prev);
      setInfoSaved(true);
      setTimeout(() => setInfoSaved(false), 3000);
    } catch (err) {
      setInfoError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setSavingInfo(false);
    }
  }

  async function handleToggleAP() {
    if (!page) return;
    setSavingAP(true);
    const next = !apEnabled;
    setApEnabled(next);
    try {
      await gqlClient(SET_AP_ENABLED_MUTATION, {
        pageId: page.id,
        input: { apEnabled: next },
      });
    } catch {
      setApEnabled(!next);
    } finally {
      setSavingAP(false);
    }
  }

  async function handleAddMember(e: React.FormEvent) {
    e.preventDefault();
    if (!page || !addUsername.trim()) return;
    setAddingMember(true);
    setAddMemberError(null);
    try {
      const userData = await gqlClient<{ userByUsername: { id: string; username: string; displayName: string | null } | null }>(
        FIND_USER_QUERY,
        { username: addUsername.trim() }
      );
      if (!userData.userByUsername) {
        setAddMemberError(`User @${addUsername} not found`);
        return;
      }
      await gqlClient(ADD_MEMBER_MUTATION, {
        pageId: page.id,
        userId: userData.userByUsername.id,
        role: addRole,
      });
      // Refresh members
      setMembers((prev) => {
        const existing = prev.find((m) => m.user.id === userData.userByUsername!.id);
        if (existing) {
          return prev.map((m) =>
            m.user.id === userData.userByUsername!.id ? { ...m, role: addRole } : m
          );
        }
        return [
          ...prev,
          {
            role: addRole,
            joinedAt: new Date().toISOString(),
            user: userData.userByUsername!,
          },
        ];
      });
      setAddUsername("");
    } catch (err) {
      setAddMemberError(err instanceof Error ? err.message : "Failed to add member");
    } finally {
      setAddingMember(false);
    }
  }

  async function handleRemoveMember(userId: string) {
    if (!page) return;
    try {
      await gqlClient(REMOVE_MEMBER_MUTATION, { pageId: page.id, userId });
      setMembers((prev) => prev.filter((m) => m.user.id !== userId));
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed to remove member");
    }
  }

  async function handleDeletePage() {
    if (!page) return;
    setDeleting(true);
    try {
      await gqlClient(DELETE_PAGE_MUTATION, { pageId: page.id });
      router.replace("/settings/pages");
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed to delete page");
      setDeleting(false);
      setConfirmDelete(false);
    }
  }

  async function handleCreateSeries(e: React.FormEvent) {
    e.preventDefault();
    if (!page || !newSeriesTitle.trim()) return;
    setCreatingSeries(true);
    setSeriesError(null);
    try {
      const data = await gqlClient<{ createSeries: { id: string; title: string; description: string | null; articleCount: number } }>(
        CREATE_SERIES_MUTATION,
        { pageId: page.id, title: newSeriesTitle.trim(), description: newSeriesDesc.trim() || undefined }
      );
      setSeriesList((prev) => [...prev, data.createSeries]);
      setNewSeriesTitle("");
      setNewSeriesDesc("");
      setShowCreateSeries(false);
    } catch (err) {
      setSeriesError(err instanceof Error ? err.message : "Failed to create series");
    } finally {
      setCreatingSeries(false);
    }
  }

  async function handleSaveSeries() {
    if (!editSeries) return;
    setSavingSeries(true);
    try {
      await gqlClient(UPDATE_SERIES_MUTATION, {
        id: editSeries.id,
        title: editSeries.title,
        description: editSeries.description || undefined,
      });
      setSeriesList((prev) =>
        prev.map((s) => s.id === editSeries.id ? { ...s, title: editSeries.title, description: editSeries.description || null } : s)
      );
      setEditSeries(null);
    } catch (err) {
      setSeriesError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setSavingSeries(false);
    }
  }

  async function handleDeleteSeries(id: string) {
    if (!confirm("Delete this series? Articles will be unlinked.")) return;
    try {
      await gqlClient(DELETE_SERIES_MUTATION, { id });
      setSeriesList((prev) => prev.filter((s) => s.id !== id));
    } catch (err) {
      setSeriesError(err instanceof Error ? err.message : "Failed to delete");
    }
  }

  if (authLoading || fetching) {
    return (
      <div className="mx-auto mt-10 max-w-3xl px-4 text-sm text-[#7a8090]">
        {tCommon("loading")}
      </div>
    );
  }

  if (fetchError || !page) {
    return (
      <div className="mx-auto mt-10 max-w-3xl px-4">
        <div className="rounded-xl border border-red-900/50 bg-red-950/30 px-4 py-3 text-sm text-red-400">
          {fetchError ?? "Page not found"}
        </div>
      </div>
    );
  }

  const pagePolicy: PagePolicy = {
    defaultAccess: page.defaultAccess,
    minTrustLevel: page.minTrustLevel,
    commentPolicy: page.commentPolicy,
    minCommentTrust: page.minCommentTrust,
    requireVcs: page.requireVcs,
    requireCommentVcs: page.requireCommentVcs,
  };

  const navItems: { id: typeof activeSection; label: string }[] = [
    { id: "info", label: t("adminPanelInfo") },
    { id: "policy", label: t("policySection") },
    { id: "members", label: t("members") },
    { id: "series", label: "Series" },
    { id: "danger", label: t("dangerZone") },
  ];

  return (
    <div className="mx-auto mt-6 max-w-3xl px-4 pb-20">
      {/* Breadcrumb */}
      <nav className="mb-6 flex items-center gap-2 text-sm text-[#7a8090]">
        <Link href="/" className="hover:text-[#c8cdd8] transition-colors">Feed</Link>
        <span>›</span>
        <Link href={`/p/${slug}`} className="hover:text-[#c8cdd8] transition-colors">
          {page.name}
        </Link>
        <span>›</span>
        <span className="text-[#c8cdd8]">{t("adminPanel")}</span>
      </nav>

      <h1 className="mb-6 font-serif text-2xl text-[#f3f5f9]">
        {t("adminPanel")} — {page.name}
      </h1>

      {/* Tab navigation */}
      <div className="mb-8 flex gap-1 overflow-x-auto border-b border-[#2a2e38]">
        {navItems.map((item) => (
          <button
            key={item.id}
            onClick={() => setActiveSection(item.id)}
            className={`shrink-0 px-4 py-2 text-sm transition-colors ${
              activeSection === item.id
                ? "border-b-2 border-[#f09a45] text-[#f3f5f9]"
                : "text-[#7a8090] hover:text-[#c8cdd8]"
            }`}
          >
            {item.label}
          </button>
        ))}
      </div>

      {/* ── Info section ── */}
      {activeSection === "info" && (
        <div className="space-y-6">
          <form onSubmit={handleSaveInfo} className="space-y-5 rounded-2xl border border-[#2a2e38] bg-[#0f1117] p-6">
            <h2 className="font-semibold text-[#f3f5f9]">{t("adminPanelInfo")}</h2>

            {/* Cover image */}
            <div>
              <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-[#7a8090]">
                Cover image
              </label>
              <div
                className="relative h-28 w-full cursor-pointer overflow-hidden rounded-lg border border-dashed border-[#2a2e38] bg-[#171b24] hover:border-[#f09a45] transition-colors"
                onClick={() => coverInputRef.current?.click()}
              >
                {editCoverUrl ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img src={editCoverUrl} alt="" className="h-full w-full object-cover" />
                ) : (
                  <div className="flex h-full items-center justify-center text-xs text-[#555c6e]">
                    Click to upload cover (recommended 1200×400)
                  </div>
                )}
                {uploadingCover && (
                  <div className="absolute inset-0 flex items-center justify-center bg-black/60 text-xs text-white">
                    Uploading…
                  </div>
                )}
              </div>
              <input
                ref={coverInputRef}
                type="file"
                accept="image/*"
                className="hidden"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) void uploadImage(file, setUploadingCover, setEditCoverUrl);
                  e.target.value = "";
                }}
              />
              {editCoverUrl && (
                <button
                  type="button"
                  onClick={() => setEditCoverUrl(null)}
                  className="mt-1 text-xs text-[#7a8090] hover:text-red-400 transition-colors"
                >
                  Remove cover
                </button>
              )}
            </div>

            {/* Avatar */}
            <div>
              <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-[#7a8090]">
                Avatar
              </label>
              <div className="flex items-center gap-4">
                <div
                  className="relative flex h-16 w-16 cursor-pointer items-center justify-center overflow-hidden rounded-full border border-dashed border-[#2a2e38] bg-[#171b24] hover:border-[#f09a45] transition-colors shrink-0"
                  onClick={() => avatarInputRef.current?.click()}
                >
                  {editAvatarUrl ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={editAvatarUrl} alt="" className="h-full w-full object-cover" />
                  ) : (
                    <span className="text-lg text-[#555c6e]">
                      {(editName[0] || "P").toUpperCase()}
                    </span>
                  )}
                  {uploadingAvatar && (
                    <div className="absolute inset-0 flex items-center justify-center rounded-full bg-black/60 text-[10px] text-white">
                      …
                    </div>
                  )}
                </div>
                <div className="text-xs text-[#7a8090]">
                  <p>Click the circle to upload an avatar.</p>
                  {editAvatarUrl && (
                    <button
                      type="button"
                      onClick={() => setEditAvatarUrl(null)}
                      className="mt-1 text-red-400 hover:text-red-300 transition-colors"
                    >
                      Remove avatar
                    </button>
                  )}
                </div>
              </div>
              <input
                ref={avatarInputRef}
                type="file"
                accept="image/*"
                className="hidden"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) void uploadImage(file, setUploadingAvatar, setEditAvatarUrl);
                  e.target.value = "";
                }}
              />
            </div>

            <div>
              <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-[#7a8090]">
                {t("pageName")}
              </label>
              <input
                value={editName}
                onChange={(e) => setEditName(e.target.value)}
                className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] focus:border-[#f09a45] focus:outline-none"
                maxLength={100}
              />
            </div>

            <div>
              <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-[#7a8090]">
                {t("pageDescription")}
              </label>
              <textarea
                value={editDescription}
                onChange={(e) => setEditDescription(e.target.value)}
                rows={3}
                className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] focus:border-[#f09a45] focus:outline-none resize-none"
                maxLength={500}
              />
            </div>

            <div>
              <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-[#7a8090]">
                {t("pageCategory")}
              </label>
              <select
                value={editCategory}
                onChange={(e) => setEditCategory(e.target.value)}
                className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] focus:border-[#f09a45] focus:outline-none"
              >
                {CATEGORIES.map((c) => (
                  <option key={c} value={c}>{c.charAt(0).toUpperCase() + c.slice(1)}</option>
                ))}
              </select>
            </div>

            {infoError && <p className="text-sm text-red-400">{infoError}</p>}

            <div className="flex items-center gap-4">
              <button
                type="submit"
                disabled={savingInfo}
                className="rounded-md bg-[#f09a45] px-5 py-2 text-sm font-medium text-[#0b0d12] hover:bg-[#fbb468] disabled:opacity-50 transition-colors"
              >
                {savingInfo ? tp("saving") : tCommon("save")}
              </button>
              {infoSaved && <span className="text-sm text-emerald-400">{tp("saved")}</span>}
            </div>
          </form>

          {/* ActivityPub toggle */}
          <div className="rounded-2xl border border-[#2a2e38] bg-[#0f1117] p-6">
            <div className="flex items-center justify-between">
              <div>
                <h2 className="font-semibold text-[#f3f5f9]">{t("apEnabled")}</h2>
                <p className="mt-1 text-xs text-[#7a8090]">{t("apEnabledDesc")}</p>
              </div>
              <button
                onClick={handleToggleAP}
                disabled={savingAP}
                className={`relative h-6 w-11 rounded-full transition-colors disabled:opacity-50 ${
                  apEnabled ? "bg-[#f09a45]" : "bg-[#2a2e38]"
                }`}
              >
                <span
                  className={`absolute top-0.5 h-5 w-5 rounded-full bg-white transition-transform ${
                    apEnabled ? "translate-x-5" : "translate-x-0.5"
                  }`}
                />
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ── Policy section ── */}
      {activeSection === "policy" && (
        <PagePolicyForm pageId={page.id} initial={pagePolicy} />
      )}

      {/* ── Members section ── */}
      {activeSection === "members" && (
        <div className="space-y-6">
          {/* Current members */}
          <div className="rounded-2xl border border-[#2a2e38] bg-[#0f1117] p-6">
            <h2 className="mb-4 font-semibold text-[#f3f5f9]">{t("members")}</h2>
            {members.length === 0 ? (
              <p className="text-sm text-[#7a8090]">{tCommon("noData")}</p>
            ) : (
              <ul className="space-y-3">
                {members.map((m) => (
                  <li key={m.user.id} className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <div className="flex h-8 w-8 items-center justify-center rounded-full bg-[var(--app-border-hover)] text-sm text-[#f3f5f9]">
                        {(m.user.displayName ?? m.user.username)[0].toUpperCase()}
                      </div>
                      <div>
                        <p className="text-sm text-[#e6e7ea]">
                          {m.user.displayName ?? m.user.username}
                        </p>
                        <p className="text-xs text-[#7a8090]">@{m.user.username}</p>
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                        m.role === "admin"
                          ? "bg-[#f09a45]/20 text-[#f09a45]"
                          : "bg-[#2a2e38] text-[#9ea4b0]"
                      }`}>
                        {m.role === "admin" ? t("roleAdmin") : t("roleEditor")}
                      </span>
                      {m.user.username !== user?.username && (
                        <button
                          onClick={() => handleRemoveMember(m.user.id)}
                          className="text-xs text-[#7a8090] hover:text-red-400 transition-colors"
                        >
                          {t("removeMember")}
                        </button>
                      )}
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </div>

          {/* Add member */}
          <form
            onSubmit={handleAddMember}
            className="rounded-2xl border border-[#2a2e38] bg-[#0f1117] p-6 space-y-4"
          >
            <h2 className="font-semibold text-[#f3f5f9]">{t("addMember")}</h2>
            <div className="flex gap-3">
              <div className="flex flex-1 items-center rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 focus-within:border-[#f09a45]">
                <span className="mr-1 text-sm text-[#555c6e]">@</span>
                <input
                  value={addUsername}
                  onChange={(e) => setAddUsername(e.target.value)}
                  placeholder="username"
                  className="flex-1 bg-transparent text-sm text-[#e6e7ea] placeholder-[#555c6e] focus:outline-none"
                />
              </div>
              <select
                value={addRole}
                onChange={(e) => setAddRole(e.target.value as "admin" | "editor")}
                className="rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] focus:border-[#f09a45] focus:outline-none"
              >
                <option value="editor">{t("roleEditor")}</option>
                <option value="admin">{t("roleAdmin")}</option>
              </select>
              <button
                type="submit"
                disabled={addingMember || !addUsername.trim()}
                className="rounded-md bg-[#f09a45] px-4 py-2 text-sm font-medium text-[#0b0d12] hover:bg-[#fbb468] disabled:opacity-50 transition-colors"
              >
                {t("addMember")}
              </button>
            </div>
            {addMemberError && <p className="text-xs text-red-400">{addMemberError}</p>}
          </form>
        </div>
      )}

      {/* ── Series section ── */}
      {activeSection === "series" && (
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <h2 className="font-serif text-xl text-[#f3f5f9]">Article Series</h2>
            <button
              onClick={() => setShowCreateSeries(true)}
              className="rounded-md bg-[#f09a45] px-4 py-1.5 text-sm font-medium text-[#0b0d12] hover:bg-[#fbb468] transition-colors"
            >
              + New series
            </button>
          </div>

          {seriesError && <p className="text-xs text-red-400">{seriesError}</p>}

          {showCreateSeries && (
            <form onSubmit={handleCreateSeries} className="rounded-2xl border border-[#2a2e38] bg-[#0f1117] p-5 space-y-4">
              <h3 className="font-medium text-[#f3f5f9]">New series</h3>
              <input
                value={newSeriesTitle}
                onChange={(e) => setNewSeriesTitle(e.target.value)}
                placeholder="Series title"
                required
                className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] focus:border-[#f09a45] focus:outline-none"
              />
              <textarea
                value={newSeriesDesc}
                onChange={(e) => setNewSeriesDesc(e.target.value)}
                placeholder="Description (optional)"
                rows={2}
                className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] focus:border-[#f09a45] focus:outline-none resize-none"
              />
              <div className="flex gap-2">
                <button type="submit" disabled={creatingSeries || !newSeriesTitle.trim()}
                  className="rounded-md bg-[#f09a45] px-4 py-1.5 text-sm font-medium text-[#0b0d12] hover:bg-[#fbb468] disabled:opacity-50 transition-colors">
                  {creatingSeries ? "Creating…" : "Create"}
                </button>
                <button type="button" onClick={() => setShowCreateSeries(false)}
                  className="text-sm text-[#7a8090] hover:text-[#c8cdd8] transition-colors">
                  Cancel
                </button>
              </div>
            </form>
          )}

          {seriesList.length === 0 ? (
            <div className="rounded-xl border border-[#2a2e38] bg-[#0f1117] px-6 py-10 text-center text-sm text-[#7a8090]">
              No series yet. Create one to group your articles.
            </div>
          ) : (
            <div className="space-y-3">
              {seriesList.map((s) => (
                <div key={s.id} className="rounded-xl border border-[#2a2e38] bg-[#0f1117] p-4">
                  {editSeries?.id === s.id ? (
                    <div className="space-y-3">
                      <input
                        value={editSeries.title}
                        onChange={(e) => setEditSeries({ ...editSeries, title: e.target.value })}
                        className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] focus:border-[#f09a45] focus:outline-none"
                      />
                      <textarea
                        value={editSeries.description}
                        onChange={(e) => setEditSeries({ ...editSeries, description: e.target.value })}
                        rows={2}
                        className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] focus:border-[#f09a45] focus:outline-none resize-none"
                      />
                      <div className="flex gap-2">
                        <button onClick={handleSaveSeries} disabled={savingSeries}
                          className="rounded-md bg-[#f09a45] px-3 py-1 text-xs font-medium text-[#0b0d12] hover:bg-[#fbb468] disabled:opacity-50">
                          Save
                        </button>
                        <button onClick={() => setEditSeries(null)}
                          className="text-xs text-[#7a8090] hover:text-[#c8cdd8]">
                          Cancel
                        </button>
                      </div>
                    </div>
                  ) : (
                    <div className="flex items-start justify-between gap-4">
                      <div>
                        <p className="font-medium text-[#e6e7ea]">{s.title}</p>
                        {s.description && <p className="mt-0.5 text-xs text-[#7a8090]">{s.description}</p>}
                        <p className="mt-1 text-xs text-[#555c6e]">{s.articleCount} article(s)</p>
                      </div>
                      <div className="flex shrink-0 gap-3">
                        <button
                          onClick={() => setEditSeries({ id: s.id, title: s.title, description: s.description ?? "" })}
                          className="text-xs text-[#7a8090] hover:text-[#c8cdd8] transition-colors"
                        >
                          Edit
                        </button>
                        <button
                          onClick={() => void handleDeleteSeries(s.id)}
                          className="text-xs text-red-400/70 hover:text-red-400 transition-colors"
                        >
                          Delete
                        </button>
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* ── Danger zone ── */}
      {activeSection === "danger" && (
        <div className="rounded-2xl border border-red-900/50 bg-[#0f1117] p-6">
          <h2 className="mb-4 font-semibold text-red-400">{t("dangerZone")}</h2>
          <p className="mb-6 text-sm text-[#7a8090]">
            Permanently delete this page. All posts and articles attributed to this page will be preserved but unlinked.
          </p>
          {!confirmDelete ? (
            <button
              onClick={() => setConfirmDelete(true)}
              className="rounded-md border border-red-900/50 px-4 py-2 text-sm text-red-400 hover:bg-red-950/30 transition-colors"
            >
              Delete this page
            </button>
          ) : (
            <div className="space-y-3">
              <p className="text-sm font-medium text-red-400">
                Are you sure? This cannot be undone.
              </p>
              <div className="flex gap-3">
                <button
                  onClick={handleDeletePage}
                  disabled={deleting}
                  className="rounded-md bg-red-700 px-4 py-2 text-sm font-medium text-white hover:bg-red-600 disabled:opacity-50 transition-colors"
                >
                  {deleting ? "Deleting…" : "Yes, delete permanently"}
                </button>
                <button
                  onClick={() => setConfirmDelete(false)}
                  className="text-sm text-[#7a8090] hover:text-[#c8cdd8] transition-colors"
                >
                  {tCommon("cancel")}
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
