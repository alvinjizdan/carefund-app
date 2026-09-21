/**
 * CareFund Web - Midtrans Snap Ambient TypeScript Declarations
 *
 * Exposes the minimal typed interface for the window.snap object
 * injected by Midtrans Snap JS.
 *
 * INVARIANTS:
 * - Reflects only the methods actually invoked by the application (pay).
 * - Avoids broad `any` escape hatches where structured callbacks are used.
 * - Callbacks are untrusted UX events; financial state is never mutated directly.
 */

export interface SnapPayCallbacks {
  /**
   * Invoked when the user completes payment in the Snap modal.
   * UX trigger only; authoritative read must verify backend status.
   */
  onSuccess?: (result: Record<string, unknown>) => void;

  /**
   * Invoked when an asynchronous payment method (e.g. Bank Transfer / VA) is selected.
   * UX trigger only; instructs the user on completing the transfer.
   */
  onPending?: (result: Record<string, unknown>) => void;

  /**
   * Invoked when the payment gateway encounters an error.
   */
  onError?: (result: Record<string, unknown>) => void;

  /**
   * Invoked when the donor dismisses or closes the Snap modal.
   */
  onClose?: () => void;
}

export interface MidtransSnap {
  pay: (token: string, callbacks?: SnapPayCallbacks) => void;
}

declare global {
  interface Window {
    snap?: MidtransSnap;
  }
}
