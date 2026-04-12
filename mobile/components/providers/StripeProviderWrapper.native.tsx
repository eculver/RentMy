import { StripeProvider } from "@stripe/stripe-react-native";

const STRIPE_PUBLISHABLE_KEY =
  process.env.EXPO_PUBLIC_STRIPE_PUBLISHABLE_KEY ?? "pk_test_placeholder";

export default function StripeProviderWrapper({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <StripeProvider publishableKey={STRIPE_PUBLISHABLE_KEY}>
      {children as React.ReactElement}
    </StripeProvider>
  );
}
