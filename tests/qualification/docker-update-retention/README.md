# Docker update tracking retention

Run `go test -race ./internal/alerts -run 'TestDockerUpdate|TestCheckDockerContainerImageUpdate|TestCleanupStaleMaps|TestCleanupDockerContainerAlerts|TestCleanup$|TestUpdateConfig.*DockerContainerUpdate|TestEvaluateDockerContainerClearsUpdate'  -count=1`.

`TestDockerUpdateTrackingSurvivesDailyCleanup` seeds a first detection 25 hours
ago, with a cached registry result, and runs the real update evaluator and
hourly tracking cleanup. Both 24-hour (already firing) and 48-hour (pending)
delays must retain the detection time. Missing/error results retain it too;
an affirmative no-update result still clears the incident and both tracking
identities. Tracking is reclaimed after observations stop, and existing tests
cover host identity changes, removal and disabled update policy.

The regression fails against the preceding source in both delay cases: cleanup
uses first detection as inactivity and discards live tracking. Keep pending age
separate from last observation. The new timestamps are ephemeral, bounded by the
same removal/configuration/24-hour inactivity paths as their tracking entries;
they are not registry-check timestamps or persisted evidence.

This proves a tracking reset defect, not the cause of every open/resolved/open
cycle in issue #2048. The legacy recovery adapter labels an existing clear with
unknown confidence; it does not initiate that clear. No registry requests,
Docker updates, reporter installation or release availability are exercised.
