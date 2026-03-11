export const PMG_EMPTY_STATE_TITLE = 'No Mail Gateways configured';

export const PMG_EMPTY_STATE_DESCRIPTION =
  'Add a Proxmox Mail Gateway via Settings → Infrastructure → Proxmox to start collecting mail analytics and security metrics.';

export const PMG_LOADING_STATE_TITLE = 'Loading mail gateway data...';

export const PMG_LOADING_STATE_DESCRIPTION = 'Connecting to the monitoring service.';

export const PMG_DISCONNECTED_STATE_TITLE = 'Connection lost';

export const PMG_SEARCH_PLACEHOLDER = 'Search gateways...';

export const PMG_DETAILS_EMPTY_STATE_TITLE = 'No PMG details for this resource yet';

export const PMG_DETAILS_EMPTY_STATE_DESCRIPTION =
  "Pulse hasn't ingested PMG analytics for this instance.";

export const PMG_DETAILS_LOADING_STATE_TITLE = 'Loading mail gateway details...';

export const PMG_DETAILS_LOADING_STATE_DESCRIPTION = 'Fetching PMG resource details.';

export const PMG_DETAILS_FAILURE_STATE_TITLE = 'Failed to load PMG details';
export const PMG_DETAILS_DEFAULT_RESOURCE_NAME = 'Mail Gateway';
export const PMG_DETAILS_UNKNOWN_HOST_LABEL = 'Unknown host';
export const PMG_DETAILS_UPDATED_PREFIX = 'Updated';
export const PMG_DETAILS_NODES_SECTION_TITLE = 'Nodes';
export const PMG_DETAILS_RELAY_DOMAINS_SECTION_TITLE = 'Relay Domains';
export const PMG_DETAILS_DOMAIN_STATS_SECTION_TITLE = 'Domain Stats';
export const PMG_DETAILS_SPAM_DISTRIBUTION_SECTION_TITLE = 'Spam Distribution';
export const PMG_DETAILS_AS_OF_PREFIX = 'As of';
export const PMG_DETAILS_DOMAIN_SEARCH_PLACEHOLDER = 'Search domains...';
export const PMG_DETAILS_NODE_COLUMN_LABEL = 'Node';
export const PMG_DETAILS_ROLE_COLUMN_LABEL = 'Role';
export const PMG_DETAILS_STATUS_COLUMN_LABEL = 'Status';
export const PMG_DETAILS_QUEUE_COLUMN_LABEL = 'Queue';
export const PMG_DETAILS_DOMAIN_COLUMN_LABEL = 'Domain';
export const PMG_DETAILS_COMMENT_COLUMN_LABEL = 'Comment';
export const PMG_DETAILS_MAIL_COLUMN_LABEL = 'Mail';
export const PMG_DETAILS_SPAM_COLUMN_LABEL = 'Spam';
export const PMG_DETAILS_VIRUS_COLUMN_LABEL = 'Virus';
export const PMG_DETAILS_BYTES_COLUMN_LABEL = 'Bytes';

export function getPMGDetailsDrawerPresentation() {
  return {
    defaultResourceName: PMG_DETAILS_DEFAULT_RESOURCE_NAME,
    unknownHostLabel: PMG_DETAILS_UNKNOWN_HOST_LABEL,
    updatedPrefix: PMG_DETAILS_UPDATED_PREFIX,
    nodesSectionTitle: PMG_DETAILS_NODES_SECTION_TITLE,
    relayDomainsSectionTitle: PMG_DETAILS_RELAY_DOMAINS_SECTION_TITLE,
    domainStatsSectionTitle: PMG_DETAILS_DOMAIN_STATS_SECTION_TITLE,
    spamDistributionSectionTitle: PMG_DETAILS_SPAM_DISTRIBUTION_SECTION_TITLE,
    asOfPrefix: PMG_DETAILS_AS_OF_PREFIX,
    domainSearchPlaceholder: PMG_DETAILS_DOMAIN_SEARCH_PLACEHOLDER,
    nodeColumnLabel: PMG_DETAILS_NODE_COLUMN_LABEL,
    roleColumnLabel: PMG_DETAILS_ROLE_COLUMN_LABEL,
    statusColumnLabel: PMG_DETAILS_STATUS_COLUMN_LABEL,
    queueColumnLabel: PMG_DETAILS_QUEUE_COLUMN_LABEL,
    domainColumnLabel: PMG_DETAILS_DOMAIN_COLUMN_LABEL,
    commentColumnLabel: PMG_DETAILS_COMMENT_COLUMN_LABEL,
    mailColumnLabel: PMG_DETAILS_MAIL_COLUMN_LABEL,
    spamColumnLabel: PMG_DETAILS_SPAM_COLUMN_LABEL,
    virusColumnLabel: PMG_DETAILS_VIRUS_COLUMN_LABEL,
    bytesColumnLabel: PMG_DETAILS_BYTES_COLUMN_LABEL,
  } as const;
}

export function getPMGDisconnectedState(reconnecting: boolean) {
  return {
    title: PMG_DISCONNECTED_STATE_TITLE,
    description: reconnecting
      ? 'Attempting to reconnect…'
      : 'Unable to connect to the backend server',
    actionLabel: reconnecting ? undefined : 'Reconnect now',
  };
}

export function getPMGSearchEmptyState(term: string) {
  return {
    description: `No gateways match "${term}"`,
    actionLabel: 'Clear search',
  };
}
