"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { gqlClient } from "@/lib/gql-client";

const CREATE_PAGE_MUTATION = `
  mutation CreatePage($input: CreatePageInput!) {
    createPage(input: $input) {
      id
      slug
      name
      category
    }
  }
`;

const CATEGORIES = ["general", "music", "sports", "tech", "art", "gaming", "politics", "education", "other"];

interface CreatedPage {
  id: string;
  slug: string;
  name: string;
  category: string;
}

interface Props {
  onCreated: (page: CreatedPage) => void;
  onClose: () => void;
}

export function CreatePageModal({ onCreated, onClose }: Props) {
  const t = useTranslations("fanPage");
  const tCommon = useTranslations("common");
  const [slug, setSlug] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState("general");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const slugValid = /^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$/.test(slug);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!slug.trim() || !name.trim()) return;
    setSaving(true);
    setError(null);
    try {
      const data = await gqlClient<{ createPage: CreatedPage }>(CREATE_PAGE_MUTATION, {
        input: {
          slug: slug.trim(),
          name: name.trim(),
          description: description.trim() || null,
          category,
        },
      });
      onCreated(data.createPage);
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to create page";
      setError(msg.includes("slug") ? t("slugTaken") : msg);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
      <div className="w-full max-w-md rounded-2xl border border-[#2a2e38] bg-[#0f1117] p-6 shadow-2xl">
        <div className="mb-6 flex items-center justify-between">
          <h2 className="font-serif text-xl text-[#f3f5f9]">{t("createPage")}</h2>
          <button
            onClick={onClose}
            className="text-[#7a8090] hover:text-[#c8cdd8] transition-colors text-lg"
          >
            ✕
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-[#7a8090]">
              {t("pageSlug")}
            </label>
            <div className="flex items-center rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 focus-within:border-[#f09a45]">
              <span className="mr-1 text-sm text-[#555c6e]">/p/</span>
              <input
                value={slug}
                onChange={(e) => setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ""))}
                placeholder="my-band"
                className="flex-1 bg-transparent text-sm text-[#e6e7ea] placeholder-[#555c6e] focus:outline-none font-mono"
                maxLength={64}
              />
            </div>
            {slug && !slugValid && (
              <p className="mt-1 text-xs text-amber-400">
                3–64 chars, lowercase letters, numbers, and hyphens only
              </p>
            )}
          </div>

          <div>
            <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-[#7a8090]">
              {t("pageName")}
            </label>
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t("pageName")}
              className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] placeholder-[#555c6e] focus:border-[#f09a45] focus:outline-none"
              maxLength={100}
            />
          </div>

          <div>
            <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-[#7a8090]">
              {t("pageDescription")}
            </label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={t("pageDescription")}
              rows={3}
              className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] placeholder-[#555c6e] focus:border-[#f09a45] focus:outline-none resize-none"
              maxLength={500}
            />
          </div>

          <div>
            <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-[#7a8090]">
              {t("pageCategory")}
            </label>
            <select
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              className="w-full rounded-md border border-[#2a2e38] bg-[#171b24] px-3 py-2 text-sm text-[#e6e7ea] focus:border-[#f09a45] focus:outline-none"
            >
              {CATEGORIES.map((c) => (
                <option key={c} value={c}>
                  {c.charAt(0).toUpperCase() + c.slice(1)}
                </option>
              ))}
            </select>
          </div>

          {error && (
            <div className="rounded-lg border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
              {error}
            </div>
          )}

          <div className="flex gap-3 pt-2">
            <button
              type="submit"
              disabled={saving || !slug.trim() || !name.trim() || !slugValid}
              className="flex-1 rounded-md bg-[#f09a45] px-4 py-2 text-sm font-medium text-[#0b0d12] hover:bg-[#fbb468] disabled:opacity-50 transition-colors"
            >
              {saving ? tCommon("loading") : t("createPage")}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="rounded-md border border-[#2a2e38] px-4 py-2 text-sm text-[#7a8090] hover:text-[#c8cdd8] transition-colors"
            >
              {tCommon("cancel")}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
