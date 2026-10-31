# 0008 — Customer communications

**Status:** accepted 2026-10-10

## Decision
- Edit Shopify's notification templates (confirmation, cancellation, ready for pickup, local delivery) to print the bake day and window; the defaults do not reliably show line-item properties.
- Bakehouse sends a day-before reminder by email (Resend) and, with a separate opt-in collected in the cart, by SMS (Twilio).
- "Ordering is open" broadcasts go to customers with email-marketing consent (Resend broadcasts), syncing unsubscribes back to Shopify.

## Consequences
- Twilio requires A2P 10DLC registration before launch (days to weeks).
- CAN-SPAM requires a physical address in broadcasts; use a PO box, not her home.
