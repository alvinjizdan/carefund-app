"use client";

/**
 * CareFund Donation Checkout & Payment UX Modal
 *
 * Implements the client-side donation checkout dialog using React 18.3.1 and Next.js 15.
 *
 * ARCHITECTURAL CONTRACTS:
 * 1. Authority Delegation: Go API is the SOLE authority for payment status and financial integrity.
 * 2. Snap Callbacks: Treated strictly as UX events; never directly infer financial settlement.
 * 3. Token Security: Payment token is held in component memory only; never stored in localStorage,
 *    sessionStorage, cookies, or URL query parameters. Discarded upon completion.
 * 4. Polling Safety Budget: Client-driven interval of 3,000ms, max 10 requests, 30s wall-clock limit.
 *    Stops immediately upon reaching any terminal status.
 * 5. Strict Non-Mutating Validation: Prevents silent input stripping or floating-point precision loss.
 * 6. Accessibility: Full keyboard navigation, WAI-ARIA labels, assertive alert announcements,
 *    and focus management adhering to WCAG 2.1 AA.
 */

import * as React from "react";
import Script from "next/script";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/forms/input";
import { Label } from "@/components/forms/label";
import {
  FormField,
  FormLabel,
  FormHelperText,
  FormErrorMessage,
} from "@/components/forms/form-field";
import { Textarea } from "@/components/forms/textarea";
import { Alert, AlertTitle, AlertDescription } from "@/components/feedback/alert";
import { Spinner } from "@/components/ui/spinner";
import { Badge } from "@/components/ui/badge";
import { createDonationAction, getPaymentStatusAction } from "@/lib/actions/donations";
import {
  validateDonationAmountInput,
  formatIDR,
  RECOMMENDED_MINIMUM_DONATION_IDR,
} from "@/lib/donations/validation";
import type { PaymentDetail, PaymentStatus } from "@/lib/api/authenticated/donations";

export type CheckoutState =
  | "IDLE"
  | "SUBMITTING"
  | "IN_FLIGHT_WAIT"
  | "SNAP_OPEN"
  | "VERIFYING_PAYMENT"
  | "SUCCESS"
  | "PENDING_INSTRUCTIONS"
  | "FAILURE"
  | "REFUNDED"
  | "TIMEOUT";

export interface DonationCheckoutModalProps {
  /** Trusted campaign ID from server-rendered context */
  campaignId: string;
  /** Optional campaign title for dialog presentation */
  campaignTitle?: string;
  /** Controlled dialog visibility */
  isOpen: boolean;
  /** Dialog visibility change handler */
  onOpenChange: (open: boolean) => void;
  /** Optional callback invoked upon confirmed payment settlement */
  onDonationSuccess?: (payment: PaymentDetail) => void;
}

const PRESET_AMOUNTS = [25_000, 50_000, 100_000, 250_000, 500_000];
const POLL_INTERVAL_MS = 3_000;
const MAX_POLL_REQUESTS = 10;
const MAX_WALL_CLOCK_DURATION_MS = 30_000;

export function DonationCheckoutModal({
  campaignId,
  campaignTitle,
  isOpen,
  onOpenChange,
  onDonationSuccess,
}: DonationCheckoutModalProps) {
  // Form input states
  const [rawAmount, setRawAmount] = React.useState<string>("50000");
  const [isAnonymous, setIsAnonymous] = React.useState<boolean>(false);
  const [message, setMessage] = React.useState<string>("");
  const [amountError, setAmountError] = React.useState<string | undefined>();

  // Checkout workflow state machine
  const [state, setState] = React.useState<CheckoutState>("IDLE");
  const [activePaymentId, setActivePaymentId] = React.useState<string | null>(null);
  const [activeOrderId, setActiveOrderId] = React.useState<string | null>(null);
  const [confirmedPayment, setConfirmedPayment] = React.useState<PaymentDetail | null>(null);
  const [errorMessage, setErrorMessage] = React.useState<string | null>(null);
  const [errorCode, setErrorCode] = React.useState<string | null>(null);
  const [snapReady, setSnapReady] = React.useState<boolean>(false);

  // In-memory ephemeral payment token (NEVER persisted to browser storage)
  const paymentTokenRef = React.useRef<string | null>(null);
  const pollingTimerRef = React.useRef<NodeJS.Timeout | null>(null);
  const pollingStartRef = React.useRef<number>(0);
  const pollCountRef = React.useRef<number>(0);

  // Reset checkout state when modal opens or closes
  React.useEffect(() => {
    if (!isOpen) {
      // Clear timers and release ephemeral token on close
      if (pollingTimerRef.current) {
        clearTimeout(pollingTimerRef.current);
        pollingTimerRef.current = null;
      }
      paymentTokenRef.current = null;
      // Reset to initial state after modal unmounts/closes
      setState("IDLE");
      setActivePaymentId(null);
      setActiveOrderId(null);
      setConfirmedPayment(null);
      setErrorMessage(null);
      setErrorCode(null);
      setAmountError(undefined);
    }
  }, [isOpen]);

  // Check if Snap is already available in window
  React.useEffect(() => {
    if (typeof window !== "undefined" && window.snap && typeof window.snap.pay === "function") {
      setSnapReady(true);
    }
  }, []);

  // Safe user-facing error message mapper adhering to Go backend codes
  const mapBackendErrorToMessage = (code?: string, defaultMsg?: string): string => {
    switch (code) {
      case "INVALID_REQUEST":
        return "The donation request was invalid. Please check your entered details and try again.";
      case "UNAUTHORIZED":
        return "Your session has expired. Please sign in again to complete your donation.";
      case "FORBIDDEN":
        return "You are not authorized to perform this donation. Please contact support.";
      case "NOT_FOUND":
        return "This campaign is currently not accepting donations.";
      case "INVALID_STATE_TRANSITION":
        return "The donation cannot be processed in its current state. Please refresh and try again.";
      case "payment_failed":
        return "Your payment could not be completed. Please try another payment method.";
      case "TOO_MANY_REQUESTS":
        return "You have made too many donation attempts. Please wait 60 seconds before trying again.";
      case "request_in_flight":
        return "Your donation request is currently processing. Please wait a moment.";
      case "TIMEOUT":
        return "The payment request timed out. Please check your donation history before trying again.";
      default:
        return defaultMsg || "An unexpected error occurred. Please try again.";
    }
  };

  // Launch Midtrans Snap Popup
  const launchSnapModal = React.useCallback(
    (token: string, paymentId: string) => {
      if (typeof window === "undefined" || !window.snap || typeof window.snap.pay !== "function") {
        setState("FAILURE");
        setErrorMessage("Payment gateway is temporarily unavailable. Please refresh and try again.");
        return;
      }

      setState("SNAP_OPEN");

      window.snap.pay(token, {
        onSuccess: () => {
          // UX signal only: Transition to verification and initiate authoritative polling
          setState("VERIFYING_PAYMENT");
        },
        onPending: () => {
          // Asynchronous payment method selected (e.g. Bank Transfer / VA / QRIS)
          setState("PENDING_INSTRUCTIONS");
        },
        onError: () => {
          setState("FAILURE");
          setErrorCode("payment_failed");
          setErrorMessage("Your payment could not be completed. Please try another payment method.");
        },
        onClose: async () => {
          // Check latest authoritative status on modal dismissal
          if (paymentId) {
            setState("VERIFYING_PAYMENT");
          }
        },
      });
    },
    []
  );

  // Polling execution loop: Client-driven interval of 3,000ms, max 10 requests, 30s wall-clock
  React.useEffect(() => {
    if ((state !== "VERIFYING_PAYMENT" && state !== "PENDING_INSTRUCTIONS") || !activePaymentId) {
      if (pollingTimerRef.current) {
        clearTimeout(pollingTimerRef.current);
        pollingTimerRef.current = null;
      }
      return;
    }

    let isSubscribed = true;
    pollingStartRef.current = Date.now();
    pollCountRef.current = 0;

    const executePoll = async () => {
      if (!isSubscribed) return;

      const elapsed = Date.now() - pollingStartRef.current;
      pollCountRef.current += 1;

      // Wall-clock (30s) or request count (10) budget safety check
      if (elapsed >= MAX_WALL_CLOCK_DURATION_MS || pollCountRef.current > MAX_POLL_REQUESTS) {
        if (state === "VERIFYING_PAYMENT") {
          setState("TIMEOUT");
        }
        return;
      }

      try {
        const result = await getPaymentStatusAction(activePaymentId);

        if (!isSubscribed) return;

        if (result.ok) {
          const payment = result.data;
          setConfirmedPayment(payment);

          // Terminal Status Checks
          if (payment.status === "CAPTURED" || payment.status === "SETTLED") {
            setState("SUCCESS");
            onDonationSuccess?.(payment);
            paymentTokenRef.current = null;
            return;
          }

          if (
            payment.status === "FAILED" ||
            payment.status === "EXPIRED" ||
            payment.status === "CANCELLED"
          ) {
            setState("FAILURE");
            setErrorCode(payment.status);
            setErrorMessage(
              payment.status === "EXPIRED"
                ? "The payment request has expired. Please try again."
                : "Your payment was not completed. Please try another payment method."
            );
            paymentTokenRef.current = null;
            return;
          }

          if (payment.status === "REFUNDED" || payment.status === "PARTIALLY_REFUNDED") {
            setState("REFUNDED");
            paymentTokenRef.current = null;
            return;
          }

          // Non-terminal: PENDING or AUTHORIZED -> schedule next tick
          if (payment.status === "PENDING" || payment.status === "AUTHORIZED") {
            pollingTimerRef.current = setTimeout(executePoll, POLL_INTERVAL_MS);
          }
        } else {
          // If server action fails with terminal client error, stop polling
          if (result.error.status === 401 || result.error.status === 403 || result.error.status === 404) {
            setState("FAILURE");
            setErrorCode(result.error.code);
            setErrorMessage(mapBackendErrorToMessage(result.error.code, result.error.message));
            return;
          }

          // Transient error: retry if budget remains
          pollingTimerRef.current = setTimeout(executePoll, POLL_INTERVAL_MS);
        }
      } catch {
        if (!isSubscribed) return;
        pollingTimerRef.current = setTimeout(executePoll, POLL_INTERVAL_MS);
      }
    };

    // Execute first poll tick after interval
    pollingTimerRef.current = setTimeout(executePoll, POLL_INTERVAL_MS);

    return () => {
      isSubscribed = false;
      if (pollingTimerRef.current) {
        clearTimeout(pollingTimerRef.current);
        pollingTimerRef.current = null;
      }
    };
  }, [state, activePaymentId, onDonationSuccess]);

  // Handle Form Submission
  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();

    // 1. Strict Amount Validation
    const validation = validateDonationAmountInput(rawAmount, true);
    if (!validation.isValid) {
      setAmountError(validation.error);
      return;
    }
    setAmountError(undefined);

    // 2. Transition to SUBMITTING immediately to prevent double submissions
    setState("SUBMITTING");
    setErrorMessage(null);
    setErrorCode(null);

    try {
      const result = await createDonationAction({
        campaignId,
        amount: validation.amount,
        isAnonymous,
        message: message.trim() ? message.trim() : undefined,
      });

      if (!result.ok) {
        setState("FAILURE");
        setErrorCode(result.error.code);
        setErrorMessage(mapBackendErrorToMessage(result.error.code, result.error.message));
        return;
      }

      // Successful 201 response
      const creation = result.data;
      paymentTokenRef.current = creation.paymentToken;
      setActivePaymentId(creation.paymentId);
      setActiveOrderId(creation.orderId);

      // Open Snap modal with ephemeral token if present
      if (creation.paymentToken) {
        launchSnapModal(creation.paymentToken, creation.paymentId);
      } else {
        setState("FAILURE");
        setErrorMessage("Payment gateway token was not returned. Please check your donation history.");
      }
    } catch {
      setState("FAILURE");
      setErrorMessage("Unable to process donation. Please check your network connection and try again.");
    }
  };

  const handleSelectPreset = (amount: number) => {
    setRawAmount(String(amount));
    setAmountError(undefined);
  };

  const isSubmitting = state === "SUBMITTING" || state === "IN_FLIGHT_WAIT";
  const isVerifying = state === "VERIFYING_PAYMENT";

  const snapScriptUrl =
    process.env.NEXT_PUBLIC_MIDTRANS_IS_PRODUCTION === "true"
      ? "https://app.midtrans.com/snap/snap.js"
      : "https://app.sandbox.midtrans.com/snap/snap.js";

  const clientKey = process.env.NEXT_PUBLIC_MIDTRANS_CLIENT_KEY || "";

  return (
    <>
      {/* Midtrans Snap JS External Script */}
      <Script
        src={snapScriptUrl}
        data-client-key={clientKey}
        strategy="lazyOnload"
        onLoad={() => setSnapReady(true)}
      />

      <Dialog open={isOpen} onOpenChange={onOpenChange}>
        <DialogContent
          className="sm:max-w-md"
          aria-describedby="checkout-dialog-description"
        >
          <DialogHeader>
            <DialogTitle className="text-xl font-bold">
              {state === "SUCCESS"
                ? "Donation Successful!"
                : state === "PENDING_INSTRUCTIONS"
                ? "Payment Pending"
                : state === "TIMEOUT"
                ? "Payment Processing"
                : state === "FAILURE"
                ? "Donation Unsuccessful"
                : "Make a Donation"}
            </DialogTitle>
            <DialogDescription id="checkout-dialog-description">
              {campaignTitle
                ? `Supporting: ${campaignTitle}`
                : "Your contribution directly empowers patient healthcare."}
            </DialogDescription>
          </DialogHeader>

          {/* STATE: SUCCESS */}
          {state === "SUCCESS" && (
            <div className="flex flex-col items-center gap-4 py-6 text-center">
              <div className="flex h-16 w-16 items-center justify-center rounded-full bg-success-surface text-success">
                <svg
                  className="h-8 w-8"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth="2.5"
                  aria-hidden="true"
                >
                  <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                </svg>
              </div>

              <div className="space-y-1">
                <h3 className="text-lg font-semibold text-text-primary">
                  Thank you for your generosity!
                </h3>
                <p className="text-sm text-text-secondary">
                  Your payment has been officially settled and verified.
                </p>
              </div>

              {confirmedPayment && (
                <div className="w-full rounded-lg border border-border-subtle bg-surface-muted p-3 text-left text-xs space-y-1">
                  <div className="flex justify-between text-text-secondary">
                    <span>Order ID:</span>
                    <span className="font-mono text-text-primary">{confirmedPayment.orderId}</span>
                  </div>
                  <div className="flex justify-between text-text-secondary">
                    <span>Amount:</span>
                    <span className="font-semibold text-text-primary">
                      {formatIDR(confirmedPayment.grossAmount)}
                    </span>
                  </div>
                  <div className="flex justify-between text-text-secondary">
                    <span>Status:</span>
                    <Badge variant="success" size="sm">
                      {confirmedPayment.status}
                    </Badge>
                  </div>
                </div>
              )}

              <DialogFooter className="w-full sm:justify-center pt-2">
                <Button variant="primary" className="w-full" onClick={() => onOpenChange(false)}>
                  Close
                </Button>
              </DialogFooter>
            </div>
          )}

          {/* STATE: PENDING_INSTRUCTIONS */}
          {state === "PENDING_INSTRUCTIONS" && (
            <div className="flex flex-col gap-4 py-4">
              <Alert variant="warning">
                <AlertTitle>Awaiting Payment Transfer</AlertTitle>
                <AlertDescription>
                  Please follow the payment instructions provided in the Midtrans window (such as
                  Virtual Account or QRIS transfer). Your donation will automatically update once
                  received.
                </AlertDescription>
              </Alert>

              {activeOrderId && (
                <p className="text-xs text-text-secondary text-center">
                  Reference Order ID: <span className="font-mono font-medium">{activeOrderId}</span>
                </p>
              )}

              <div className="flex items-center justify-center gap-2 py-2 text-xs text-text-secondary">
                <Spinner size="sm" aria-hidden="true" />
                <span>Checking payment status automatically...</span>
              </div>

              <DialogFooter className="sm:justify-between gap-2 pt-2">
                {paymentTokenRef.current && (
                  <Button
                    variant="outline"
                    onClick={() => launchSnapModal(paymentTokenRef.current!, activePaymentId!)}
                  >
                    Reopen Payment Window
                  </Button>
                )}
                <Button variant="primary" onClick={() => onOpenChange(false)}>
                  View Donation History
                </Button>
              </DialogFooter>
            </div>
          )}

          {/* STATE: VERIFYING_PAYMENT */}
          {state === "VERIFYING_PAYMENT" && (
            <div className="flex flex-col items-center gap-4 py-8 text-center" aria-live="polite">
              <Spinner size="lg" aria-label="Verifying payment status" />
              <div className="space-y-1">
                <h3 className="text-base font-semibold text-text-primary">
                  Verifying payment with CareFund...
                </h3>
                <p className="text-xs text-text-secondary">
                  Confirming settlement with the payment provider. Please do not close this window.
                </p>
              </div>
            </div>
          )}

          {/* STATE: TIMEOUT */}
          {state === "TIMEOUT" && (
            <div className="flex flex-col gap-4 py-4 text-center">
              <Alert variant="info">
                <AlertTitle>Payment Processing</AlertTitle>
                <AlertDescription>
                  Your payment is currently being verified by the provider. Please check your Donation
                  History in a few moments for the updated status.
                </AlertDescription>
              </Alert>

              {activeOrderId && (
                <p className="text-xs text-text-secondary">
                  Order ID: <span className="font-mono font-medium">{activeOrderId}</span>
                </p>
              )}

              <DialogFooter className="w-full sm:justify-center pt-2">
                <Button variant="primary" className="w-full" onClick={() => onOpenChange(false)}>
                  Go to Donation History
                </Button>
              </DialogFooter>
            </div>
          )}

          {/* STATE: REFUNDED */}
          {state === "REFUNDED" && (
            <div className="flex flex-col gap-4 py-4 text-center">
              <Alert variant="warning">
                <AlertTitle>Payment Refunded</AlertTitle>
                <AlertDescription>
                  This transaction has been refunded by the payment processor.
                </AlertDescription>
              </Alert>
              <DialogFooter className="w-full sm:justify-center pt-2">
                <Button variant="secondary" className="w-full" onClick={() => onOpenChange(false)}>
                  Close
                </Button>
              </DialogFooter>
            </div>
          )}

          {/* STATE: FAILURE */}
          {state === "FAILURE" && (
            <div className="flex flex-col gap-4 py-4">
              <Alert variant="destructive">
                <AlertTitle>Transaction Failed</AlertTitle>
                <AlertDescription>
                  {errorMessage || "Your payment could not be processed. Please try again."}
                </AlertDescription>
              </Alert>

              <DialogFooter className="sm:justify-end gap-2 pt-2">
                <Button variant="secondary" onClick={() => onOpenChange(false)}>
                  Cancel
                </Button>
                <Button
                  variant="primary"
                  onClick={() => {
                    setState("IDLE");
                    setErrorMessage(null);
                    setErrorCode(null);
                  }}
                >
                  Try Again
                </Button>
              </DialogFooter>
            </div>
          )}

          {/* STATE: IDLE / SUBMITTING / SNAP_OPEN (Form View) */}
          {(state === "IDLE" || state === "SUBMITTING" || state === "SNAP_OPEN") && (
            <form onSubmit={handleSubmit} className="flex flex-col gap-4 pt-2" noValidate>
              {/* Preset Amount Selectors */}
              <div className="flex flex-col gap-1.5">
                <Label className="text-xs text-text-secondary">Quick Amount Select</Label>
                <div className="grid grid-cols-3 gap-2">
                  {PRESET_AMOUNTS.map((preset) => (
                    <Button
                      key={preset}
                      type="button"
                      variant={rawAmount === String(preset) ? "primary" : "outline"}
                      size="sm"
                      disabled={isSubmitting}
                      onClick={() => handleSelectPreset(preset)}
                      className="text-xs"
                    >
                      {formatIDR(preset)}
                    </Button>
                  ))}
                </div>
              </div>

              {/* Amount Input */}
              <FormField id="donation-amount" error={amountError} required>
                <FormLabel>Donation Amount (IDR)</FormLabel>
                <div className="relative">
                  <span className="absolute left-3 top-2.5 text-sm font-semibold text-text-secondary pointer-events-none">
                    Rp
                  </span>
                  <Input
                    id="donation-amount"
                    name="amount"
                    type="text"
                    inputMode="numeric"
                    placeholder="50000"
                    value={rawAmount}
                    onChange={(e) => {
                      setRawAmount(e.target.value);
                      if (amountError) setAmountError(undefined);
                    }}
                    disabled={isSubmitting}
                    className="pl-10"
                    aria-invalid={Boolean(amountError)}
                    aria-describedby={amountError ? "donation-amount-error" : "donation-amount-helper"}
                    required
                  />
                </div>
                <FormHelperText id="donation-amount-helper">
                  Recommended minimum: {formatIDR(RECOMMENDED_MINIMUM_DONATION_IDR)}
                </FormHelperText>
                <FormErrorMessage id="donation-amount-error" message={amountError} />
              </FormField>

              {/* Anonymous Checkbox */}
              <div className="flex items-start gap-2.5 pt-1">
                <input
                  id="donation-anonymous"
                  name="isAnonymous"
                  type="checkbox"
                  checked={isAnonymous}
                  onChange={(e) => setIsAnonymous(e.target.checked)}
                  disabled={isSubmitting}
                  className="mt-1 h-4 w-4 rounded border-border-strong text-primary focus:ring-focus"
                />
                <div className="flex flex-col">
                  <label
                    htmlFor="donation-anonymous"
                    className="text-sm font-medium text-text-primary cursor-pointer"
                  >
                    Donate anonymously
                  </label>
                  <span className="text-xs text-text-secondary">
                    Your name will be hidden from the public campaign donor list.
                  </span>
                </div>
              </div>

              {/* Message Textarea */}
              <FormField id="donation-message">
                <FormLabel>Support Message (Optional)</FormLabel>
                <Textarea
                  id="donation-message"
                  name="message"
                  rows={2}
                  maxLength={500}
                  placeholder="Write a message of hope or encouragement..."
                  value={message}
                  onChange={(e) => setMessage(e.target.value)}
                  disabled={isSubmitting}
                  className="resize-none"
                />
              </FormField>

              {state === "SNAP_OPEN" && (
                <Alert variant="info" className="mt-1">
                  <AlertTitle>Payment Gateway Active</AlertTitle>
                  <AlertDescription>
                    Complete your payment in the Midtrans window. If the window did not open, click
                    below.
                  </AlertDescription>
                </Alert>
              )}

              <DialogFooter className="pt-2 sm:justify-end gap-2">
                <Button
                  type="button"
                  variant="secondary"
                  disabled={isSubmitting}
                  onClick={() => onOpenChange(false)}
                >
                  Cancel
                </Button>
                <Button
                  type="submit"
                  variant="primary"
                  isLoading={isSubmitting}
                  disabled={isSubmitting}
                  aria-busy={isSubmitting}
                >
                  {isSubmitting ? "Processing..." : "Proceed to Payment"}
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
