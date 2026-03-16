import { marked } from "marked";

// Configure marked with safe defaults (no raw HTML pass-through)
marked.setOptions({
  gfm: true,
  breaks: true,
});

export function renderMarkdown(md: string): string {
  return marked.parse(md) as string;
}
