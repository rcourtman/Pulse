package licensing

const legacyV5AgentLimitKey = "max_agents"
const legacyV5NodeLimitKey = "max_nodes"

var legacyV5MonitoredSystemLimitAliasKeys = [...]string{
	legacyV5AgentLimitKey,
	legacyV5NodeLimitKey,
}
