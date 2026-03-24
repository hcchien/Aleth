// Root loading skeleton — shown while any server page is streaming.

function SkeletonLine({ w = "w-full", h = "h-4" }: { w?: string; h?: string }) {
  return (
    <div className={`${w} ${h} rounded-lg bg-[var(--app-surface-4)] animate-pulse`} />
  );
}

function PostSkeleton() {
  return (
    <div className="flex gap-3 sm:gap-6">
      {/* Avatar */}
      <div className="h-10 w-10 sm:h-14 sm:w-14 shrink-0 rounded-xl bg-[var(--app-surface-4)] animate-pulse" />
      <div className="flex-1 space-y-3">
        <div className="flex items-center gap-2">
          <SkeletonLine w="w-24" h="h-3" />
          <SkeletonLine w="w-12" h="h-3" />
        </div>
        <SkeletonLine w="w-3/4" h="h-6" />
        <SkeletonLine w="w-full" h="h-4" />
        <SkeletonLine w="w-2/3" h="h-4" />
        <div className="flex gap-4 pt-1">
          <SkeletonLine w="w-16" h="h-3" />
          <SkeletonLine w="w-16" h="h-3" />
        </div>
      </div>
    </div>
  );
}

export default function Loading() {
  return (
    <div className="pt-20 pb-24 px-4 sm:px-6 md:px-10 lg:px-16 md:ml-64 xl:mr-80">
      {/* Page heading skeleton */}
      <div className="mb-12 space-y-3">
        <SkeletonLine w="w-48" h="h-10" />
        <SkeletonLine w="w-72" h="h-4" />
      </div>

      {/* Post skeletons */}
      <div className="space-y-16">
        {[0, 1, 2, 3].map((i) => (
          <PostSkeleton key={i} />
        ))}
      </div>
    </div>
  );
}
