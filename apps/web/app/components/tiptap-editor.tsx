"use client";

import { useEditor, EditorContent } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Image from "@tiptap/extension-image";

interface TiptapEditorProps {
  value: string;
  onChange: (html: string) => void;
}

export function TiptapEditor({ value, onChange }: TiptapEditorProps) {
  const editor = useEditor({
    extensions: [StarterKit, Image.configure({ allowBase64: false })],
    content: value,
    immediatelyRender: false,
    onUpdate({ editor }) {
      onChange(editor.getHTML());
    },
  });

  if (!editor) return null;

  const btn = (active: boolean, onClick: () => void, label: string) => (
    <button
      type="button"
      onClick={onClick}
      className={`rounded px-2 py-1 text-xs font-mono transition-colors ${
        active
          ? "bg-[#e89246] text-[#0e1117]"
          : "text-[#9ea4b0] hover:bg-[#252933] hover:text-white"
      }`}
    >
      {label}
    </button>
  );

  function insertImage() {
    const url = window.prompt("Image URL");
    if (url) editor?.chain().focus().setImage({ src: url }).run();
  }

  return (
    <div className="rounded-lg border border-[#3a3f48] bg-[#161b22]">
      {/* Toolbar */}
      <div className="flex flex-wrap gap-1 border-b border-[#3a3f48] p-2">
        {btn(editor.isActive("bold"), () => editor.chain().focus().toggleBold().run(), "B")}
        {btn(editor.isActive("italic"), () => editor.chain().focus().toggleItalic().run(), "I")}
        {btn(editor.isActive("heading", { level: 1 }), () => editor.chain().focus().toggleHeading({ level: 1 }).run(), "H1")}
        {btn(editor.isActive("heading", { level: 2 }), () => editor.chain().focus().toggleHeading({ level: 2 }).run(), "H2")}
        {btn(editor.isActive("heading", { level: 3 }), () => editor.chain().focus().toggleHeading({ level: 3 }).run(), "H3")}
        {btn(editor.isActive("blockquote"), () => editor.chain().focus().toggleBlockquote().run(), '"')}
        {btn(editor.isActive("codeBlock"), () => editor.chain().focus().toggleCodeBlock().run(), "<>")}
        {btn(editor.isActive("bulletList"), () => editor.chain().focus().toggleBulletList().run(), "•")}
        {btn(editor.isActive("orderedList"), () => editor.chain().focus().toggleOrderedList().run(), "1.")}
        <button
          type="button"
          onClick={insertImage}
          className="rounded px-2 py-1 text-xs font-mono text-[#9ea4b0] transition-colors hover:bg-[#252933] hover:text-white"
        >
          IMG
        </button>
      </div>

      {/* Editor area */}
      <EditorContent
        editor={editor}
        className="prose prose-invert max-w-none p-4 text-sm focus:outline-none [&_.ProseMirror]:min-h-[200px] [&_.ProseMirror]:outline-none"
      />
    </div>
  );
}
