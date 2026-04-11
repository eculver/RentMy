# Phase 8 — Test Seed Data

## Test Users

| Name | Email | Password | User ID |
|------|-------|----------|---------|
| Alice Test | alice@test.com | password123 | 01KNZ4649E3NV0PJB82YHG38CQ |
| Bob Renter | bob@test.com | password123 | 01KNZ468WYRDRAGT1VSA3B4GHA |

## Test Listings (all owned by Alice)

| Title | Listing ID | Daily Rate |
|-------|-----------|------------|
| DeWalt Power Drill | 01KNZ47G1A5JWQNZ8YR8YFKS66 | $15 |
| Canon EOS R6 Camera | 01KNZ47ZKWMNR3MRWH1AKYEDT3 | $75 |
| Stand Up Paddleboard | 01KNZ47ZRQ4CA2NWFQ31FD7HF0 | $35 |
| Camping Tent 4-Person | 01KNZ47ZT5QHZJFDEEK43AVN8R | $20 |
| Pressure Washer 3000 PSI | 01KNZ47ZV9EJN9WVECTRSZDX1K | $45 |

## Limitations

- **No bookings**: POST /api/v1/bookings requires Stripe payment method — fails with placeholder API keys
- **No messages**: Messages are nested under /bookings/{id}/messages — require a booking first
- **Location**: Simulator location set to 34.0522, -118.2437 (Los Angeles)

## Known Issues Found During Seeding

1. `.env` had wrong postgres port (5432 vs 5433 from docker-compose) — fixed
2. App bypasses auth gate on fresh install — goes straight to feed instead of login
3. Location shows "unavailable" despite simulated location being set
