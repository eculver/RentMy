// Generates a unique test email for registration flows.
// Maestro runScript sets output variables via the `output` object.
// Usage in YAML:
//   - runScript: e2e/scripts/gen-unique-email.js
//     env:
//       OUTPUT: UNIQUE_EMAIL
var epoch = Date.now();
output.UNIQUE_EMAIL = 'e2e+' + epoch + '@test.com';
