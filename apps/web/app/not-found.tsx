import Link from "next/link";

export default function NotFound() {
  return (
    <div className="flex flex-1 flex-col items-center justify-center p-6 text-center">
      <h2 className="text-2xl font-bold text-slate-900 dark:text-slate-100">404 - Page Not Found</h2>
      <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
        The page you are looking for does not exist.
      </p>
      <Link
        href="/"
        className="mt-4 rounded-lg bg-emerald-600 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-700 transition-colors"
      >
        Return Home
      </Link>
    </div>
  );
}
