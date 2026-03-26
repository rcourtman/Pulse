import { beforeEach, describe, expect, it } from 'vitest';

import { renderAccountUI, renderAddWorkspaceSection, renderTeamSection } from './account_view';
import type { PortalAccountState, PortalAccountUIEntry } from './types';

function createEntry(overrides: Partial<PortalAccountUIEntry> = {}): PortalAccountUIEntry {
  return {
    addWorkspaceOpen: false,
    teamVisible: false,
    teamLoading: false,
    teamError: '',
    teamMembers: [],
    ...overrides,
  };
}

describe('account view', function() {
  beforeEach(function() {
    document.body.innerHTML = '';
  });

  it('renders add-workspace visibility from account UI state', function() {
    document.body.innerHTML = '<div id="add-ws-form-acct_1" class="add-workspace-form"></div>';

    renderAddWorkspaceSection('acct_1', createEntry({ addWorkspaceOpen: true }));
    expect(document.getElementById('add-ws-form-acct_1')?.classList.contains('visible')).toBe(true);

    renderAddWorkspaceSection('acct_1', createEntry({ addWorkspaceOpen: false }));
    expect(document.getElementById('add-ws-form-acct_1')?.classList.contains('visible')).toBe(false);
  });

  it('renders team loading, error, and populated member states', function() {
    document.body.innerHTML =
      '<div id="team-section-acct_1" class="team-section" data-actor-role="owner">' +
      '<table><tbody id="team-list-acct_1"></tbody></table>' +
      '</div>';

    renderTeamSection('acct_1', createEntry({ teamVisible: true, teamLoading: true }));
    expect(document.getElementById('team-list-acct_1')?.textContent).toContain('Loading');

    renderTeamSection('acct_1', createEntry({ teamVisible: true, teamError: 'Failed to load team.' }));
    expect(document.getElementById('team-list-acct_1')?.textContent).toContain('Failed to load team.');

    renderTeamSection(
      'acct_1',
      createEntry({
        teamVisible: true,
        teamMembers: [{ email: 'owner@example.com', role: 'owner', user_id: 'u1' }],
      })
    );
    expect(document.querySelector('[data-action="change-role"]')).not.toBeNull();
    expect(document.querySelector('[data-action="remove-member"]')).not.toBeNull();
  });

  it('renders account UI for every tracked account entry', function() {
    document.body.innerHTML =
      '<div id="add-ws-form-acct_1" class="add-workspace-form"></div>' +
      '<div id="team-section-acct_1" class="team-section" data-actor-role="admin"><table><tbody id="team-list-acct_1"></tbody></table></div>' +
      '<div id="add-ws-form-acct_2" class="add-workspace-form"></div>' +
      '<div id="team-section-acct_2" class="team-section" data-actor-role="owner"><table><tbody id="team-list-acct_2"></tbody></table></div>';

    var state: PortalAccountState = {
      byAccountID: {
        acct_1: createEntry({ addWorkspaceOpen: true, teamVisible: true }),
        acct_2: createEntry({
          addWorkspaceOpen: false,
          teamVisible: true,
          teamMembers: [{ email: 'tech@example.com', role: 'tech', user_id: 'u2' }],
        }),
      },
    };

    renderAccountUI(state);

    expect(document.getElementById('add-ws-form-acct_1')?.classList.contains('visible')).toBe(true);
    expect(document.getElementById('add-ws-form-acct_2')?.classList.contains('visible')).toBe(false);
    expect(document.getElementById('team-list-acct_2')?.textContent).toContain('tech@example.com');
  });
});
