import Link from "next/link";
import { getPublicCampaigns, type PublicCampaign } from "@/lib/api/public/campaigns";
import { getCategories, type Category } from "@/lib/api/public/categories";
import { Container } from "@/components/layout/container";
import { PageHeader } from "@/components/layout/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/feedback/alert";
import { EmptyState } from "@/components/feedback/empty-state";
import { CampaignExploreCard } from "@/components/campaigns/campaign-explore-card";
import type { PaginatedResult } from "@/lib/api/types";
import type { Metadata } from "next";

export const revalidate = 60;

export const metadata: Metadata = {
  title: "Explore Campaigns — CareFund",
  description: "Discover and support verified crowdfunding campaigns making a real impact.",
};

interface ExplorePageProps {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}

export default async function ExplorePage({ searchParams }: ExplorePageProps) {
  // 1. Resolve query parameters for pagination
  const resolvedParams = await searchParams;
  const rawPage = resolvedParams.page;
  let page = 1;
  if (typeof rawPage === "string") {
    const parsed = parseInt(rawPage, 10);
    if (Number.isInteger(parsed) && parsed > 0) {
      page = parsed;
    }
  }

  const limit = 12;
  const offset = (page - 1) * limit;

  // 2. Parallel server-side fetching of categories and active public campaigns
  let categories: Category[] = [];
  let campaignResult: PaginatedResult<PublicCampaign> = {
    items: [],
    limit,
    offset,
    hasMore: false,
  };
  let fetchError = false;

  try {
    const [categoriesData, campaignsData] = await Promise.all([
      getCategories().catch(() => [] as Category[]),
      getPublicCampaigns({ limit, offset }),
    ]);
    categories = categoriesData;
    campaignResult = campaignsData;
  } catch {
    fetchError = true;
  }

  // 3. Build category lookup map for presentation
  const categoriesMap = new Map<string, string>();
  for (const cat of categories) {
    categoriesMap.set(cat.id, cat.name);
  }

  const startRecord = campaignResult.items.length > 0 ? offset + 1 : 0;
  const endRecord = offset + campaignResult.items.length;

  return (
    <main className="py-8 sm:py-12">
      <Container size="2xl">
        <PageHeader
          title="Explore Campaigns"
          description="Discover and support transparent medical and social crowdfunding initiatives."
        />

        <div className="space-y-8">
          {/* Informational Category Taxonomy Section */}
          {categories.length > 0 && (
            <section
              aria-label="Supported Campaign Categories"
              className="space-y-3"
            >
              <h2 className="text-xs font-semibold uppercase tracking-wider text-text-muted">
                Supported Causes &amp; Categories
              </h2>
              <div className="flex flex-wrap gap-2">
                {categories.map((cat) => (
                  <Badge key={cat.id} variant="secondary" size="md">
                    {cat.name}
                  </Badge>
                ))}
              </div>
            </section>
          )}

          {/* Campaign Grid, Error State, or Empty State */}
          {fetchError ? (
            <Alert variant="destructive">
              <AlertTitle>Failed to load campaigns</AlertTitle>
              <AlertDescription>
                Unable to load campaigns at this time. Please try refreshing the
                page.
              </AlertDescription>
            </Alert>
          ) : campaignResult.items.length === 0 ? (
            <EmptyState
              title="No active campaigns found"
              description="There are currently no active campaigns seeking support. Check back soon or start a new campaign."
              action={
                <Link href="/campaigns/new">
                  <Button variant="primary">Start a Campaign</Button>
                </Link>
              }
            />
          ) : (
            <div className="space-y-8">
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
                {campaignResult.items.map((campaign) => (
                  <CampaignExploreCard
                    key={campaign.id}
                    campaign={campaign}
                    categoryName={categoriesMap.get(campaign.categoryId)}
                  />
                ))}
              </div>

              {/* Server-driven Pagination Controls */}
              <div className="flex flex-col sm:flex-row items-center justify-between gap-4 pt-4 border-t border-border-subtle">
                <p className="text-xs sm:text-sm text-text-secondary">
                  Showing{" "}
                  <span className="font-medium text-text-primary">
                    {startRecord}
                  </span>{" "}
                  to{" "}
                  <span className="font-medium text-text-primary">
                    {endRecord}
                  </span>
                </p>

                <div className="flex items-center gap-2">
                  {page > 1 ? (
                    <Link href={`/explore?page=${page - 1}`}>
                      <Button variant="outline" size="sm">
                        Previous
                      </Button>
                    </Link>
                  ) : (
                    <Button variant="outline" size="sm" disabled>
                      Previous
                    </Button>
                  )}

                  <span className="text-xs sm:text-sm font-medium text-text-secondary px-2">
                    Page {page}
                  </span>

                  {campaignResult.hasMore ? (
                    <Link href={`/explore?page=${page + 1}`}>
                      <Button variant="outline" size="sm">
                        Next
                      </Button>
                    </Link>
                  ) : (
                    <Button variant="outline" size="sm" disabled>
                      Next
                    </Button>
                  )}
                </div>
              </div>
            </div>
          )}
        </div>
      </Container>
    </main>
  );
}
