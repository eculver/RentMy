import { View, Text } from "react-native";
import { Callout } from "react-native-maps";
import { RankedListing } from "../../lib/hooks/useDiscovery";

interface MapCalloutProps {
  listing: RankedListing;
}

// MapCallout renders the inline tooltip that appears on the map when a marker
// is tapped. Used when an overlay preview card is not desired.
export default function MapCallout({ listing }: MapCalloutProps) {
  return (
    <Callout tooltip={false}>
      <View className="p-1">
        <Text className="text-sm font-semibold text-gray-900 max-w-48" numberOfLines={2}>
          {listing.title}
        </Text>
      </View>
    </Callout>
  );
}
