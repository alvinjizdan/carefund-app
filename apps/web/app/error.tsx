"use client";

import { useEffect } from "react";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // Log client error for debugging
    console.error("App error:", error);
  }, [error]);

  return (
    <div className="flex flex-1 flex-col items-center justify-center p-6 text-center">
      <h2 className="text-xl font-bold text-slate-900 dark:text-slate-100">
        Something went wrong
      </h2>
      <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
        An unexpected error occurred.
      </p>
      <button
        onClick={() => reset()}
        className="mt-4 rounded-lg bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-700 transition-colors"
      >
        Try again
      </button>
    </div>
  );
}
