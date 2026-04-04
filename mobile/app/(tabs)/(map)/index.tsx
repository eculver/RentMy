import { View, ActivityIndicator, Text } from "react-native";
import MapView, { PROVIDER_GOOGLE, Region } from "react-native-maps";
import { useState, useCallback } from "react";
import { useDebouncedCallback } from "use-debounce";
import { useLocation } from "../../../lib/hooks/useLocation";
import { useMapListings, RankedListing, MapBounds } from "../../../lib/hooks/useDiscovery";
import ListingMarker from "../../../components/map/ListingMarker";
import ListingPreviewCard from "../../../components/map/ListingPreviewCard";

// Initial zoom level covering roughly a 5km radius.
const INITIAL_DELTA = 0.05;

function regionToBounds(region: Region): MapBounds {
  return {
    swLat: region.latitude - region.latitudeDelta / 2,
    swLng: region.longitude - region.longitudeDelta / 2,
    neLat: region.latitude + region.latitudeDelta / 2,
    neLng: region.longitude + region.longitudeDelta / 2,
  };
}

export default function MapScreen() {
  const { lat, lng, loading: locationLoading, error: locationError } = useLocation();
  const [bounds, setBounds] = useState<MapBounds | null>(null);
  const [selectedListing, setSelectedListing] = useState<RankedListing | null>(null);

  const { data } = useMapListings(bounds);
  const listings: RankedListing[] = data?.listings ?? [];

  // Debounce region changes 500ms to avoid API spam while panning.
  const onRegionChangeComplete = useDebouncedCallback((region: Region) => {
    setBounds(regionToBounds(region));
  }, 500);

  const handleMarkerPress = useCallback((listing: RankedListing) => {
    setSelectedListing(listing);
  }, []);

  const handleDismiss = useCallback(() => {
    setSelectedListing(null);
  }, []);

  if (locationLoading) {
    return (
      <View className="flex-1 items-center justify-center bg-white">
        <ActivityIndicator size="large" color="#0284c7" />
        <Text className="text-gray-500 mt-3 text-sm">Getting your location…</Text>
      </View>
    );
  }

  if (locationError || lat === null || lng === null) {
    return (
      <View className="flex-1 items-center justify-center bg-white px-8">
        <Text className="text-lg font-semibold text-gray-800 text-center">
          Location unavailable
        </Text>
        <Text className="text-sm text-gray-500 text-center mt-2">
          {locationError ?? "Enable location access to browse the map."}
        </Text>
      </View>
    );
  }

  const initialRegion: Region = {
    latitude: lat,
    longitude: lng,
    latitudeDelta: INITIAL_DELTA,
    longitudeDelta: INITIAL_DELTA,
  };

  return (
    <View className="flex-1">
      <MapView
        provider={PROVIDER_GOOGLE}
        style={{ flex: 1 }}
        initialRegion={initialRegion}
        onRegionChangeComplete={onRegionChangeComplete}
        showsUserLocation
        showsMyLocationButton
        onPress={() => setSelectedListing(null)}
      >
        {listings.map((listing) => (
          <ListingMarker
            key={listing.id}
            listing={listing}
            onPress={handleMarkerPress}
            selected={selectedListing?.id === listing.id}
          />
        ))}
      </MapView>

      {selectedListing !== null && (
        <ListingPreviewCard listing={selectedListing} onDismiss={handleDismiss} />
      )}
    </View>
  );
}
