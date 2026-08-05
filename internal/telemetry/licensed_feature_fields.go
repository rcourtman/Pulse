package telemetry

// LicensedFeatureAdoptionFields lists every telemetry field whose only purpose
// is to measure adoption of a licensed feature.
//
// Membership carries an obligation, enforced by the guard in
// pkg/server/telemetry_licensed_features_guard_test.go: an install that uses
// none of these features must report the zero value for every field here. A
// field that reads the same on a used and an unused install measures nothing,
// and is worse than no field at all because it looks like data.
//
// This registry exists because that exact defect shipped twice. Schema v6 added
// audit_logging_persistent and audit_events_30d, both sourced from an audit
// store that is installed unconditionally for defense in depth, so both
// reported identically on free and paid installs. The same schema removed
// pulse_intelligence_patrol_autofixes_30d, which had been hardcoded to zero
// with no increment site anywhere. Three instances of one bug class is a guard,
// not a habit.
//
// Adding a field here without teaching the guard how to exercise it fails the
// guard, which is the intended friction.
var LicensedFeatureAdoptionFields = []string{
	"alert_ai_enabled",
	"rbac_custom_roles",
	"rbac_user_assignments",
	"audit_reads_30d",
	"report_schedules",
	"report_schedules_enabled",
	"report_schedules_run_30d",
	"agent_profiles",
}
