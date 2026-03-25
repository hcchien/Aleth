import { ForumShell } from "@/app/components/forum-shell";
import { ComposeForm } from "./compose-form";

export default function ComposePage() {
  return (
    <ForumShell>
      <ComposeForm />
    </ForumShell>
  );
}
