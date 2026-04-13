.PHONY: test-mobile-e2e test-mobile-e2e-auth test-mobile-e2e-discovery \
        test-mobile-e2e-listing test-mobile-e2e-booking test-mobile-e2e-handoff \
        test-mobile-e2e-messaging test-mobile-e2e-profile test-mobile-e2e-disputes \
        test-mobile-e2e-ratings

# Run the full E2E suite (all flows)
test-mobile-e2e:
	cd mobile && ~/.maestro/bin/maestro test e2e/flows/ --env-file e2e/config/dev.env

# Run auth flows only
test-mobile-e2e-auth:
	cd mobile && ~/.maestro/bin/maestro test e2e/flows/auth/ --env-file e2e/config/dev.env

# Run discovery flows only
test-mobile-e2e-discovery:
	cd mobile && ~/.maestro/bin/maestro test e2e/flows/discovery/ --env-file e2e/config/dev.env

# Run listing flows only
test-mobile-e2e-listing:
	cd mobile && ~/.maestro/bin/maestro test e2e/flows/listing/ --env-file e2e/config/dev.env

# Run booking flows only
test-mobile-e2e-booking:
	cd mobile && ~/.maestro/bin/maestro test e2e/flows/booking/ --env-file e2e/config/dev.env

# Run handoff flows only
test-mobile-e2e-handoff:
	cd mobile && ~/.maestro/bin/maestro test e2e/flows/handoff/ --env-file e2e/config/dev.env

# Run messaging flows only
test-mobile-e2e-messaging:
	cd mobile && ~/.maestro/bin/maestro test e2e/flows/messaging/ --env-file e2e/config/dev.env

# Run profile flows only
test-mobile-e2e-profile:
	cd mobile && ~/.maestro/bin/maestro test e2e/flows/profile/ --env-file e2e/config/dev.env

# Run dispute flows only
test-mobile-e2e-disputes:
	cd mobile && ~/.maestro/bin/maestro test e2e/flows/disputes/ --env-file e2e/config/dev.env

# Run ratings flows only
test-mobile-e2e-ratings:
	cd mobile && ~/.maestro/bin/maestro test e2e/flows/ratings/ --env-file e2e/config/dev.env
