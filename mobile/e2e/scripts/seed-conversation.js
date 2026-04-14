// seed-conversation.js — Seeds a conversation (pre-existing messages on a booking)
// via real API endpoints (no test-only endpoints required).
//
// Steps:
//   1. Login as renter (bob) → get access token
//   2. Login as host (alice) → get access token
//   3. Fetch bob's bookings to find one with alice
//   4. Send messages from both parties via POST /bookings/:id/messages
//
// Env vars (set in the calling Maestro flow):
//   API_URL        — Backend base URL (default: http://localhost:8080)
//   RENTER_EMAIL   — Email of the renter account (default: bob@test.com)
//   RENTER_PASSWORD — Password (default: password123)
//   HOST_EMAIL     — Email of the host account (default: alice@test.com)
//   HOST_PASSWORD  — Password (default: password123)
//
// Output variables (readable as ${TRANSACTION_ID} etc. in the flow):
//   TRANSACTION_ID  — The booking's transaction ID used for the conversation
//   OTHER_PARTY_NAME — Display name of the host (other party)
//   LISTING_TITLE   — Title of the listing the booking is for

var apiUrl = API_URL || 'http://localhost:8080';
var renterEmail = RENTER_EMAIL || 'bob@test.com';
var renterPassword = RENTER_PASSWORD || 'password123';
var hostEmail = HOST_EMAIL || 'alice@test.com';
var hostPassword = HOST_PASSWORD || 'password123';

// 1. Login as renter
var renterLoginResp = http.post(
  apiUrl + '/api/v1/auth/login',
  JSON.stringify({ email: renterEmail, password: renterPassword }),
  { 'Content-Type': 'application/json' }
);
if (renterLoginResp.status !== 200) {
  throw new Error('renter login failed (' + renterLoginResp.status + '): ' + renterLoginResp.body);
}
var renterToken = JSON.parse(renterLoginResp.body).accessToken;

// 2. Login as host
var hostLoginResp = http.post(
  apiUrl + '/api/v1/auth/login',
  JSON.stringify({ email: hostEmail, password: hostPassword }),
  { 'Content-Type': 'application/json' }
);
if (hostLoginResp.status !== 200) {
  throw new Error('host login failed (' + hostLoginResp.status + '): ' + hostLoginResp.body);
}
var hostToken = JSON.parse(hostLoginResp.body).accessToken;

// 3. Fetch renter's bookings to find one shared with the host
var bookingsResp = http.get(
  apiUrl + '/api/v1/users/me/bookings',
  { 'Authorization': 'Bearer ' + renterToken }
);
if (bookingsResp.status !== 200) {
  throw new Error('fetch bookings failed (' + bookingsResp.status + '): ' + bookingsResp.body);
}
var bookingsData = JSON.parse(bookingsResp.body);
var bookings = bookingsData.bookings || bookingsData;

if (!bookings || bookings.length === 0) {
  throw new Error('seed-conversation: renter has no bookings — run setup.sh first');
}

// Pick the first booking (most recent)
var booking = bookings[0];
var transactionId = booking.id || booking.transactionId;

// 4. Send seed messages via real messaging API
var msg1Resp = http.post(
  apiUrl + '/api/v1/bookings/' + transactionId + '/messages',
  JSON.stringify({ content: 'Hi! Is the item ready for pickup?' }),
  {
    'Content-Type': 'application/json',
    'Authorization': 'Bearer ' + renterToken,
  }
);
if (msg1Resp.status !== 201) {
  throw new Error('renter send message failed (' + msg1Resp.status + '): ' + msg1Resp.body);
}

var msg2Resp = http.post(
  apiUrl + '/api/v1/bookings/' + transactionId + '/messages',
  JSON.stringify({ content: 'Yes, come by anytime after 10am!' }),
  {
    'Content-Type': 'application/json',
    'Authorization': 'Bearer ' + hostToken,
  }
);
if (msg2Resp.status !== 201) {
  throw new Error('host send message failed (' + msg2Resp.status + '): ' + msg2Resp.body);
}

// Set output variables for the calling Maestro flow
output.TRANSACTION_ID = transactionId;
output.OTHER_PARTY_NAME = booking.hostName || 'Alice Host';
output.LISTING_TITLE = booking.listingTitle || 'Test Listing';
