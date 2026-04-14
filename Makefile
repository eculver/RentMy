.PHONY: test-mobile-e2e test-mobile-e2e-auth test-mobile-e2e-discovery \
        test-mobile-e2e-listing test-mobile-e2e-booking test-mobile-e2e-handoff \
        test-mobile-e2e-messaging test-mobile-e2e-profile test-mobile-e2e-disputes \
        test-mobile-e2e-ratings _e2e-clean-drivers

MAESTRO = ~/.maestro/bin/maestro
E2E_ENV = $(shell grep -v '^\#' mobile/e2e/config/dev.env | grep -v '^$$' | sed 's/^/-e /')

# Kill stale Maestro driver/xcodebuild processes that can hog port 7001
_e2e-clean-drivers:
	@-pkill -f 'maestro-driver-iosUITests-Runner' 2>/dev/null || true
	@-pkill -f 'xcodebuild test-without-building.*maestro-driver' 2>/dev/null || true
	@sleep 1

# Run the full E2E suite (all flows)
test-mobile-e2e: _e2e-clean-drivers
	cd mobile && $(MAESTRO) test e2e/flows/ $(E2E_ENV)

# Run auth flows only
test-mobile-e2e-auth:
	cd mobile && $(MAESTRO) test e2e/flows/auth/ $(E2E_ENV)

# Run discovery flows only
test-mobile-e2e-discovery:
	cd mobile && $(MAESTRO) test e2e/flows/discovery/ $(E2E_ENV)

# Run listing flows only
test-mobile-e2e-listing:
	cd mobile && $(MAESTRO) test e2e/flows/listing/ $(E2E_ENV)

# Run booking flows only
test-mobile-e2e-booking:
	cd mobile && $(MAESTRO) test e2e/flows/booking/ $(E2E_ENV)

# Run handoff flows only
test-mobile-e2e-handoff:
	cd mobile && $(MAESTRO) test e2e/flows/handoff/ $(E2E_ENV)

# Run messaging flows only
test-mobile-e2e-messaging:
	cd mobile && $(MAESTRO) test e2e/flows/messaging/ $(E2E_ENV)

# Run profile flows only
test-mobile-e2e-profile:
	cd mobile && $(MAESTRO) test e2e/flows/profile/ $(E2E_ENV)

# Run dispute flows only
test-mobile-e2e-disputes:
	cd mobile && $(MAESTRO) test e2e/flows/disputes/ $(E2E_ENV)

# Run ratings flows only
test-mobile-e2e-ratings:
	cd mobile && $(MAESTRO) test e2e/flows/ratings/ $(E2E_ENV)
