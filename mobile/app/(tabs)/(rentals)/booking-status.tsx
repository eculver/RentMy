/**
 * Re-exports the booking-status screen so it can be pushed within the Rentals
 * tab's Stack navigator.  Expo Router requires each route to live inside the
 * tab group where it is navigated — cross-tab pushes cause ScrollView content
 * rendering issues.
 */
export { default } from "../(feed)/booking-status";
