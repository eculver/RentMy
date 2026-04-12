// @stripe/stripe-react-native is not supported on web. This wrapper is a
// pass-through so the root layout can render without importing native modules.
export default function StripeProviderWrapper({
  children,
}: {
  children: React.ReactNode;
}) {
  return <>{children}</>;
}
