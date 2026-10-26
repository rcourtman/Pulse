package monitoring

// PollErrorCategoryPartial is the ErrorDetail.Category recordTaskOutcome sets
// when a poll reached the connection but left some readings unread. The
// connections ledger reads it to present a degraded, still-reachable
// connection instead of an outage (#2960).
const PollErrorCategoryPartial = pollReadErrorCategory
