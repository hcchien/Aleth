import Link from "next/link";
import { SubscribeButton } from "./subscribe-button";

interface BoardHeaderProps {
  board: {
    id: string;
    name: string;
    description: string | null;
    subscriberCount: number;
    isSubscribed: boolean;
    owner: { id: string; username: string; displayName: string | null };
  };
}

export function BoardHeader({ board }: BoardHeaderProps) {
  const ownerName = board.owner.displayName ?? board.owner.username;
  return (
    <div className="mb-8 rounded-2xl border border-[#2a2e38] bg-[#0f1117] p-6">
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 min-w-0">
          <h1 className="mb-1 font-serif text-3xl text-[#f3f5f9]">
            {board.name}
          </h1>
          <p className="mb-1 text-sm text-[#7a8090]">
            by{" "}
            <Link
              href={`/@${board.owner.username}`}
              className="text-[#aeb4bf] hover:text-white"
            >
              {ownerName}
            </Link>{" "}
            · @{board.owner.username}
          </p>
          {board.description && (
            <p className="mt-3 text-sm leading-relaxed text-[#c8cdd8]">
              {board.description}
            </p>
          )}
        </div>
        <SubscribeButton
          ownerID={board.owner.id}
          initialSubscribed={board.isSubscribed}
          initialCount={board.subscriberCount}
        />
      </div>
    </div>
  );
}
