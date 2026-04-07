import { View, Text, StyleSheet } from "react-native";
import { Marker } from "react-native-maps";
import { RankedListing } from "../../lib/hooks/useDiscovery";

interface ListingMarkerProps {
  listing: RankedListing;
  onPress: (listing: RankedListing) => void;
  selected?: boolean;
}

function priceLabel(listing: RankedListing): string {
  if (listing.pricePerDay != null) return `$${listing.pricePerDay}/day`;
  if (listing.pricePerHour != null) return `$${listing.pricePerHour}/hr`;
  return "—";
}

export default function ListingMarker({
  listing,
  onPress,
  selected = false,
}: ListingMarkerProps) {
  return (
    <Marker
      coordinate={{ latitude: listing.lat, longitude: listing.lng }}
      onPress={() => onPress(listing)}
      tracksViewChanges={false}
    >
      <View style={[styles.pill, selected && styles.pillSelected]}>
        <Text style={[styles.label, selected && styles.labelSelected]}>
          {priceLabel(listing)}
        </Text>
      </View>
    </Marker>
  );
}

const styles = StyleSheet.create({
  pill: {
    paddingHorizontal: 8,
    paddingVertical: 4,
    borderRadius: 12,
    backgroundColor: "#ffffff",
    borderWidth: 1,
    borderColor: "#e5e7eb",
    shadowColor: "#000",
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.12,
    shadowRadius: 2,
    elevation: 2,
  },
  pillSelected: {
    backgroundColor: "#0284c7",
    borderColor: "#0369a1",
  },
  label: {
    fontSize: 11,
    fontWeight: "600",
    color: "#111827",
  },
  labelSelected: {
    color: "#ffffff",
  },
});
