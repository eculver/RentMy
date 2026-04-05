# Branch Stack

Linear chain of task branches. Each branch was created from the HEAD of the previous.

## Stack Order (bottom → top)

```
main
  └─ eculver/agent-bootstrap          # Harness setup: CLAUDE.md, progress.json, plans, runner
       └─ task-1.1-user-service
            └─ task-1.2-media-service
                 └─ task-1.3-listing-service
                      └─ task-1.4-auth-screens
                           └─ task-1.5-listing-creation-flow
                                └─ task-1.6-profile-screen
                                     └─ task-2.1-discovery-service
                                          └─ task-2.2-payment-service
                                               └─ task-2.3-feed-screen
                                                    └─ task-2.4-search-screen
                                                         └─ task-2.5-map-screen
                                                              └─ task-2.6-listing-detail-screen
                                                                   └─ task-2.7-checkout-screen
                                                                        └─ task-3.1-booking-service
                                                                             └─ task-3.2-proximity-service
                                                                                  └─ task-3.3-notification-service
                                                                                       └─ task-3.4-messaging-service
                                                                                            └─ task-3.5-booking-flow
                                                                                                 └─ task-3.6-handoff-screens
                                                                                                      └─ task-3.7-messaging-screen
                                                                                                           └─ task-4.1-model-router
                                                                                                                └─ task-4.2-verification-agent
                                                                                                                     └─ task-4.3-appraisal-agent
                                                                                                                          └─ task-4.4-risk-agent
                                                                                                                               └─ task-4.5-agreement-agent
                                                                                                                                    └─ task-4.6-kyc-booking-flow
                                                                                                                                         └─ task-4.7-ai-autofill
                                                                                                                                              └─ task-4.8-backfill-existing-data
                                                                                                                                                   └─ shim-test-infrastructure  ← YOU ARE HERE
                                                                                                                                                        └─ (Phase 7 tasks)
                                                                                                                                                             └─ (Phase 5 tasks)
                                                                                                                                                                  └─ (Phase 6 tasks)
```

## Notes

- All branches use vanilla git (Graphite not yet enabled)
- `eculver/agent-bootstrap` is kept in sync with harness improvements (recovery protocol, log viewer, etc.)
- `shim-test-infrastructure` inserts test infrastructure and requirements between Phase 4 and Phase 5
- The agent updates this file when creating new branches (added to handoff doc workflow)
