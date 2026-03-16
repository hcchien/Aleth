"use client";

import { useEffect, useState } from "react";
import { useRouter, useParams } from "next/navigation";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth-context";
import { gqlClient } from "@/lib/gql-client";
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
      edges {
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
  edges: PageMemberEdge[];
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
  const [savingInfo, setSavingInfo] = useState(false);
  const [infoSaved, setInfoSaved] = useState(false);
  const [infoError, setInfoError] = useState<string | null>(null);

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

  // Active section
  const [activeSection, setActiveSection] = useState<"info" | "policy" | "members" | "danger">("info");

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
        setMembers(data.pageMembers?.edges ?? []);
        setEditName(data.page.name);
        setEditDescription(data.page.description ?? "");
        setEditCategory(data.page.category);
        setApEnabled(data.page.apEnabled);
      })
      .catch((err) => {
        setFetchError(err instanceof Error ? err.message : "Failed to load page");
      })
      .finally(() => setFetching(false));
  }, [user, authLoading, slug]);

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
