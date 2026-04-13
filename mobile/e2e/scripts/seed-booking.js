// seed-booking.js — Creates a booking via the test-only backend endpoint.
//
// Requires the backend to be running with E2E_MODE=true.
//
// Env vars (set in the calling Maestro flow):
//   API_URL        — Backend base URL (default: http://localhost:8080)
//   RENTER_EMAIL   — Email of the renter account (default: bob@test.com)
//   STATUS         — Desired booking status: REQUESTED | ACCEPTED | ACTIVE | COMPLETED
//                    (default: REQUESTED)
//
// Output variables (readable as ${TRANSACTION_ID}, ${PIN} etc. in the flow):
//   TRANSACTION_ID — The created booking's transaction ID
//   PIN            — The check-in PIN (only set when STATUS=ACCEPTED, value: "1234")
//   LISTING_LAT    — Listing latitude (may be 0 if not set on the test listing)
//   LISTING_LNG    — Listing longitude (may be 0 if not set on the test listing)

var apiUrl = API_URL || 'http://localhost:8080';
var renterEmail = RENTER_EMAIL || 'bob@test.com';
var status = STATUS || 'REQUESTED';

var response = http.post(
  apiUrl + '/api/v1/test/booking',
  JSON.stringify({ renterEmail: renterEmail, status: status }),
  { 'Content-Type': 'application/json' }
);

if (response.status !== 201) {
  throw new Error('seed-booking failed (' + response.status + '): ' + response.body);
}

var data = JSON.parse(response.body);
output.TRANSACTION_ID = data.transactionId;
output.PIN = data.pin || '';
output.LISTING_LAT = String(data.listingLat || 0);
output.LISTING_LNG = String(data.listingLng || 0);
