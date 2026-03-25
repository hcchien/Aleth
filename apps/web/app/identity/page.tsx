import { ForumShell } from "@/app/components/forum-shell";
import { IdentityClient } from "@/app/components/identity-client";

// Identity overview page — shows DID, trust level, verification stamps
export default function IdentityPage() {
  return (
    <ForumShell>
      <IdentityClient />
    </ForumShell>
  );
}
