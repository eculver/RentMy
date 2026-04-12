import { RankedListing } from "../../lib/hooks/useDiscovery";

interface ListingMarkerProps {
  listing: RankedListing;
  onPress: (listing: RankedListing) => void;
  selected?: boolean;
}

// react-native-maps is not supported on web. This stub prevents Metro from
// trying to bundle native-only imports when building for the web platform.
export default function ListingMarker(_props: ListingMarkerProps) {
  return null;
}
