"use client";

import * as React from "react";
import { notFound } from "next/navigation";

// Core UI Primitives
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from "@/components/ui/card";
import { Spinner } from "@/components/ui/spinner";
import { Skeleton } from "@/components/ui/skeleton";
import { Divider } from "@/components/ui/divider";

// Form Primitives
import { Input } from "@/components/forms/input";
import { Textarea } from "@/components/forms/textarea";
import { Select } from "@/components/forms/select";
import { Label } from "@/components/forms/label";
import {
  FormField,
  FormLabel,
  FormHelperText,
  FormErrorMessage,
} from "@/components/forms/form-field";

// Feedback Primitives
import {
  Alert,
  AlertTitle,
  AlertDescription,
} from "@/components/feedback/alert";
import { EmptyState } from "@/components/feedback/empty-state";

// Layout Primitives
import { Container } from "@/components/layout/container";
import { Stack } from "@/components/layout/stack";
import { Grid } from "@/components/layout/grid";
import { Section } from "@/components/layout/section";
import { PageHeader } from "@/components/layout/page-header";

// Overlay Primitive
import {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
  DialogClose,
} from "@/components/ui/dialog";

export default function DesignSystemShowcasePage() {
  // Production Route Guard:
  // Must return 404 in production unless NEXT_PUBLIC_ENABLE_DEV_SHOWCASE === "true"
  if (
    process.env.NODE_ENV === "production" &&
    process.env.NEXT_PUBLIC_ENABLE_DEV_SHOWCASE !== "true"
  ) {
    notFound();
  }

  // Theme toggle state for visual verification
  const [isDark, setIsDark] = React.useState(false);

  // Synchronize with document element class
  const toggleTheme = () => {
    setIsDark((prev) => {
      const next = !prev;
      if (next) {
        document.documentElement.classList.add("dark");
      } else {
        document.documentElement.classList.remove("dark");
      }
      return next;
    });
  };

  return (
    <main className="min-h-screen bg-surface-ground text-text-primary transition-colors">
      <Container size="2xl" className="py-8 space-y-12">
        {/* Showcase Header */}
        <PageHeader
          title="CareFund Design System Showcase"
          description="Development & accessibility validation harness for Core UI, Forms, Feedback, Layout, and Dialog primitives."
          action={
            <Button
              variant="outline"
              size="sm"
              onClick={toggleTheme}
              aria-label={`Switch to ${isDark ? "Light" : "Dark"} Mode`}
            >
              {isDark ? "Switch to Light Mode" : "Switch to Dark Mode"}
            </Button>
          }
        />

        {/* SECTION 1: CORE UI PRIMITIVES */}
        <Section variant="card" spacing="lg" className="rounded-2xl border border-border-subtle p-6 sm:p-8">
          <div className="space-y-8">
            <div>
              <h2 className="text-xl font-bold tracking-tight text-text-primary">
                1. Core UI Primitives
              </h2>
              <p className="text-sm text-text-secondary mt-1">
                Foundational interactive and display elements from Phase F2.2.
              </p>
            </div>

            {/* Buttons */}
            <div className="space-y-4">
              <h3 className="text-sm font-semibold text-text-muted uppercase tracking-wider">
                Buttons — Variants & States
              </h3>
              <div className="flex flex-wrap items-center gap-3">
                <Button variant="primary">Primary</Button>
                <Button variant="secondary">Secondary</Button>
                <Button variant="outline">Outline</Button>
                <Button variant="ghost">Ghost</Button>
                <Button variant="destructive">Destructive</Button>
                <Button variant="link">Link Style</Button>
                <Button variant="primary" disabled>
                  Disabled
                </Button>
                <Button variant="primary" isLoading>
                  Loading
                </Button>
              </div>
              <div className="flex flex-wrap items-center gap-3 pt-2">
                <Button size="sm">Small (sm)</Button>
                <Button size="md">Medium (md)</Button>
                <Button size="lg">Large (lg)</Button>
                <Button size="icon" aria-label="Icon button demo">
                  <span aria-hidden="true">★</span>
                </Button>
              </div>
            </div>

            <Divider />

            {/* Badges */}
            <div className="space-y-4">
              <h3 className="text-sm font-semibold text-text-muted uppercase tracking-wider">
                Badges — Semantic Status
              </h3>
              <div className="flex flex-wrap items-center gap-2.5">
                <Badge variant="default">Default</Badge>
                <Badge variant="primary">Primary</Badge>
                <Badge variant="secondary">Secondary</Badge>
                <Badge variant="success">Success</Badge>
                <Badge variant="warning">Warning</Badge>
                <Badge variant="info">Info</Badge>
                <Badge variant="destructive">Destructive</Badge>
                <Badge variant="outline">Outline</Badge>
              </div>
            </div>

            <Divider />

            {/* Cards */}
            <div className="space-y-4">
              <h3 className="text-sm font-semibold text-text-muted uppercase tracking-wider">
                Card — Composition Family
              </h3>
              <Grid cols={2} gap="md">
                <Card>
                  <CardHeader>
                    <CardTitle>Medical Emergency Relief</CardTitle>
                    <CardDescription>
                      Verified clinical support for pediatric care.
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <p className="text-sm text-text-secondary leading-relaxed">
                      Structured card content demonstrates surface elevation, high-contrast text hierarchy, and semantic borders.
                    </p>
                  </CardContent>
                  <CardFooter className="flex justify-between items-center">
                    <span className="text-xs text-text-muted">Target: Rp 50.000.000</span>
                    <Button size="sm">View Details</Button>
                  </CardFooter>
                </Card>

                <Card>
                  <CardHeader>
                    <CardTitle>Community Clinic Grant</CardTitle>
                    <CardDescription>
                      Healthcare accessibility fund allocation.
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <p className="text-sm text-text-secondary leading-relaxed">
                      Second compositional card exhibiting responsive grid layout and uniform padding tokens.
                    </p>
                  </CardContent>
                  <CardFooter className="flex justify-between items-center">
                    <span className="text-xs text-text-muted">Target: Rp 25.000.000</span>
                    <Button variant="outline" size="sm">
                      Read Report
                    </Button>
                  </CardFooter>
                </Card>
              </Grid>
            </div>

            <Divider />

            {/* Spinners & Skeletons */}
            <div className="space-y-4">
              <h3 className="text-sm font-semibold text-text-muted uppercase tracking-wider">
                Loading Indicators & Placeholders
              </h3>
              <div className="flex items-center gap-6">
                <div className="flex items-center gap-2">
                  <Spinner size="sm" />
                  <span className="text-xs text-text-secondary">sm</span>
                </div>
                <div className="flex items-center gap-2">
                  <Spinner size="md" />
                  <span className="text-xs text-text-secondary">md</span>
                </div>
                <div className="flex items-center gap-2">
                  <Spinner size="lg" />
                  <span className="text-xs text-text-secondary">lg</span>
                </div>
              </div>
              <div className="space-y-2 pt-2 max-w-md">
                <Skeleton className="h-4 w-3/4" />
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-1/2" />
              </div>
            </div>

            <Divider />

            {/* Dividers */}
            <div className="space-y-4">
              <h3 className="text-sm font-semibold text-text-muted uppercase tracking-wider">
                Dividers
              </h3>
              <Divider />
              <Divider label="OR" />
              <div className="flex items-center h-8 gap-4 text-sm text-text-secondary">
                <span>Left Section</span>
                <Divider orientation="vertical" />
                <span>Right Section</span>
              </div>
            </div>
          </div>
        </Section>

        {/* SECTION 2: FORMS & ACCESSIBILITY TEST MATRIX */}
        <Section variant="card" spacing="lg" className="rounded-2xl border border-border-subtle p-6 sm:p-8">
          <div className="space-y-8">
            <div>
              <h2 className="text-xl font-bold tracking-tight text-text-primary">
                2. Forms & Accessibility Validation Matrix
              </h2>
              <p className="text-sm text-text-secondary mt-1">
                Form field verification across Cases A through F demonstrating label association, aria-describedby linkage, and error semantics.
              </p>
            </div>

            {/* Standalone Form Controls */}
            <div className="space-y-4">
              <h3 className="text-sm font-semibold text-text-muted uppercase tracking-wider">
                Standalone Controls (Input, Textarea, Select)
              </h3>
              <Grid cols={3} gap="md">
                <div className="space-y-1.5">
                  <Label htmlFor="demo-input-default">Standard Input</Label>
                  <Input id="demo-input-default" placeholder="Enter placeholder text..." />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="demo-input-disabled">Disabled Input</Label>
                  <Input id="demo-input-disabled" disabled value="Read-only disabled value" />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="demo-input-readonly">Read-Only Input</Label>
                  <Input id="demo-input-readonly" readOnly value="System generated reference" />
                </div>
              </Grid>

              <Grid cols={2} gap="md" className="pt-2">
                <div className="space-y-1.5">
                  <Label htmlFor="demo-select">Native Select with Chevron</Label>
                  <Select id="demo-select" defaultValue="idr">
                    <option value="idr">IDR — Indonesian Rupiah</option>
                    <option value="usd">USD — United States Dollar</option>
                    <option value="eur">EUR — Euro</option>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="demo-textarea">Multiline Textarea</Label>
                  <Textarea id="demo-textarea" placeholder="Provide campaign description..." rows={3} />
                </div>
              </Grid>
            </div>

            <Divider />

            {/* Form Field Test Matrix (Cases A through F) */}
            <div className="space-y-6">
              <h3 className="text-sm font-semibold text-text-muted uppercase tracking-wider">
                Form Field Test Matrix (Cases A – F)
              </h3>

              <Grid cols={2} gap="lg">
                {/* CASE A: Control Only */}
                <Card className="p-4 bg-surface-ground">
                  <span className="text-xs font-mono font-semibold text-text-muted uppercase">
                    Case A: Control Only (No Dangling ARIA IDs)
                  </span>
                  <div className="mt-3">
                    <FormField id="case-a-field">
                      <FormLabel>Full Name</FormLabel>
                      <Input id="case-a-field" placeholder="John Doe" />
                    </FormField>
                  </div>
                </Card>

                {/* CASE B: Control + Helper */}
                <Card className="p-4 bg-surface-ground">
                  <span className="text-xs font-mono font-semibold text-text-muted uppercase">
                    Case B: Control + Helper Text
                  </span>
                  <div className="mt-3">
                    <FormField id="case-b-field" hasHelper>
                      <FormLabel>Corporate Email</FormLabel>
                      <Input
                        id="case-b-field"
                        aria-describedby="case-b-field-helper"
                        placeholder="you@institution.org"
                      />
                      <FormHelperText>We send verification links to this address.</FormHelperText>
                    </FormField>
                  </div>
                </Card>

                {/* CASE C: Control + Error */}
                <Card className="p-4 bg-surface-ground">
                  <span className="text-xs font-mono font-semibold text-text-muted uppercase">
                    Case C: Control + Error Message
                  </span>
                  <div className="mt-3">
                    <FormField id="case-c-field" error="National ID must be 16 digits.">
                      <FormLabel>National ID Number</FormLabel>
                      <Input
                        id="case-c-field"
                        aria-invalid="true"
                        aria-describedby="case-c-field-error"
                        defaultValue="12345"
                      />
                      <FormErrorMessage />
                    </FormField>
                  </div>
                </Card>

                {/* CASE D: Control + Helper + Error */}
                <Card className="p-4 bg-surface-ground">
                  <span className="text-xs font-mono font-semibold text-text-muted uppercase">
                    Case D: Control + Helper + Error (Deterministic Order)
                  </span>
                  <div className="mt-3">
                    <FormField
                      id="case-d-field"
                      hasHelper
                      error="Campaign target must be at least Rp 1.000.000."
                    >
                      <FormLabel>Fundraising Target</FormLabel>
                      <Input
                        id="case-d-field"
                        aria-invalid="true"
                        aria-describedby="case-d-field-helper case-d-field-error"
                        defaultValue="500000"
                      />
                      <FormHelperText>Minimum operational threshold applies.</FormHelperText>
                      <FormErrorMessage />
                    </FormField>
                  </div>
                </Card>

                {/* CASE E: Required Control */}
                <Card className="p-4 bg-surface-ground">
                  <span className="text-xs font-mono font-semibold text-text-muted uppercase">
                    Case E: Required Control (Non-Color Indicator)
                  </span>
                  <div className="mt-3">
                    <FormField id="case-e-field" required>
                      <FormLabel>Hospital / Medical Center</FormLabel>
                      <Input
                        id="case-e-field"
                        required
                        placeholder="e.g. RS Cipto Mangunkusumo"
                      />
                    </FormField>
                  </div>
                </Card>

                {/* CASE F: Invalid Required Control */}
                <Card className="p-4 bg-surface-ground">
                  <span className="text-xs font-mono font-semibold text-text-muted uppercase">
                    Case F: Invalid Required Control
                  </span>
                  <div className="mt-3">
                    <FormField
                      id="case-f-field"
                      required
                      error="Medical documentation upload is mandatory."
                    >
                      <FormLabel>Diagnosis Summary</FormLabel>
                      <Textarea
                        id="case-f-field"
                        required
                        aria-invalid="true"
                        aria-describedby="case-f-field-error"
                        placeholder="Provide primary clinical diagnosis..."
                        rows={2}
                      />
                      <FormErrorMessage />
                    </FormField>
                  </div>
                </Card>
              </Grid>
            </div>
          </div>
        </Section>

        {/* SECTION 3: FEEDBACK & STATE PRIMITIVES */}
        <Section variant="card" spacing="lg" className="rounded-2xl border border-border-subtle p-6 sm:p-8">
          <div className="space-y-8">
            <div>
              <h2 className="text-xl font-bold tracking-tight text-text-primary">
                3. Feedback & State Primitives
              </h2>
              <p className="text-sm text-text-secondary mt-1">
                Alerts with semantic roles and presentational EmptyState layout.
              </p>
            </div>

            {/* Alerts */}
            <div className="space-y-3.5">
              <h3 className="text-sm font-semibold text-text-muted uppercase tracking-wider">
                Alerts — Semantic Status Roles
              </h3>

              <Alert variant="default">
                <AlertTitle>Scheduled Maintenance Notice</AlertTitle>
                <AlertDescription>
                  Database re-indexing is planned for Sunday at 02:00 UTC. No donation downtime is expected.
                </AlertDescription>
              </Alert>

              <Alert variant="info">
                <AlertTitle>Identity Verification Pending</AlertTitle>
                <AlertDescription>
                  Your campaign documents have been submitted and are under review by clinical compliance officers.
                </AlertDescription>
              </Alert>

              <Alert variant="success">
                <AlertTitle>Disbursement Completed</AlertTitle>
                <AlertDescription>
                  Batch payout transfer #89211 was acknowledged by the bank switch.
                </AlertDescription>
              </Alert>

              <Alert variant="warning">
                <AlertTitle>Outbox Retry Threshold Approaching</AlertTitle>
                <AlertDescription>
                  Dead-letter queue has 3 pending events requiring manual inspection.
                </AlertDescription>
              </Alert>

              <Alert variant="destructive">
                <AlertTitle>Security Policy Violation</AlertTitle>
                <AlertDescription>
                  Signature validation failed for webhook callback. Origin IP logged.
                </AlertDescription>
              </Alert>
            </div>

            <Divider />

            {/* Empty State */}
            <div className="space-y-4">
              <h3 className="text-sm font-semibold text-text-muted uppercase tracking-wider">
                EmptyState Primitive
              </h3>
              <EmptyState
                icon={
                  <span className="text-2xl" aria-hidden="true">
                    📁
                  </span>
                }
                title="No Active Campaigns Found"
                description="You have not created any fundraising campaigns yet. Start your first initiative to request medical aid."
                action={<Button size="sm">Create New Campaign</Button>}
              />
            </div>
          </div>
        </Section>

        {/* SECTION 4: LAYOUT PRIMITIVES */}
        <Section variant="card" spacing="lg" className="rounded-2xl border border-border-subtle p-6 sm:p-8">
          <div className="space-y-8">
            <div>
              <h2 className="text-xl font-bold tracking-tight text-text-primary">
                4. Layout Primitives
              </h2>
              <p className="text-sm text-text-secondary mt-1">
                Structural primitives from Phase F2.4 (`Container`, `Stack`, `Grid`, `Section`, `PageHeader`).
              </p>
            </div>

            {/* Stack Demonstration */}
            <div className="space-y-4">
              <h3 className="text-sm font-semibold text-text-muted uppercase tracking-wider">
                Stack — Horizontal & Vertical
              </h3>
              <div className="space-y-3">
                <Stack direction="horizontal" gap="sm" align="center" className="p-3 bg-surface-ground rounded-lg">
                  <Badge variant="primary">Item 1</Badge>
                  <Badge variant="secondary">Item 2</Badge>
                  <Badge variant="outline">Item 3</Badge>
                </Stack>
                <Stack direction="vertical" gap="xs" className="p-3 bg-surface-ground rounded-lg">
                  <span className="text-xs text-text-secondary">Vertical Stack Row 1</span>
                  <span className="text-xs text-text-secondary">Vertical Stack Row 2</span>
                  <span className="text-xs text-text-secondary">Vertical Stack Row 3</span>
                </Stack>
              </div>
            </div>

            <Divider />

            {/* Grid Demonstration */}
            <div className="space-y-4">
              <h3 className="text-sm font-semibold text-text-muted uppercase tracking-wider">
                Grid — Configurable Columns
              </h3>
              <Grid cols={4} gap="sm">
                <div className="p-3 text-center text-xs bg-surface-muted rounded-lg font-mono">Col 1</div>
                <div className="p-3 text-center text-xs bg-surface-muted rounded-lg font-mono">Col 2</div>
                <div className="p-3 text-center text-xs bg-surface-muted rounded-lg font-mono">Col 3</div>
                <div className="p-3 text-center text-xs bg-surface-muted rounded-lg font-mono">Col 4</div>
              </Grid>
            </div>
          </div>
        </Section>

        {/* SECTION 5: OVERLAY & DIALOG PRIMITIVE */}
        <Section variant="card" spacing="lg" className="rounded-2xl border border-border-subtle p-6 sm:p-8">
          <div className="space-y-6">
            <div>
              <h2 className="text-xl font-bold tracking-tight text-text-primary">
                5. Overlay & Dialog Primitive
              </h2>
              <p className="text-sm text-text-secondary mt-1">
                Radix-backed modal dialog verifying accessible name, focus trap, Escape dismissal, and focus restoration.
              </p>
            </div>

            <div>
              <Dialog>
                <DialogTrigger asChild>
                  <Button variant="primary">Open Validation Dialog</Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>Confirm Aid Allocation</DialogTitle>
                    <DialogDescription>
                      This action will authorize emergency funds to the designated clinical provider. Please confirm patient record details.
                    </DialogDescription>
                  </DialogHeader>

                  <div className="space-y-3 py-2">
                    <div className="text-sm text-text-secondary space-y-1">
                      <p><strong className="text-text-primary">Recipient Hospital:</strong> RSUP Dr. Sardjito</p>
                      <p><strong className="text-text-primary">Authorized Amount:</strong> Rp 15.000.000</p>
                      <p><strong className="text-text-primary">Verification Ref:</strong> CF-MED-2026-901</p>
                    </div>
                  </div>

                  <DialogFooter>
                    <DialogClose asChild>
                      <Button variant="outline">Cancel</Button>
                    </DialogClose>
                    <DialogClose asChild>
                      <Button variant="primary">Authorize Disbursement</Button>
                    </DialogClose>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </div>
          </div>
        </Section>
      </Container>
    </main>
  );
}
