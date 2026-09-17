export default function HomePage() {
  return (
    <main className="flex flex-1 flex-col items-center justify-center p-8 text-center">
      <div className="max-w-xl rounded-xl border border-slate-200 bg-white p-8 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div className="inline-flex items-center gap-2 rounded-full bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-400">
          <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
          Frontend Scaffolding Ready
        </div>
        <h1 className="mt-4 text-3xl font-bold tracking-tight text-slate-900 dark:text-slate-50">
          CareFund Web
        </h1>
        <p className="mt-3 text-sm text-slate-600 dark:text-slate-400">
          Transparent Medical &amp; Social Crowdfunding Platform.
          Next.js App Router foundation initialized.
        </p>
        <div className="mt-6 border-t border-slate-100 pt-4 text-xs text-slate-400 dark:border-slate-800 dark:text-slate-500">
          Phase F1.1 Scaffolding Baseline
        </div>
      </div>
    </main>
  );
}
