#!/bin/bash
# Patches @stripe/stripe-react-native for Xcode 26+ compatibility.
# The forward declaration of STPPaymentStatus uses NSUInteger but the
# actual Swift enum maps to NSInteger. Xcode 26+ treats this as a hard error.
# Fixed upstream in stripe-react-native 0.61.0, but we're pinned to 0.59.2
# due to StripeCore version conflicts with stripe-identity-react-native.

FILE="node_modules/@stripe/stripe-react-native/ios/StripeSwiftInterop.h"
if [ -f "$FILE" ]; then
  sed -i '' 's/NS_ENUM(NSUInteger, STPPaymentStatus)/NS_ENUM(NSInteger, STPPaymentStatus)/' "$FILE"
fi
