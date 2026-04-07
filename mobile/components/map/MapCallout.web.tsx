import { RankedListing } from "../../lib/hooks/useDiscovery";

interface MapCalloutProps {
  listing: RankedListing;
}

// react-native-maps is not supported on web. This stub prevents Metro from
// trying to bundle native-only imports when building for the web platform.
export default function MapCallout(_props: MapCalloutProps) {
  return null;
}
