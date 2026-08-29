# Privacy

Pulse is designed to run locally. By default, your monitoring data stays on your server.

## Usage Data

Pulse has one outbound usage-data scope: **outbound usage telemetry** to
help me understand active installations, release uptake, and feature use in
aggregate.

Commercial activation and license-recovery runtime records stay on the Pulse
instance where they were created. They are not exported to Pulse infrastructure,
third-party analytics, support diagnostics, or ordinary Settings surfaces.

### Outbound usage telemetry

Pulse includes outbound usage telemetry that is **enabled by default**. It sends a lightweight ping on startup and once every 24 hours with a rotating pseudonymous install ID to help me understand how many active installations exist, which releases are actually deployed, which features are in use, and whether Patrol control and governed Pulse Intelligence operations are being adopted.

The telemetry payload does not include hostnames, credentials, infrastructure identifiers, IP addresses, URLs, paths, locale, prompts, chat messages, command text, action output, token values, names, email addresses, or account identifiers. Lifecycle and outcome signals are deliberately limited to closed buckets, booleans, and aggregate counts. Local service-health signals are likewise limited to closed buckets, booleans, and normalized release versions. Pulse does not send browser events or an event-level clickstream. The local service-health probe requests only Pulse's own bound listener and never transmits the listener address, request URL, response content, or raw failure text. See the full field list below.

While mock/demo fixture mode is enabled, Pulse suppresses outbound telemetry entirely: a mock-mode instance reports a synthetic fixture fleet rather than a real installation, so it never pings.

#### How to disable

- **Settings → System → General → Outbound usage telemetry** (toggle off), or
- Set the environment variable `PULSE_TELEMETRY=false`

#### How to inspect or rotate it

- **Settings → System → General → Preview payload** shows the exact heartbeat JSON Pulse would send with the current runtime state.
- **Settings → System → General → Reset ID** immediately rotates the local telemetry install ID and refreshes the previewed payload.
- If telemetry is currently disabled, the preview still shows the payload Pulse would send if you enable it.

#### Exactly what is sent

Every field is listed below with the reason it exists. Nothing else is included in the telemetry payload:

| Field | Example | Purpose |
|-------|---------|---------|
| Schema version | `13` | Identify the exact payload contract so old and new signals are not mixed silently |
| Sent at | `2026-07-23T08:30:00Z` | Date the individual heartbeat without sending a history of client activity |
| Install ID | `a1b2c3d4-...` | Distinguish active installations within one rotation window without tying telemetry to an account or person |
| Version | `6.0.0-rc.1` | Track the canonical release identity currently deployed |
| Version raw | `v6.0.0-rc.1-45-gabcdef` | Preserve the original build string when it differs so manual/dev builds do not pollute release reporting |
| Version channel | `stable`, `rc`, `dev` | Distinguish published stable/RC assets from development or prerelease builds |
| Version build | `git.45.gabcdef` | Preserve build metadata for git-describe and other non-release builds |
| Version is development | `true`/`false` | Mark manual or source-built development installs explicitly |
| Version is published release | `true`/`false` | Mark whether the running build matches a published stable or RC release asset |
| Platform | `docker` or `binary` | Understand whether runtime behavior differs between container and non-container installs |
| OS | `linux` | See whether operating-system-specific issues exist |
| Arch | `amd64` | See whether CPU-architecture-specific issues exist |
| Event | `startup` or `heartbeat` | Distinguish first-run/session starts from daily active-install heartbeats |
| Deployment method | `docker_compose`, `docker_run`, `container_other`, `systemd`, `binary_other`, or `other` | Compare coarse current-runtime installation evidence without sending an image name, filesystem path, command, or URL. Upgraded installs often fall back to `container_other` or `binary_other`, so this is not precise evidence of the original installation path |
| Known install age bucket | `under_1d`, `1_7d`, `8_30d`, `31_90d`, `91_365d`, or `over_365d` | Measure coarse time since Pulse first created the schema-v2 lifecycle record. For upgraded installs this is only a lower bound, not the original installation age |
| Activation stage | `started`, `secured`, `connected`, `monitoring`, or `outcome_observed` | Measure the highest coarse setup milestone reached without sending a user journey or event log |
| Time to first monitored resource bucket | `not_observed`, `present_at_first_observation`, `under_15m`, `15m_1h`, `1_6h`, `6_24h`, `1_3d`, `4_7d`, `8_30d`, or `over_30d` | Measure coarse time to initial monitoring value without sending exact timestamps or resource identity; `present_at_first_observation` keeps upgraded installs from being assigned an invented historical duration |
| Estate size bucket | `empty`, `1_10`, `11_50`, `51_200`, `201_1000`, or `over_1000` | Segment aggregate usage by approximate monitored-resource scale without adding a new identifier |
| Auth configured | `true`/`false` | See whether an installation has crossed the basic security setup milestone without sending auth type, usernames, or account data |
| Configured connections | `4` | Count configured monitoring connections in aggregate without sending connection names, addresses, credentials, or resource IDs |
| Monitoring active | `true`/`false` | Distinguish currently populated monitoring from historical activation without sending resource identity |
| Outcome observed 30d | `true`/`false` | See whether alert or notification outcome evidence exists in the aggregate windows without sending alert or notification content |
| PVE nodes | `3` | Understand Proxmox VE deployment size in aggregate |
| PBS instances | `1` | Understand Proxmox Backup Server adoption in aggregate |
| PMG instances | `0` | Understand Proxmox Mail Gateway adoption in aggregate |
| VMs | `25` | Understand approximate infrastructure scale in aggregate |
| Containers | `12` | Understand approximate LXC usage in aggregate |
| Agent hosts | `4` | Understand Pulse Agent adoption for node-local telemetry in aggregate |
| Docker hosts | `2` | Understand Docker monitoring adoption in aggregate |
| Docker containers | `18` | Understand Docker / Podman workload scale in aggregate |
| Kubernetes clusters | `0` | Understand Kubernetes monitoring adoption in aggregate |
| Kubernetes nodes | `3` | Understand Kubernetes node scale in aggregate |
| Kubernetes pods | `42` | Understand Kubernetes workload scale in aggregate |
| Kubernetes deployments | `8` | Understand Kubernetes deployment adoption in aggregate |
| Storage pools | `6` | Understand storage monitoring adoption in aggregate |
| Physical disks | `24` | Understand disk-health monitoring adoption in aggregate |
| Ceph clusters | `1` | Understand Ceph monitoring adoption in aggregate |
| Network shares | `5` | Understand NAS/share monitoring adoption in aggregate |
| TrueNAS systems | `1` | Understand TrueNAS integration adoption in aggregate |
| TrueNAS VMs | `2` | Understand TrueNAS VM visibility adoption in aggregate |
| TrueNAS apps | `7` | Understand TrueNAS app visibility adoption in aggregate |
| VMware hosts | `3` | Understand VMware vSphere host monitoring adoption in aggregate |
| VMware VMs | `35` | Understand VMware vSphere VM monitoring adoption in aggregate |
| VMware datastores | `4` | Understand VMware datastore visibility adoption in aggregate |
| Availability targets | `9` | Understand agentless availability-check adoption in aggregate |
| Availability probe targets | `2` | Understand external-probe adoption in aggregate; counts only, no agent names or addresses |
| Availability probe agents | `1` | Understand how many distinct probe agents installs deploy; counts only |
| AI enabled | `true`/`false` | See whether AI features are actually used before expanding or removing them |
| Patrol enabled | `true`/`false` | See whether proactive AI health patrol is used |
| Discovery enabled | `true`/`false` | See whether network or model-backed discovery is used |
| Notifications enabled | `true`/`false` | See whether alert notification delivery is configured |
| AI actions enabled | `true`/`false` | See whether AI control tools are enabled without sending action history or command content |
| Active alerts | `4` | Understand how noisy or quiet installations are in aggregate |
| Alerts fired 30d | `18` | Count unique locally retained alert occurrences in the current 30-day window without sending alert text, resource IDs, or timestamps |
| Alerts acknowledged 30d | `7` | Count acknowledgements in the current 30-day window without sending actors, reasons, alert IDs, or timestamps |
| Alerts resolved 30d | `12` | Count resolved alert records in the current 30-day window without sending resolution details, alert IDs, or resource IDs |
| Notification attempts 7d | `14` | Count delivery attempts, including retry attempts, in the locally retained seven-day queue window without sending recipients, endpoints, titles, or message content |
| Notification deliveries 7d | `11` | Count successfully delivered queue records in the local seven-day window without sending channel, recipient, endpoint, or content |
| Notification failures 7d (terminal in schema v3) | `3` | Count terminal failed or dead-lettered delivery outcomes in the local seven-day window without sending retry-attempt failures, error text, endpoint, recipient, or message content |
| Notification failures authentication 7d (schema v5) | `1` | Count terminal failures classified locally as credential or permission failures without sending raw errors, provider names, endpoints, recipients, or message content |
| Notification failures rate limited 7d (schema v5) | `0` | Count terminal failures classified locally as provider rate limiting without sending provider identity or response content |
| Notification failures connectivity 7d (schema v5) | `2` | Count terminal failures classified locally as DNS, timeout, or connection failures without sending addresses, hosts, or raw errors |
| Notification failures TLS 7d (schema v5) | `0` | Count terminal failures classified locally as certificate or TLS failures without sending certificates, hostnames, or raw errors |
| Notification failures configuration 7d (schema v5) | `0` | Count terminal failures classified locally as missing or invalid destination configuration without sending configuration values |
| Notification failures rejected 7d (schema v5) | `0` | Count terminal failures classified locally as destination request or payload rejection without sending response or payload content |
| Notification failures unknown 7d (schema v5) | `0` | Count terminal failures that do not match another fixed class without sending raw errors |
| Alert AI enabled | `true`/`false` | See whether AI analysis on alert firing is switched on, without sending alert content, resource identifiers, or analysis text |
| Relay enabled | `true`/`false` | See whether remote-access features are being used |
| SSO enabled | `true`/`false` | See whether single-sign-on support is being used |
| Multi-tenant | `true`/`false` | See whether multi-tenant/runtime-org features are being used |
| Paid license | `true`/`false` | Distinguish free from paid posture without sending the exact commercial tier |
| Has API tokens | `true`/`false` | See whether token-based automation/integration is being used without sending token counts |
| RBAC custom roles | `3` | Count roles created by the operator, excluding the roles that ship built in, without sending role names, permissions, or scopes |
| RBAC user assignments | `7` | Count user-to-role assignments without sending usernames, role names, or permissions |
| Audit reads 30d | `41` | Count how many times an entitled operator opened or exported the audit log in the current 30-day telemetry window, without sending query filters, actors, ranges, or any audit event |
| Report schedules | `5` | Count configured scheduled reports without sending schedule names, report scope, formats, or delivery recipients |
| Report schedules enabled | `4` | Count scheduled reports that are switched on, without sending schedule names, scope, or recipients |
| Report schedules run 30d | `2` | Count scheduled reports that last ran inside the current 30-day telemetry window, without sending run times, report contents, or delivery results |
| Agent profiles | `9` | Count configured agent profiles without sending profile names, targets, or profile contents |
| Update attempts 30d | `2` | Count update attempts recorded in the current 30-day telemetry window without sending download URLs, logs, or command output |
| Update successes 30d | `1` | Count successful update attempts in the current 30-day telemetry window |
| Update failures 30d | `1` | Count failed or rolled-back update attempts in the current 30-day telemetry window without sending raw errors, logs, URLs, or command output |
| Update last failure category | `download` | Send only a coarse category for the latest update failure, such as `download`, `signature`, `checksum`, `disk_space`, `extract`, `backup`, `apply`, `restart`, `rolled_back`, or `unknown` |
| Service health observed | `true`/`false` | Distinguish a release that performed the bounded local UI/API self-check from an older release with no signal |
| Service health healthy | `true`/`false` | Report whether Pulse's locally bound listener served a healthy API response, UI document, and every referenced local frontend asset without sending an address, URL, response, or error text |
| Service health failure category | `listener`, `startup`, `runtime`, `api_connectivity`, `api_status`, `ui_status`, `frontend_assets`, or `unknown` | Classify a failed local self-check or startup path into one fixed category without sending the listener address, request URL, HTTP body, asset name, IP address, or raw error |
| Service health cohort | `first_observation`, `same_version`, or `version_change` | Mark whether this direct observation is the first schema-v13 observation, another observation of the same release, or the first observed state after a release-version change |
| Service health previous version | `6.4.0` | Retain only the normalized immediately previous Pulse release identity so aggregate reporting can compare a post-upgrade before/after cohort without treating rolling counters as new-release activity |
| Service health previous observed | `true`/`false` | State whether the immediately previous release recorded the bounded local service-health signal |
| Service health previous healthy | `true`/`false` | State whether that immediately previous release's last bounded local observation was healthy, without retaining its address, URL, response, asset names, or failure text |
| Node test attempts 30d | `3` | Count node connection tests that reached the connection stage in the current 30-day telemetry window, without sending hostnames, addresses, credentials, or error text |
| Node test failures 30d | `2` | Count those connection tests that could not reach or authenticate against the target, so an install that tried to add a node and failed is distinguishable from one that never attempted it, without sending hostnames, addresses, credentials, or error text |
| Pulse Intelligence loop configured | `true`/`false` | See whether Assistant, Patrol, governed actions, or external-agent access is configured so adoption can be measured without sending configuration details |
| Pulse Intelligence loop active 30d | `true`/`false` | See whether Assistant, Patrol, external-agent, or governed-action activity occurred in the current 30-day telemetry window |
| Pulse Intelligence complete operations loop 30d | `true`/`false` | See whether Patrol issue activity reached an approved or rejected governed-action decision without sending prompts, findings, resource identifiers, command text, or action output |
| Pulse Intelligence approved execution loop 30d | `true`/`false` | See whether Patrol issue activity reached an approved governed action attempt without sending action details |
| Pulse Intelligence resolved operations loop 30d | `true`/`false` | See whether Patrol issue activity reached resolution with an approved action success without sending prompts, findings, resource identifiers, command text, or action output |
| Pulse Intelligence Patrol control completed operations loop 30d | `true`/`false` | See whether a Patrol mode starter, Patrol issue activity, contextual Assistant or external-agent collaboration, and either a rejected governed decision or an approved governed decision with a verified outcome occurred without sending prompt text, checkout details, account links, token details, resource identifiers, command text, or action output |
| Pulse Intelligence Patrol control resolved operations loop 30d | `true`/`false` | See whether a Patrol mode starter, Patrol issue activity, contextual Assistant or external-agent collaboration, an approved governed decision, and a verified outcome occurred without sending prompt text, checkout details, account links, token details, resource identifiers, command text, or action output |
| Pulse Intelligence Patrol control paid completed operations loop 30d | `true`/`false` | See whether the install currently has a paid license and Patrol mode reached a governed decision without sending the exact tier, checkout details, account links, prompts, token details, resource identifiers, command text, or action output |
| Pulse Intelligence Patrol control paid resolved operations loop 30d | `true`/`false` | See whether the install currently has a paid license and Patrol mode reached a resolved issue without sending the exact tier, checkout details, account links, prompts, token details, resource identifiers, command text, or action output |
| Pulse Intelligence Pro activation completed operations loop 30d | `true`/`false` | Compatibility mirror of the Patrol control completed field for historical aggregate reporting without sending prompt text, checkout details, account links, token details, resource identifiers, command text, or action output |
| Pulse Intelligence Pro activation resolved operations loop 30d | `true`/`false` | Compatibility mirror of the Patrol mode resolved field for historical aggregate reporting without sending prompt text, checkout details, account links, token details, resource identifiers, command text, or action output |
| Pulse Intelligence Pro activation paid completed operations loop 30d | `true`/`false` | Compatibility mirror of the paid Patrol mode decision field for historical aggregate reporting without sending exact tier, checkout details, account links, token details, resource identifiers, command text, or action output |
| Pulse Intelligence Pro activation paid resolved operations loop 30d | `true`/`false` | Compatibility mirror of the paid Patrol mode resolved field for historical aggregate reporting without sending exact tier, checkout details, account links, token details, resource identifiers, command text, or action output |
| Pulse Intelligence governed action active 30d | `true`/`false` | See whether Patrol or Assistant reached action planning, approval, approve/reject decision, or approved-action depth in the current 30-day telemetry window without sending action details |
| Pulse Intelligence Assistant operations loop 30d | `true`/`false` | See whether Assistant collaboration specifically reached the Patrol issue plus approved/rejected governed-action path without sending prompts, chat messages, tool details, findings, command text, or action output |
| Pulse Intelligence Assistant approved execution loop 30d | `true`/`false` | See whether Assistant collaboration specifically reached approved governed-action execution in the 30-day window without sending prompts, chat messages, command text, actors, or action output |
| Pulse Intelligence Assistant approved action success loop 30d | `true`/`false` | See whether Assistant collaboration specifically reached approved action success in the 30-day window without sending prompts, chat messages, verification details, command text, or action output |
| Pulse Intelligence Assistant resolved operations loop 30d | `true`/`false` | See whether Assistant collaboration specifically reached Patrol resolution plus approved action success in the 30-day window without sending findings, prompts, chat messages, verification details, command text, or action output |
| Pulse Intelligence external agent operations loop 30d | `true`/`false` | See whether direct external-agent or MCP collaboration reached the Patrol issue plus approved/rejected governed-action path without sending route parameters, prompts, resource identifiers, command text, or action output |
| Pulse Intelligence external agent approved execution loop 30d | `true`/`false` | See whether direct external-agent or MCP collaboration reached approved governed-action execution in the 30-day window without sending route parameters, command text, actors, or action output |
| Pulse Intelligence external agent approved action success loop 30d | `true`/`false` | See whether direct external-agent or MCP collaboration reached approved action success in the 30-day window without sending route parameters, command text, verification details, or action output |
| Pulse Intelligence external agent resolved operations loop 30d | `true`/`false` | See whether direct external-agent or MCP collaboration reached Patrol resolution plus approved action success in the 30-day window without sending findings, route parameters, command text, verification details, or action output |
| Pulse Intelligence MCP adapter operations loop 30d | `true`/`false` | See whether the `pulse-mcp` adapter specifically reached the Patrol issue plus approved/rejected governed-action path without sending prompts, tool inputs, route parameters, resource identifiers, command text, or action output |
| Pulse Intelligence MCP adapter approved execution loop 30d | `true`/`false` | See whether the `pulse-mcp` adapter specifically reached approved governed-action execution in the 30-day window without sending prompts, tool inputs, command text, actors, or action output |
| Pulse Intelligence MCP adapter approved action success loop 30d | `true`/`false` | See whether the `pulse-mcp` adapter specifically reached approved action success in the 30-day window without sending prompts, tool inputs, verification details, command text, or action output |
| Pulse Intelligence MCP adapter resolved operations loop 30d | `true`/`false` | See whether the `pulse-mcp` adapter specifically reached Patrol resolution plus approved action success in the 30-day window without sending findings, prompts, tool inputs, verification details, command text, or action output |
| Pulse Intelligence operations loop starter requests 30d | `3` | Count shared Patrol-work starter requests in the current 30-day telemetry window without sending prompt text, chat messages, tool inputs, resource IDs, or request details |
| Pulse Intelligence Assistant operations loop starter requests 30d | `2` | Count Assistant requests for shared Patrol work in the current 30-day telemetry window without sending prompt text, chat messages, resource IDs, or request details |
| Pulse Intelligence Patrol operations loop starter requests 30d | `1` | Count Patrol work starter requests in the current 30-day telemetry window without sending prompt text, findings, resource IDs, or request details |
| Pulse Intelligence Patrol control operations loop starter requests 30d | `1` | Count Patrol work, Patrol-mode, and historical entry-point requests for shared Patrol work in the current 30-day telemetry window without sending prompt text, checkout details, account links, resource IDs, or request details |
| Pulse Intelligence Pro activation operations loop starter requests 30d | `1` | Count historical entry-point requests for Patrol mode in the current 30-day telemetry window without sending prompt text, checkout details, account links, resource IDs, or request details |
| Pulse Intelligence MCP operations loop starter requests 30d | `1` | Count `pulse-mcp` requests for shared Patrol work in the current 30-day telemetry window without sending prompt text, tool inputs, resource IDs, route parameters, or request details |
| Pulse Intelligence Assistant AI calls 30d | `18` | Count Assistant model calls in the current 30-day telemetry window without sending prompts, responses, session IDs, or chat text |
| Pulse Intelligence Assistant context AI calls 30d | `7` | Count Assistant model calls tied to a governed resource, finding, handoff, or action context in the current 30-day telemetry window without sending prompts, responses, session IDs, resource IDs, finding IDs, or chat text |
| Pulse Intelligence Assistant tool calls 30d | `11` | Count Assistant tool calls in the current 30-day telemetry window without sending tool names, tool inputs, tool outputs, prompts, responses, session IDs, resource IDs, finding IDs, command text, or chat text |
| Pulse Intelligence Patrol AI calls 30d | `6` | Count Patrol model calls in the current 30-day telemetry window without sending provider-bound context or findings text |
| Pulse Intelligence Patrol runs 30d | `12` | Count Patrol investigations in the current 30-day telemetry window |
| Pulse Intelligence Patrol new findings 30d | `5` | Count new findings produced by Patrol in the current 30-day telemetry window without sending finding IDs or details |
| Pulse Intelligence Patrol investigations 30d | `3` | Count findings investigated by Patrol in the current 30-day telemetry window without sending finding IDs, resource IDs, or details |
| Pulse Intelligence Patrol resolved findings 30d | `2` | Count findings resolved or fix-verified in the current 30-day telemetry window without sending finding IDs, resource IDs, fix details, or verification detail |
| Pulse Intelligence Patrol blocked cause | `provider_not_configured` | See one fixed machine cause code when Patrol is enabled but blocked from running, so an install whose Patrol can never work is distinguishable from one that runs and finds nothing, without sending blocked-reason text, provider endpoints, model names, error text, or configuration |
| Pulse Intelligence external agent enabled | `true`/`false` | See whether at least one token can use the external Pulse Intelligence agent/MCP surface without sending token counts, names, scopes, or values |
| Pulse Intelligence external agent used 30d | `true`/`false` | See whether an external-agent-capable API token reached a Pulse Intelligence agent/MCP route in the current 30-day telemetry window without sending token identity, route parameters, resource IDs, or request details |
| Pulse Intelligence MCP adapter used 30d | `true`/`false` | See whether the `pulse-mcp` adapter reached a Pulse Intelligence agent/MCP route in the current 30-day telemetry window without sending token identity, route parameters, resource IDs, prompts, or request details |
| Pulse Intelligence external agent context requests 30d | `8` | Count external-agent/MCP resource-context and fleet-context requests in the current 30-day telemetry window without sending resource IDs, route parameters, or request details |
| Pulse Intelligence external agent event stream requests 30d | `3` | Count external-agent/MCP event-stream requests in the current 30-day telemetry window without sending event content, route parameters, or request details |
| Pulse Intelligence external agent provisioning requests 30d | `2` | Count external-agent/MCP provisioning requests in the current 30-day telemetry window without sending discovered resources, credentials, route parameters, or request details |
| Pulse Intelligence external agent operator state requests 30d | `5` | Count external-agent/MCP operator-state requests in the current 30-day telemetry window without sending state payloads, route parameters, or request details |
| Pulse Intelligence external agent finding requests 30d | `4` | Count external-agent/MCP finding-list and finding-decision requests in the current 30-day telemetry window without sending finding IDs, finding text, route parameters, or request details |
| Pulse Intelligence external agent action requests 30d | `1` | Count external-agent/MCP action-plan, action-decision, and action-execution requests in the current 30-day telemetry window without sending command text, action output, route parameters, or request details |
| Pulse Intelligence action plans 30d | `4` | Count governed action plans in the current 30-day telemetry window without sending command text, resource IDs, or plan details |
| Pulse Intelligence approval requests 30d | `2` | Count approval-gated action requests in the current 30-day telemetry window without sending approvers, reasons, command text, or targets |
| Pulse Intelligence rejected action decisions 30d | `1` | Count rejected governed action decisions in the current 30-day telemetry window without sending approvers, reasons, command text, targets, or action IDs |
| Pulse Intelligence approved action decisions 30d | `1` | Count approved governed action decisions in the current 30-day telemetry window without sending approvers, reasons, command text, targets, or action IDs |
| Pulse Intelligence approved action attempts 30d | `1` | Count approved governed action attempts in the current 30-day telemetry window without sending action output, command text, or verification detail |
| Pulse Intelligence approved action successes 30d | `1` | Count approved governed actions that completed successfully in the current 30-day telemetry window without sending action output, command text, resource IDs, actors, reasons, or verification detail |
| Pulse Intelligence Patrol action plans 30d | `2` | Count governed action plans proposed from Patrol investigations in the current 30-day telemetry window without sending action IDs, finding IDs, investigation IDs, command text, resource IDs, or plan details |
| Pulse Intelligence Patrol approval requests 30d | `2` | Count approval-gated Patrol-origin actions in the current 30-day telemetry window without sending action IDs, finding IDs, approvers, reasons, command text, or targets |
| Pulse Intelligence Patrol rejected action decisions 30d | `1` | Count rejected Patrol-origin action decisions in the current 30-day telemetry window without sending action IDs, finding IDs, approvers, reasons, command text, or targets |
| Pulse Intelligence Patrol approved action decisions 30d | `1` | Count approved Patrol-origin action decisions in the current 30-day telemetry window without sending action IDs, finding IDs, approvers, reasons, command text, or targets |
| Pulse Intelligence Patrol approved action attempts 30d | `1` | Count approved Patrol-origin action attempts in the current 30-day telemetry window without sending action IDs, finding IDs, command text, output, resources, or verification detail |
| Pulse Intelligence Patrol approved action successes 30d | `1` | Count approved Patrol-origin actions that completed successfully in the current 30-day telemetry window without sending action IDs, finding IDs, command text, output, resources, actors, reasons, or verification detail |
| Pulse Intelligence approved action failures (pre-dispatch) 30d | `1` | Count approved governed actions refused before dispatch (for example an expired or drifted plan) in the current 30-day telemetry window without sending action output, command text, resource IDs, actors, or reasons |
| Pulse Intelligence approved action failures (execution) 30d | `1` | Count approved governed actions whose dispatched execution failed in the current 30-day telemetry window without sending action output, error text, command text, resource IDs, or actors |
| Pulse Intelligence approved action failures (unverified) 30d | `1` | Count approved governed actions that executed but whose outcome verification was not confirmed in the current 30-day telemetry window without sending verification evidence, action output, command text, resource IDs, or actors |
| Pulse Intelligence approved action stuck executing 30d | `1` | Count approved governed actions abandoned in the executing state in the current 30-day telemetry window without sending action output, command text, resource IDs, or actors |
| Pulse Intelligence approved action in flight 30d | `1` | Count approved governed actions still inside the legitimate execution window so the action-attempt total reconciles without sending action details |
| Pulse Intelligence approved action unclassified 30d | `1` | Count approved governed action attempts whose authoritative audit could not be classified so accounting gaps remain visible without sending action IDs or content |
| Pulse Intelligence approved action refusals plan stale 30d | `1` | Count pre-dispatch refusals caused by an expired, drifted, or legacy-unbound plan without sending plan or resource details |
| Pulse Intelligence approved action refusals: policy 30d | `1` | Count pre-dispatch refusals caused by an operator or policy safety control without sending policy, actor, resource, or command details |
| Pulse Intelligence approved action refusals: capability 30d | `1` | Count pre-dispatch refusals caused by a dry-run-only or unavailable capability without sending capability names, targets, or command details |
| Pulse Intelligence approved action refusals: target changed 30d | `1` | Count pre-dispatch refusals caused by target or inventory drift after planning without sending target, inventory, or command details |
| Pulse Intelligence approved action refusals: prerequisite 30d | `1` | Count pre-dispatch refusals caused by an unavailable inspection, busy or unhealthy package manager, or failed bounded preflight without sending raw errors, targets, or command details |
| Pulse Intelligence approved action refusals: contract 30d | `1` | Count pre-dispatch refusals caused by an invalid typed action contract without sending payloads, targets, or command details |
| Pulse Intelligence approved action refusals: uncoded 30d | `1` | Count pre-dispatch refusals that arrived with no machine reason code, which is what an agent older than the typed refusal contract reports, without sending raw error or action content |
| Pulse Intelligence approved action refusals: other 30d | `1` | Count pre-dispatch refusals not covered by the fixed categories without sending raw error or action content |
| Pulse Intelligence verified finding resolutions 30d | `1` | Count approved Patrol-origin actions whose execution succeeded and whose linked finding postcondition was independently confirmed, without sending finding IDs, action IDs, evidence, resources, or fix details |
| Pulse Intelligence approved action last failure reason 30d | `plan_drift` | See one fixed machine reason code for the most recent approved-action failure in the current 30-day telemetry window without sending error text, action output, command text, resource IDs, or actors |

Telemetry schema v3 corrects the meaning of `notification_failures_7d`: v2
counted every unsuccessful queue attempt, including attempts that later
succeeded on retry; v3 counts only terminal `failed` or dead-letter outcomes.
The adoption report keeps those schema cohorts separate, so legacy retry noise
is not compared with current terminal-delivery failures. This correction adds
no notification content or identity fields.

Telemetry schema v4 adds complete approved-action outcome accounting, fixed
pre-dispatch refusal categories, and an independently verified
action-to-finding resolution count. These remain aggregate counters and do not
send action, finding, resource, evidence, actor, or command identity or content.

Telemetry schema v5 adds seven fixed notification terminal-failure counters so
fleet health can distinguish authentication, rate limiting, connectivity, TLS,
configuration, destination rejection, and unknown failures. Classification is
performed locally; raw error text and destination/provider identity are never
included in the payload.

Telemetry schema v13 adds a direct local UI/API service observation. Pulse
checks its own bound listener, `/api/health`, the UI document, and the local
frontend assets referenced by that document. It retains only the current and
immediately previous normalized release observation, so a version-change
cohort can compare before and after health without attributing rolling update
or usage counters to the new release. Listener, startup, runtime, API, UI, and
asset failures collapse into fixed categories. Addresses, URLs, IP addresses,
asset names, response bodies, and raw errors are neither sent nor stored.

#### Server-side handling and retention

- Telemetry pings are stored on the Pulse license server only for aggregate install/use analysis.
- The license server stores only the same coarse telemetry fields listed above; it does not expand them into exact commercial tiers, exact API-token counts, prompts, chat messages, command text, action output, token values, or resource identifiers.
- Pulse may derive aggregate Pulse Intelligence adoption reports from those same rows, including whether an install reached Patrol issue activity, Patrol resolution, Assistant, direct external-agent, or MCP collaboration, Patrol mode starter use, paid Patrol mode cohorts, governed-action activity, approved or rejected action decisions, approved action success, completed Patrol control work, recent retention, and observed free-to-paid movement within the source window. Those reports do not add prompts, findings, resource identifiers, tool names, tool inputs, tool outputs, command payloads, action outputs, account links, or exact commercial tiers.
- Aggregate reports preserve the closed deployment-method buckets, but treat them as best-effort current-runtime evidence. In particular, `container_other` and `binary_other` are unknown fallbacks for many upgraded installs, not precise original installation provenance.
- Aggregate reports describe the known-age bucket as time since Pulse first created the schema-v2 lifecycle record. It is only a lower bound for an upgraded install and must not be presented as original installation age.
- Aggregate release-health reports use the direct schema-v13 current and immediately previous observations. They do not infer new-release health from rolling update, feature, alert, or action counters.
- External-agent/MCP activity is stored only as a coarse adapter-origin flag plus capability-class counters: context, event stream, provisioning, operator state, findings, and action requests.
- The receiver stores only fields in its versioned telemetry allowlist. A cross-repository parity check prevents client fields from being silently dropped and prevents the storage contract from growing beyond the disclosed payload.
- Telemetry rows older than **90 days** are purged automatically.
- The license server uses request IP addresses transiently for abuse/rate limiting, but it does **not** store IP addresses in telemetry rows.

#### What is NOT sent

- No IP addresses are included in the telemetry payload or stored in telemetry rows
- No hostnames, node names, VM names, or any infrastructure identifiers
- No URLs, filesystem paths, locale, browser events, or event-level clickstream
- No Proxmox credentials, API tokens, or passwords
- No alert content, AI prompts, chat messages, tool names, tool inputs, tool outputs, command text, action output, or token values
- No names, email addresses, account identifiers, or other intentionally identifying personal content

#### Install ID rotation

The telemetry install ID is pseudonymous, is not tied to a Pulse account, and rotates automatically every 30 days.
Pulse keeps it only to avoid treating every startup ping as a brand-new install
while still limiting long-term linkage from one heartbeat window to the next.
Operators can also rotate it immediately from **Settings → System → General → Reset ID**.

Pulse separately keeps three coarse lifecycle values on the local instance: the
first v2 observation time, the first monitored-resource milestone time, and the
highest activation stage reached. This local state contains no user, account,
resource, URL, or content identifiers. It exists so daily pings can report
buckets instead of exporting a sequence of setup events.

#### Source code

The telemetry implementation is in [`internal/telemetry/telemetry.go`](../internal/telemetry/telemetry.go). You can read the `Ping` struct to see every field that is transmitted.

## No Third-Party Analytics

- There is no third-party analytics SDK in the frontend.
- Telemetry pings go only to the Pulse license server (`license.pulserelay.pro`), not to any third-party service.

## Optional Outbound Connections (Explicitly Enabled)

Pulse can make outbound connections when you enable specific features:

- **AI providers**: when AI features are configured, Pulse sends only the context required for your request to the provider you chose. This can include active Patrol objective briefs and their optional operator context when they apply to a Patrol run. Retained objective text and observer artifacts are encrypted at rest in the local organization data directory; objective text is not included in Pulse usage telemetry. A provider may return a model-authored observer proposal for an uncovered objective; Pulse encrypts that artifact with the retained objective, excludes it from public objective reads and later prompt seeds, and does not include its content in usage telemetry or audit messages. Saving an objective by itself does not call a model. Local providers stay on your network; non-local hosted providers receive provider-bound context directly from your Pulse instance. AI prompts from self-managed installs do not transit Pulse infrastructure. Before non-local model requests leave the instance, governed resource details use the same resource-policy redaction shown in Data Handling: local-only resource details are omitted from detailed prompt sections or replaced with policy-safe summaries, and known restricted resource identifiers are redacted where they appear in provider-bound context. See `docs/AI.md`.
- **Relay / Remote Access**: when relay is enabled, Pulse connects to the configured relay endpoint to enable secure remote web access, Pulse Mobile pairing for handoff, and push notifications. See Settings → Remote Access.
- **Update checks**: Pulse can check for new releases/updates (for example via GitHub release metadata) depending on your deployment and configuration.
