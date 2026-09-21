/**
 * CareFund Donation Form & Amount Validation Utilities
 *
 * Implements strict non-mutating validation for donation inputs.
 *
 * ARCHITECTURAL CONTRACTS:
 * 1. Non-Mutating Validation: Never strips malformed characters silently
 *    (prevents numeric inflation bugs, e.g. "100.5" -> 1005).
 * 2. Strict Integer Format: Enforces positive integer string syntax (/^[1-9]\d*$/).
 *    Leading zero rejection is a frontend UX policy to prevent donor confusion.
 * 3. Safe Integer Boundary: Enforces Number.isSafeInteger() to guarantee safe IDR values.
 * 4. Product Recommendation: Recommends minimum IDR 10,000 for standard UX guidance.
 *    Backend authoritative domain rule remains amount > 0.
 * 5. Currency Handling: Strict whole Indonesian Rupiah (IDR); no fractional/decimal values.
 */

export const RECOMMENDED_MINIMUM_DONATION_IDR = 10_000;

export interface AmountValidationResult {
  isValid: boolean;
  amount: number;
  error?: string;
  isBelowRecommended?: boolean;
}

/**
 * Validates raw string input for donation amounts.
 *
 * @param rawInput Unsanitized string from the user input field
 * @param enforceRecommendedMinimum Whether to enforce IDR 10,000 recommendation as an error (default: true)
 */
export function validateDonationAmountInput(
  rawInput: string,
  enforceRecommendedMinimum = true
): AmountValidationResult {
  if (typeof rawInput !== "string") {
    return {
      isValid: false,
      amount: 0,
      error: "Please enter a donation amount.",
    };
  }

  const trimmed = rawInput.trim();

  if (!trimmed) {
    return {
      isValid: false,
      amount: 0,
      error: "Please enter a donation amount.",
    };
  }

  // 1. Check for negative signs or positive signs
  if (/^[-+]/.test(trimmed)) {
    return {
      isValid: false,
      amount: 0,
      error: "Amount must be a positive whole number.",
    };
  }

  // 2. Check for decimals (dots or commas)
  if (/[.,]/.test(trimmed)) {
    return {
      isValid: false,
      amount: 0,
      error: "Decimals and fractional amounts are not supported. Please enter whole IDR.",
    };
  }

  // 3. Check for letters or other non-digit characters
  if (/\D/.test(trimmed)) {
    return {
      isValid: false,
      amount: 0,
      error: "Amount must contain numbers only without symbols, spaces, or letters.",
    };
  }

  // 4. Check for leading zero or zero
  if (trimmed === "0" || /^0+$/.test(trimmed)) {
    return {
      isValid: false,
      amount: 0,
      error: "Donation amount must be greater than zero.",
    };
  }

  if (/^0\d+/.test(trimmed)) {
    return {
      isValid: false,
      amount: 0,
      error: "Amount must not contain leading zeros.",
    };
  }

  // 5. Enforce integer regex strictly (/^[1-9]\d*$/)
  if (!/^[1-9]\d*$/.test(trimmed)) {
    return {
      isValid: false,
      amount: 0,
      error: "Please enter a valid whole number in IDR.",
    };
  }

  // 6. Check JavaScript safe integer limit
  const parsed = Number(trimmed);
  if (!Number.isSafeInteger(parsed) || parsed <= 0) {
    return {
      isValid: false,
      amount: 0,
      error: "The entered amount exceeds the maximum allowable donation value.",
    };
  }

  // 7. Product recommendation check (IDR 10,000 UX guidance)
  if (parsed < RECOMMENDED_MINIMUM_DONATION_IDR) {
    if (enforceRecommendedMinimum) {
      return {
        isValid: false,
        amount: parsed,
        isBelowRecommended: true,
        error: "Minimum recommended donation is Rp 10.000.",
      };
    }
    return {
      isValid: true,
      amount: parsed,
      isBelowRecommended: true,
    };
  }

  return {
    isValid: true,
    amount: parsed,
  };
}

/**
 * Formats a valid whole IDR amount into localized Indonesian Rupiah string (e.g. "Rp 50.000").
 */
export function formatIDR(amount: number): string {
  if (!Number.isSafeInteger(amount) || amount < 0) {
    return "Rp 0";
  }
  return `Rp ${amount.toLocaleString("id-ID")}`;
}
