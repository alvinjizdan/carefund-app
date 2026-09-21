"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { Input } from "@/components/forms/input";
import { Textarea } from "@/components/forms/textarea";
import { Select } from "@/components/forms/select";
import {
  FormField,
  FormLabel,
  FormHelperText,
  FormErrorMessage,
} from "@/components/forms/form-field";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/feedback/alert";
import { CampaignStatusBadge } from "@/components/campaigns/campaign-status-badge";
import { createCampaignAction, updateCampaignAction } from "@/lib/actions/campaigns";
import { formatIDR } from "@/lib/donations/validation";
import type { Category } from "@/lib/api/public/categories";
import type { CampaignDetail } from "@/lib/api/public/campaigns";

export interface CampaignFormProps {
  mode: "create" | "edit";
  categories: Category[];
  initialData?: CampaignDetail;
  isRejected?: boolean;
}

interface FormErrors {
  title?: string;
  categoryId?: string;
  targetAmount?: string;
  startDate?: string;
  endDate?: string;
  description?: string;
  general?: string;
}

export function CampaignForm({
  mode,
  categories,
  initialData,
  isRejected = false,
}: CampaignFormProps) {
  const router = useRouter();

  // Form field states
  const [title, setTitle] = React.useState(initialData?.title ?? "");
  const [categoryId, setCategoryId] = React.useState(
    initialData?.categoryId ?? (categories.length > 0 ? categories[0].id : "")
  );
  const [targetAmountRaw, setTargetAmountRaw] = React.useState(
    initialData ? String(initialData.targetAmount) : ""
  );
  const [startDate, setStartDate] = React.useState(
    initialData?.startAt ? initialData.startAt.slice(0, 10) : ""
  );
  const [endDate, setEndDate] = React.useState(
    initialData?.endAt ? initialData.endAt.slice(0, 10) : ""
  );
  const [description, setDescription] = React.useState(
    initialData?.description ?? ""
  );

  // Status and error states
  const [errors, setErrors] = React.useState<FormErrors>({});
  const [isSubmitting, setIsSubmitting] = React.useState(false);
  const [serverError, setServerError] = React.useState<string | null>(null);

  // Compute live IDR preview from integer string
  const targetAmountPreview = React.useMemo(() => {
    const trimmed = targetAmountRaw.trim();
    if (!trimmed || !/^\d+$/.test(trimmed)) return null;
    const num = Number(trimmed);
    if (!Number.isSafeInteger(num) || num <= 0) return null;
    return formatIDR(num);
  }, [targetAmountRaw]);

  // Frontend UX validation
  const validateForm = (): boolean => {
    const newErrors: FormErrors = {};
    const trimmedTitle = title.trim();
    const trimmedDesc = description.trim();
    const trimmedAmount = targetAmountRaw.trim();

    // Title: 5–200 characters
    if (!trimmedTitle) {
      newErrors.title = "Campaign title is required.";
    } else if (trimmedTitle.length < 5) {
      newErrors.title = "Title must be at least 5 characters.";
    } else if (trimmedTitle.length > 200) {
      newErrors.title = "Title cannot exceed 200 characters.";
    }

    // Category: required selection
    if (!categoryId) {
      newErrors.categoryId = "Please select a category.";
    }

    // Target Amount: IDR positive integer, UX minimum Rp 10,000
    if (!trimmedAmount) {
      newErrors.targetAmount = "Target amount is required.";
    } else if (/[.,]/.test(trimmedAmount)) {
      newErrors.targetAmount =
        "Decimals and fractional amounts are not supported. Enter whole IDR.";
    } else if (/\D/.test(trimmedAmount)) {
      newErrors.targetAmount = "Target amount must contain digits only.";
    } else if (/^0/.test(trimmedAmount)) {
      newErrors.targetAmount = "Target amount cannot start with zero.";
    } else {
      const parsedAmount = Number(trimmedAmount);
      if (!Number.isSafeInteger(parsedAmount) || parsedAmount <= 0) {
        newErrors.targetAmount = "Target amount exceeds the maximum allowable value.";
      } else if (parsedAmount < 10_000) {
        newErrors.targetAmount = "Minimum recommended target amount is Rp 10.000.";
      }
    }

    // Dates
    const today = new Date().toISOString().slice(0, 10);
    if (!startDate) {
      newErrors.startDate = "Start date is required.";
    } else if (mode === "create" && startDate < today) {
      newErrors.startDate = "Start date must be today or a future date.";
    }

    if (!endDate) {
      newErrors.endDate = "End date is required.";
    } else if (startDate && endDate <= startDate) {
      newErrors.endDate = "End date must be after the start date.";
    }

    // Description: 20–10,000 characters
    if (!trimmedDesc) {
      newErrors.description = "Campaign description is required.";
    } else if (trimmedDesc.length < 20) {
      newErrors.description = "Description must be at least 20 characters.";
    } else if (trimmedDesc.length > 10_000) {
      newErrors.description = "Description cannot exceed 10,000 characters.";
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const mapBackendErrorCode = (code: string, defaultMessage: string): string => {
    switch (code) {
      case "INVALID_REQUEST":
        return "The submitted campaign data is invalid. Please review the highlighted fields and try again.";
      case "UNAUTHORIZED":
        return "Your session has expired. Please log in again.";
      case "FORBIDDEN":
        return "You do not have permission to modify this campaign.";
      case "NOT_FOUND":
        return "The campaign could not be found.";
      case "INVALID_STATE_TRANSITION":
        return "This campaign cannot be modified in its current status.";
      case "TOO_MANY_REQUESTS":
        return "Too many requests. Please wait a moment before trying again.";
      default:
        return defaultMessage || "An unexpected error occurred. Please try again.";
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setServerError(null);

    if (!validateForm() || isSubmitting) {
      return;
    }

    setIsSubmitting(true);

    const parsedTarget = Number(targetAmountRaw.trim());
    // Safe ISO-8601 formatting for backend RFC3339 compatibility
    const startAt = new Date(`${startDate}T00:00:00Z`).toISOString();
    const endAt = new Date(`${endDate}T23:59:59Z`).toISOString();

    try {
      if (mode === "create") {
        const result = await createCampaignAction({
          title: title.trim(),
          categoryId,
          targetAmount: parsedTarget,
          startAt,
          endAt,
          description: description.trim(),
        });

        if (!result.ok) {
          setServerError(
            mapBackendErrorCode(result.error.code, result.error.message)
          );
          setIsSubmitting(false);
          return;
        }

        router.push(`/campaigns/${result.data.id}`);
      } else {
        if (!initialData?.id) {
          setServerError("Campaign ID missing for edit operation.");
          setIsSubmitting(false);
          return;
        }

        const result = await updateCampaignAction(initialData.id, {
          title: title.trim(),
          categoryId,
          targetAmount: parsedTarget,
          startAt,
          endAt,
          description: description.trim(),
        });

        if (!result.ok) {
          setServerError(
            mapBackendErrorCode(result.error.code, result.error.message)
          );
          setIsSubmitting(false);
          return;
        }

        router.push(`/campaigns/${result.data.id}`);
      }
    } catch {
      setServerError("An unexpected network error occurred. Please try again.");
      setIsSubmitting(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-6" noValidate>
      {/* Informational banner when editing a REJECTED campaign */}
      {isRejected && (
        <Alert variant="warning">
          <AlertTitle>Editing Rejected Campaign</AlertTitle>
          <AlertDescription>
            This campaign is currently in REJECTED status. Saving changes will update the campaign content, but the status will remain REJECTED. Resubmission is not supported in this API version.
          </AlertDescription>
        </Alert>
      )}

      {/* Read-only metadata in edit mode */}
      {mode === "edit" && initialData && (
        <div className="rounded-lg border border-border-subtle bg-surface-muted p-4 space-y-2 text-xs text-text-secondary">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span>
              Status: <CampaignStatusBadge status={initialData.status} size="sm" />
            </span>
            <span>Current Amount: {formatIDR(initialData.currentAmount)}</span>
          </div>
          <p className="text-text-muted">
            Campaign ID: <span className="font-mono">{initialData.id}</span>
          </p>
        </div>
      )}

      {/* Server Error Display */}
      {serverError && (
        <Alert variant="destructive">
          <AlertTitle>Unable to save campaign</AlertTitle>
          <AlertDescription>{serverError}</AlertDescription>
        </Alert>
      )}

      {/* Title Field */}
      <FormField id="campaign-title" error={errors.title} required>
        <FormLabel>Campaign Title</FormLabel>
        <Input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="e.g. Emergency Medical Support for Pediatric Care"
          disabled={isSubmitting}
          maxLength={200}
        />
        <FormHelperText>
          5–200 characters. Choose a clear and specific title.
        </FormHelperText>
        <FormErrorMessage>{errors.title}</FormErrorMessage>
      </FormField>

      {/* Category Field */}
      <FormField id="campaign-category" error={errors.categoryId} required>
        <FormLabel>Category</FormLabel>
        <Select
          value={categoryId}
          onChange={(e) => setCategoryId(e.target.value)}
          disabled={isSubmitting}
        >
          <option value="" disabled>
            Select a category...
          </option>
          {categories.map((cat) => (
            <option key={cat.id} value={cat.id}>
              {cat.name}
            </option>
          ))}
        </Select>
        <FormHelperText>
          Select the verified category that best classifies your initiative.
        </FormHelperText>
        <FormErrorMessage>{errors.categoryId}</FormErrorMessage>
      </FormField>

      {/* Target Amount Field */}
      <FormField id="campaign-target" error={errors.targetAmount} required>
        <FormLabel>Target Amount (IDR)</FormLabel>
        <Input
          type="text"
          inputMode="numeric"
          pattern="[0-9]*"
          value={targetAmountRaw}
          onChange={(e) => setTargetAmountRaw(e.target.value)}
          placeholder="e.g. 50000000"
          disabled={isSubmitting}
        />
        <FormHelperText>
          {targetAmountPreview ? (
            <span className="font-medium text-text-primary">
              Preview: {targetAmountPreview}
            </span>
          ) : (
            "Enter whole IDR without dots, commas, or currency symbols. Minimum Rp 10.000."
          )}
        </FormHelperText>
        <FormErrorMessage>{errors.targetAmount}</FormErrorMessage>
      </FormField>

      {/* Schedule: Start Date & End Date */}
      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
        <FormField id="campaign-start" error={errors.startDate} required>
          <FormLabel>Start Date</FormLabel>
          <Input
            type="date"
            value={startDate}
            onChange={(e) => setStartDate(e.target.value)}
            disabled={isSubmitting}
          />
          <FormHelperText>
            {mode === "create"
              ? "Date when the campaign starts (today or future)."
              : "Campaign scheduled start date."}
          </FormHelperText>
          <FormErrorMessage>{errors.startDate}</FormErrorMessage>
        </FormField>

        <FormField id="campaign-end" error={errors.endDate} required>
          <FormLabel>End Date</FormLabel>
          <Input
            type="date"
            value={endDate}
            onChange={(e) => setEndDate(e.target.value)}
            disabled={isSubmitting}
          />
          <FormHelperText>
            Date when the campaign ends (must be after start date).
          </FormHelperText>
          <FormErrorMessage>{errors.endDate}</FormErrorMessage>
        </FormField>
      </div>

      {/* Description Field */}
      <FormField id="campaign-description" error={errors.description} required>
        <FormLabel>Description &amp; Story</FormLabel>
        <Textarea
          rows={7}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="Describe why you are raising funds, the urgency, and how contributions will be utilized..."
          disabled={isSubmitting}
        />
        <FormHelperText>
          20–10,000 characters ({description.trim().length} characters entered). Provide complete context and accountability.
        </FormHelperText>
        <FormErrorMessage>{errors.description}</FormErrorMessage>
      </FormField>

      {/* Submit Controls */}
      <div className="flex flex-wrap items-center justify-end gap-3 pt-4 border-t border-border-subtle">
        <Button
          type="button"
          variant="outline"
          size="md"
          disabled={isSubmitting}
          onClick={() => router.back()}
        >
          Cancel
        </Button>
        <Button
          type="submit"
          variant="primary"
          size="md"
          isLoading={isSubmitting}
        >
          {mode === "create" ? "Create Campaign" : "Save Changes"}
        </Button>
      </div>
    </form>
  );
}
