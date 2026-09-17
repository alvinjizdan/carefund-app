export default function Loading() {
  return (
    <div className="flex flex-1 items-center justify-center min-h-[50vh]">
      <div className="h-8 w-8 animate-spin rounded-full border-2 border-emerald-600 border-t-transparent" />
      <span className="sr-only">Loading...</span>
    </div>
  );
}
