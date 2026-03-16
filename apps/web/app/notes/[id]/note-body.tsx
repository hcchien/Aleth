"use client";

import DOMPurify from "dompurify";

export function NoteBody({ html }: { html: string }) {
  const clean = DOMPurify.sanitize(html);
  return (
    <div
      className="prose prose-invert max-w-none text-base leading-relaxed"
      // eslint-disable-next-line react/no-danger
      dangerouslySetInnerHTML={{ __html: clean }}
    />
  );
}
