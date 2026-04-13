// seed-booking.js — Creates a REQUESTED booking via the test-only backend endpoint.
//
// Requires the backend to be running with E2E_MODE=true.
//
// Env vars (set in the calling Maestro flow):
//   API_URL        — Backend base URL (default: http://localhost:8080)
//   RENTER_EMAIL   — Email of the renter account (default: bob@test.com)
//
// Output variables (readable as ${TRANSACTION_ID} in the flow):
//   TRANSACTION_ID — The created booking's transaction ID

var apiUrl = API_URL || 'http://localhost:8080';
var renterEmail = RENTER_EMAIL || 'bob@test.com';

var response = http.post(
  apiUrl + '/api/v1/test/booking',
  JSON.stringify({ renterEmail: renterEmail }),
  { 'Content-Type': 'application/json' }
);

if (response.status !== 201) {
  throw new Error('seed-booking failed (' + response.status + '): ' + response.body);
}

var data = JSON.parse(response.body);
output.TRANSACTION_ID = data.transactionId;
