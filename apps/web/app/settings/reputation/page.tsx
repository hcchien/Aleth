import { ForumShell } from "@/app/components/forum-shell";
import { ReputationClient } from "./reputation-client";

export default function ReputationPage() {
  return (
    <ForumShell>
      <ReputationClient />
    </ForumShell>
  );
}
