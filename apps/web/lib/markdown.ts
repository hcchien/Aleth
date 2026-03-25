// Dependencies: npm install isomorphic-dompurify
// (isomorphic-dompurify wraps dompurify with jsdom for SSR/Node.js compatibility)
import { marked } from "marked";
import DOMPurify from "isomorphic-dompurify";

// Configure marked with safe defaults (no raw HTML pass-through)
marked.setOptions({
  gfm: true,
  breaks: true,
});

export function renderMarkdown(md: string): string {
  const rawHtml = marked.parse(md) as string;
  return DOMPurify.sanitize(rawHtml, {
    ALLOWED_TAGS: [
      "p", "br", "strong", "em", "code", "pre", "blockquote",
      "ul", "ol", "li", "h1", "h2", "h3", "h4", "h5", "h6",
      "a", "img", "hr", "table", "thead", "tbody", "tr", "th", "td",
    ],
    ALLOWED_ATTR: ["href", "src", "alt", "title", "class"],
    FORCE_BODY: true,
  });
}
