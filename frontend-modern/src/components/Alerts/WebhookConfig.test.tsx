import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';

// --- Mock setup (must be before component import) ---
const getWebhookTemplatesMock = vi.fn();

vi.mock('@/api/notifications', () => ({
  NotificationsAPI: {
    getWebhookTemplates: (...args: unknown[]) => getWebhookTemplatesMock(...args),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    debug: vi.fn(),
    error: vi.fn(),
    warn: vi.fn(),
  },
}));

import { WebhookConfig } from './WebhookConfig';
import { logger } from '@/utils/logger';
import type { Webhook } from '@/api/notifications';

// --- Helpers ---

function makeWebhook(overrides: Partial<Webhook> = {}): Webhook {
  return {
    id: 'wh-1',
    name: 'Test Webhook',
    url: 'https://example.com/hook',
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    enabled: true,
    service: 'generic',
    ...overrides,
  };
}

const discordTemplate = {
  service: 'discord',
  name: 'Discord Webhook',
  urlPattern: 'https://discord.com/api/webhooks/...',
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  payloadTemplate: '',
  instructions: 'Go to Server Settings → Integrations → Webhooks → New Webhook.',
};

const slackTemplate = {
  service: 'slack',
  name: 'Slack Webhook',
  urlPattern: 'https://hooks.slack.com/services/...',
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  payloadTemplate: '',
  instructions: 'Create an incoming webhook in your Slack workspace.',
};

const genericTemplate = {
  service: 'generic',
  name: 'Generic Webhook',
  urlPattern: 'https://example.com/webhook',
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  payloadTemplate: '',
  instructions: '',
};

const mockTemplates = [genericTemplate, discordTemplate, slackTemplate];

// --- Tests ---
describe('WebhookConfig', () => {
  let onAddMock: ReturnType<typeof vi.fn>;
  let onUpdateMock: ReturnType<typeof vi.fn>;
  let onDeleteMock: ReturnType<typeof vi.fn>;
  let onTestMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    onAddMock = vi.fn();
    onUpdateMock = vi.fn();
    onDeleteMock = vi.fn();
    onTestMock = vi.fn();
    getWebhookTemplatesMock.mockReset();
    getWebhookTemplatesMock.mockResolvedValue(mockTemplates);
    vi.mocked(logger.debug).mockReset();
    vi.mocked(logger.error).mockReset();
    vi.mocked(logger.warn).mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  // --- Rendering existing webhooks ---

  it('renders the "Add Webhook" button when no form is open', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    expect(screen.getByText('+ Add Webhook')).toBeInTheDocument();
  });

  it('renders webhook list with names, services, methods, and URLs', () => {
    const webhooks = [
      makeWebhook({
        id: 'wh-1',
        name: 'Discord Alert',
        service: 'discord',
        method: 'POST',
        url: 'https://discord.com/api/webhooks/123',
      }),
      makeWebhook({
        id: 'wh-2',
        name: 'Slack Alert',
        service: 'slack',
        method: 'PUT',
        url: 'https://hooks.slack.com/services/abc',
      }),
    ];

    render(() => (
      <WebhookConfig
        webhooks={webhooks}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    expect(screen.getByText('Discord Alert')).toBeInTheDocument();
    expect(screen.getByText('Slack Alert')).toBeInTheDocument();
    expect(screen.getByText('Discord')).toBeInTheDocument();
    expect(screen.getByText('Slack')).toBeInTheDocument();
    expect(screen.getByText('POST')).toBeInTheDocument();
    expect(screen.getByText('PUT')).toBeInTheDocument();
    expect(screen.getByText('https://discord.com/api/webhooks/123')).toBeInTheDocument();
    expect(screen.getByText('https://hooks.slack.com/services/abc')).toBeInTheDocument();
  });

  it('shows enabled count summary', () => {
    const webhooks = [
      makeWebhook({ id: 'wh-1', enabled: true }),
      makeWebhook({ id: 'wh-2', enabled: false }),
      makeWebhook({ id: 'wh-3', enabled: true }),
    ];

    render(() => (
      <WebhookConfig
        webhooks={webhooks}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    expect(screen.getByText(/2 of 3 webhooks/)).toBeInTheDocument();
  });

  it('shows "Enabled" for enabled webhooks and "Disabled" for disabled ones', () => {
    const webhooks = [
      makeWebhook({ id: 'wh-1', name: 'Active', enabled: true }),
      makeWebhook({ id: 'wh-2', name: 'Inactive', enabled: false }),
    ];

    render(() => (
      <WebhookConfig
        webhooks={webhooks}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    expect(screen.getByText('Enabled')).toBeInTheDocument();
    expect(screen.getByText('Disabled')).toBeInTheDocument();
  });

  it('falls back to "Generic" for webhooks with no service', () => {
    const webhooks = [makeWebhook({ id: 'wh-1', service: undefined })];

    render(() => (
      <WebhookConfig
        webhooks={webhooks}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    expect(screen.getByText('Generic')).toBeInTheDocument();
  });

  it('does not display internal webhook ID in the card', () => {
    const webhooks = [makeWebhook({ id: 'wh-1', name: 'Test Hook' })];

    render(() => (
      <WebhookConfig
        webhooks={webhooks}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    expect(screen.queryByText('ID: wh-1')).not.toBeInTheDocument();
  });

  // --- Enable/Disable toggles ---

  it('toggles individual webhook enabled state', () => {
    const webhook = makeWebhook({ id: 'wh-1', enabled: true });

    render(() => (
      <WebhookConfig
        webhooks={[webhook]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Enabled'));

    expect(onUpdateMock).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'wh-1', enabled: false }),
    );
  });

  it('"Enable All" calls onUpdate for each webhook with enabled=true', () => {
    const webhooks = [
      makeWebhook({ id: 'wh-1', enabled: false }),
      makeWebhook({ id: 'wh-2', enabled: false }),
    ];

    render(() => (
      <WebhookConfig
        webhooks={webhooks}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Enable All'));

    expect(onUpdateMock).toHaveBeenCalledTimes(2);
    expect(onUpdateMock).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'wh-1', enabled: true }),
    );
    expect(onUpdateMock).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'wh-2', enabled: true }),
    );
  });

  it('"Disable All" calls onUpdate for each webhook with enabled=false', () => {
    const webhooks = [
      makeWebhook({ id: 'wh-1', enabled: true }),
      makeWebhook({ id: 'wh-2', enabled: true }),
    ];

    render(() => (
      <WebhookConfig
        webhooks={webhooks}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Disable All'));

    expect(onUpdateMock).toHaveBeenCalledTimes(2);
    expect(onUpdateMock).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'wh-1', enabled: false }),
    );
    expect(onUpdateMock).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'wh-2', enabled: false }),
    );
  });

  it('"Enable All" is disabled when all webhooks are already enabled', () => {
    const webhooks = [
      makeWebhook({ id: 'wh-1', enabled: true }),
      makeWebhook({ id: 'wh-2', enabled: true }),
    ];

    render(() => (
      <WebhookConfig
        webhooks={webhooks}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    const enableAllBtn = screen.getByText('Enable All');
    expect(enableAllBtn).toBeDisabled();
  });

  it('"Disable All" is disabled when no webhooks are enabled', () => {
    const webhooks = [
      makeWebhook({ id: 'wh-1', enabled: false }),
      makeWebhook({ id: 'wh-2', enabled: false }),
    ];

    render(() => (
      <WebhookConfig
        webhooks={webhooks}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    const disableAllBtn = screen.getByText('Disable All');
    expect(disableAllBtn).toBeDisabled();
  });

  // --- Delete ---

  it('calls onDelete when Delete button is clicked', () => {
    const webhooks = [makeWebhook({ id: 'wh-1' })];

    render(() => (
      <WebhookConfig
        webhooks={webhooks}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Delete'));

    expect(onDeleteMock).toHaveBeenCalledWith('wh-1');
  });

  // --- Test button on existing webhook ---

  it('calls onTest with webhook id when Test button is clicked', () => {
    const webhooks = [makeWebhook({ id: 'wh-1', enabled: true })];

    render(() => (
      <WebhookConfig
        webhooks={webhooks}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Test'));

    expect(onTestMock).toHaveBeenCalledWith('wh-1');
  });

  it('disables Test button when webhook is disabled', () => {
    const webhooks = [makeWebhook({ id: 'wh-1', enabled: false })];

    render(() => (
      <WebhookConfig
        webhooks={webhooks}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    const testBtn = screen.getByText('Test');
    expect(testBtn).toBeDisabled();
  });

  it('shows "Testing…" and disables button when testing matches webhook id', () => {
    const webhooks = [makeWebhook({ id: 'wh-1', enabled: true })];

    render(() => (
      <WebhookConfig
        webhooks={webhooks}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
        testing="wh-1"
      />
    ));

    const testBtn = screen.getByText('Testing…');
    expect(testBtn).toBeDisabled();
  });

  // --- Add webhook flow ---

  it('opens add form when "+ Add Webhook" is clicked', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    // Form elements should appear
    expect(screen.getByText('Service Type')).toBeInTheDocument();
    expect(screen.getByText('Name')).toBeInTheDocument();
    expect(screen.getByText('Webhook URL')).toBeInTheDocument();
    expect(screen.getByText('HTTP method')).toBeInTheDocument();
  });

  it('hides "+ Add Webhook" button when form is open', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    expect(screen.queryByText('+ Add Webhook')).not.toBeInTheDocument();
  });

  it('initializes with default Content-Type header when opening add form', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    // Check that the Content-Type header input is present
    const inputs = document.querySelectorAll('input[type="text"]');
    const headerKeyInput = Array.from(inputs).find(
      (el) => (el as HTMLInputElement).value === 'Content-Type',
    ) as HTMLInputElement;
    expect(headerKeyInput).toBeTruthy();
  });

  it('calls onAdd with form data when saving a new webhook', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    // Fill in name
    const nameInput = screen.getByPlaceholderText('My Webhook');
    fireEvent.input(nameInput, { target: { value: 'My New Webhook' } });

    // Fill in URL
    const urlInput = screen.getByPlaceholderText('https://example.com/webhook');
    fireEvent.input(urlInput, { target: { value: 'https://hooks.example.com/notify' } });

    // Save
    fireEvent.click(screen.getByText('Add Webhook'));

    expect(onAddMock).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'My New Webhook',
        url: 'https://hooks.example.com/notify',
        method: 'POST',
        enabled: true,
        service: 'generic',
      }),
    );
  });

  it('does not call onAdd when name or url is missing', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    // The "Add Webhook" save button should be disabled since name and url are empty
    const saveBtn = screen.getByText('Add Webhook');
    expect(saveBtn).toBeDisabled();

    // Click anyway to verify onAdd is never called
    fireEvent.click(saveBtn);
    expect(onAddMock).not.toHaveBeenCalled();
  });

  it('resets form and closes panel after adding a webhook', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    const nameInput = screen.getByPlaceholderText('My Webhook');
    fireEvent.input(nameInput, { target: { value: 'Webhook X' } });

    const urlInput = screen.getByPlaceholderText('https://example.com/webhook');
    fireEvent.input(urlInput, { target: { value: 'https://example.com' } });

    fireEvent.click(screen.getByText('Add Webhook'));

    // Form should close, "+ Add Webhook" should reappear
    expect(screen.getByText('+ Add Webhook')).toBeInTheDocument();
    expect(screen.queryByText('Service Type')).not.toBeInTheDocument();
  });

  // --- Cancel form ---

  it('closes form when Cancel is clicked', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));
    expect(screen.getByText('Service Type')).toBeInTheDocument();

    fireEvent.click(screen.getByText('Cancel'));

    // Form should close
    expect(screen.queryByText('Service Type')).not.toBeInTheDocument();
    expect(screen.getByText('+ Add Webhook')).toBeInTheDocument();
  });

  // --- Edit webhook flow ---

  it('populates form with webhook data when Edit is clicked', () => {
    const webhook = makeWebhook({
      id: 'wh-edit',
      name: 'Edit Me',
      url: 'https://edit.example.com',
      method: 'PUT',
      service: 'discord',
      headers: { Authorization: 'Bearer token123' },
    });

    render(() => (
      <WebhookConfig
        webhooks={[webhook]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Edit'));

    // Check form is populated
    const nameInput = document.querySelector('input[type="text"]') as HTMLInputElement;
    expect(nameInput.value).toBe('Edit Me');

    const urlInput = document.querySelector('input[type="url"]') as HTMLInputElement;
    expect(urlInput.value).toBe('https://edit.example.com');
  });

  it('calls onUpdate with updated data when saving an edited webhook', () => {
    const webhook = makeWebhook({
      id: 'wh-edit',
      name: 'Original',
      url: 'https://original.example.com',
      headers: { 'Content-Type': 'application/json' },
    });

    render(() => (
      <WebhookConfig
        webhooks={[webhook]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Edit'));

    // Change name
    const nameInput = document.querySelector('input[type="text"]') as HTMLInputElement;
    fireEvent.input(nameInput, { target: { value: 'Updated Name' } });

    // Save
    fireEvent.click(screen.getByText('Update Webhook'));

    expect(onUpdateMock).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 'wh-edit',
        name: 'Updated Name',
      }),
    );
    // Should not call onAdd
    expect(onAddMock).not.toHaveBeenCalled();
  });

  it('shows "Update Webhook" button label when editing', () => {
    const webhook = makeWebhook({ id: 'wh-edit', name: 'Editable' });

    render(() => (
      <WebhookConfig
        webhooks={[webhook]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Edit'));

    expect(screen.getByText('Update Webhook')).toBeInTheDocument();
  });

  // --- Service selection ---

  it('toggles service dropdown when service type button is clicked', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    // Default service is Generic
    const serviceBtn = screen.getByText(/Generic →/);
    fireEvent.click(serviceBtn);

    // Service options should appear
    expect(screen.getByText('Discord')).toBeInTheDocument();
    expect(screen.getByText('Slack')).toBeInTheDocument();
    expect(screen.getByText('Telegram')).toBeInTheDocument();
    expect(screen.getByText('Microsoft Teams')).toBeInTheDocument();
    expect(screen.getByText('PagerDuty')).toBeInTheDocument();
    expect(screen.getByText('Pushover')).toBeInTheDocument();
    expect(screen.getByText('Gotify')).toBeInTheDocument();
  });

  it('applies template settings when selecting a service', async () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    // Wait for templates to load by checking a UI effect: the name placeholder
    // changes from fallback "My Webhook" to the generic template name
    await waitFor(() => {
      expect(screen.getByPlaceholderText('Generic Webhook')).toBeInTheDocument();
    });

    // Open service dropdown
    const serviceBtn = screen.getByText(/Generic →/);
    fireEvent.click(serviceBtn);

    // Select Discord
    const discordBtn = screen.getByText('Discord server webhook').closest('button')!;
    fireEvent.click(discordBtn);

    // Dropdown should close
    expect(screen.queryByText('Discord server webhook')).not.toBeInTheDocument();

    // Service should be updated in the button
    expect(screen.getByText(/Discord →/)).toBeInTheDocument();

    // Verify template method is applied — the select should now show POST (Discord template method)
    const methodSelect = screen.getByRole('combobox') as HTMLSelectElement;
    expect(methodSelect.value).toBe('POST');

    // Verify template name was applied to form since name was empty
    const nameField = screen.getByPlaceholderText('Discord Webhook') as HTMLInputElement;
    expect(nameField.value).toBe('Discord Webhook');
  });

  it('shows setup instructions when a template with instructions is selected', async () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    // Wait for templates to be in state
    await waitFor(() => {
      expect(screen.getByPlaceholderText('Generic Webhook')).toBeInTheDocument();
    });

    const serviceBtn = screen.getByText(/Generic →/);
    fireEvent.click(serviceBtn);

    const discordBtn = screen.getByText('Discord server webhook').closest('button')!;
    fireEvent.click(discordBtn);

    expect(screen.getByText('Setup Instructions')).toBeInTheDocument();
    expect(screen.getByText(/Integrations → Webhooks/)).toBeInTheDocument();
  });

  // --- Mention field visibility ---

  it('shows mention field for discord service', async () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    // Wait for templates to be in state
    await waitFor(() => {
      expect(screen.getByPlaceholderText('Generic Webhook')).toBeInTheDocument();
    });

    // Open service dropdown and select Discord
    const serviceBtn = screen.getByText(/Generic →/);
    fireEvent.click(serviceBtn);

    const discordBtn = screen.getByText('Discord server webhook').closest('button')!;
    fireEvent.click(discordBtn);

    expect(screen.getByText('Mention')).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/@everyone or/)).toBeInTheDocument();
  });

  it('does not show mention field for generic service', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    // Generic is default, should not show mention
    expect(screen.queryByText('Mention')).not.toBeInTheDocument();
  });

  // --- Custom payload template ---

  it('shows payload template textarea for generic service', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    expect(screen.getByText(/Custom payload template/)).toBeInTheDocument();
  });

  it('hides payload template when non-generic service is selected', async () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    // Wait for templates to be in state
    await waitFor(() => {
      expect(screen.getByPlaceholderText('Generic Webhook')).toBeInTheDocument();
    });

    // Initially shows for generic
    expect(screen.getByText(/Custom payload template/)).toBeInTheDocument();

    // Switch to Discord
    const serviceBtn = screen.getByText(/Generic →/);
    fireEvent.click(serviceBtn);

    const discordBtn = screen.getByText('Discord server webhook').closest('button')!;
    fireEvent.click(discordBtn);

    // Should be hidden now
    expect(screen.queryByText(/Custom payload template/)).not.toBeInTheDocument();
  });

  // --- Header management ---

  it('adds a custom header when "+ Add header" is clicked', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    // Initially has one header (Content-Type)
    const removeButtons = screen.getAllByText('Remove');
    const initialCount = removeButtons.length;

    fireEvent.click(screen.getByText('+ Add header'));

    // Should have one more Remove button
    expect(screen.getAllByText('Remove').length).toBe(initialCount + 1);
  });

  it('removes a header when Remove is clicked', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    // Add an extra header so there are two Remove buttons
    fireEvent.click(screen.getByText('+ Add header'));
    const removeButtons = screen.getAllByText('Remove');
    expect(removeButtons.length).toBe(2);

    // Click Remove on the first header
    fireEvent.click(removeButtons[0]);

    // Should have one fewer Remove button
    expect(screen.getAllByText('Remove').length).toBe(1);
  });

  // --- HTTP method ---

  it('allows changing HTTP method', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    const methodSelect = screen.getByRole('combobox') as HTMLSelectElement;
    expect(methodSelect.value).toBe('POST');

    fireEvent.change(methodSelect, { target: { value: 'PUT' } });
    expect(methodSelect.value).toBe('PUT');
  });

  // --- Enable checkbox ---

  it('toggles enabled checkbox in the form', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    const enableCheckbox = screen.getByRole('checkbox') as HTMLInputElement;
    expect(enableCheckbox.checked).toBe(true);

    fireEvent.change(enableCheckbox, { target: { checked: false } });
    expect(enableCheckbox.checked).toBe(false);
  });

  // --- Test from form ---

  it('shows Test button in form only when name and url are filled', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    // No Test button initially (name and url empty)
    // The save button says "Add Webhook" and there should be no "Test" button (form test button)
    const buttons = screen.getAllByRole('button');
    const testBtns = buttons.filter((b) => b.textContent === 'Test');
    expect(testBtns.length).toBe(0);

    // Fill name and url
    const nameInput = screen.getByPlaceholderText('My Webhook');
    fireEvent.input(nameInput, { target: { value: 'Test WH' } });

    const urlInput = screen.getByPlaceholderText('https://example.com/webhook');
    fireEvent.input(urlInput, { target: { value: 'https://test.com' } });

    // Now the Test button should appear
    const formTestBtn = screen.getByText('Test');
    expect(formTestBtn).toBeInTheDocument();
  });

  it('calls onTest with temp id and form data when form Test button is clicked', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    const nameInput = screen.getByPlaceholderText('My Webhook');
    fireEvent.input(nameInput, { target: { value: 'Form Test' } });

    const urlInput = screen.getByPlaceholderText('https://example.com/webhook');
    fireEvent.input(urlInput, { target: { value: 'https://form-test.com' } });

    fireEvent.click(screen.getByText('Test'));

    expect(onTestMock).toHaveBeenCalledWith(
      'temp-new-webhook',
      expect.objectContaining({
        name: 'Form Test',
        url: 'https://form-test.com',
      }),
    );
  });

  it('shows "Testing..." in form when testing matches temp id', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
        testing="temp-new-webhook"
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    const nameInput = screen.getByPlaceholderText('My Webhook');
    fireEvent.input(nameInput, { target: { value: 'Form Test' } });

    const urlInput = screen.getByPlaceholderText('https://example.com/webhook');
    fireEvent.input(urlInput, { target: { value: 'https://form-test.com' } });

    expect(screen.getByText('Testing...')).toBeInTheDocument();
  });

  // --- Template loading error ---

  it('handles template loading error gracefully', async () => {
    getWebhookTemplatesMock.mockRejectedValue(new Error('Network error'));
    const { logger } = await import('@/utils/logger');

    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    await waitFor(() => {
      expect(logger.error).toHaveBeenCalledWith(
        'Failed to load webhook templates:',
        expect.any(Error),
      );
    });

    // Component should still render and work
    expect(screen.getByText('+ Add Webhook')).toBeInTheDocument();
  });

  // --- Service name mapping ---

  it('displays correct service names for all known services', () => {
    const services = [
      { service: 'discord', expected: 'Discord' },
      { service: 'slack', expected: 'Slack' },
      { service: 'teams', expected: 'Microsoft Teams' },
      { service: 'pagerduty', expected: 'PagerDuty' },
      { service: 'telegram', expected: 'Telegram' },
      { service: 'ntfy', expected: 'ntfy' },
    ];

    for (const { service, expected } of services) {
      cleanup();
      const webhooks = [makeWebhook({ id: `wh-${service}`, service, name: `${expected} Hook` })];
      render(() => (
        <WebhookConfig
          webhooks={webhooks}
          onAdd={onAddMock}
          onUpdate={onUpdateMock}
          onDelete={onDeleteMock}
          onTest={onTestMock}
        />
      ));
      expect(screen.getByText(expected)).toBeInTheDocument();
    }
  });

  it('falls back to raw service string for unknown services', () => {
    const webhooks = [makeWebhook({ id: 'wh-custom', service: 'custom-svc' })];

    render(() => (
      <WebhookConfig
        webhooks={webhooks}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    expect(screen.getByText('custom-svc')).toBeInTheDocument();
  });

  // --- Custom fields for pushover ---

  it('shows preset custom fields when editing a pushover webhook', () => {
    const webhook = makeWebhook({
      id: 'wh-push',
      service: 'pushover',
      customFields: { app_token: 'my-app-token', user_token: 'my-user-key' },
    });

    render(() => (
      <WebhookConfig
        webhooks={[webhook]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Edit'));

    // Preset labels should be shown
    expect(screen.getByText('Application Token')).toBeInTheDocument();
    expect(screen.getByText('User Key')).toBeInTheDocument();

    // Values should be populated
    const inputs = document.querySelectorAll('input[type="text"]');
    const tokenInput = Array.from(inputs).find(
      (el) => (el as HTMLInputElement).value === 'my-app-token',
    );
    expect(tokenInput).toBeTruthy();
  });

  // --- Edge case: webhook with no id for delete/test ---

  it('does not call onDelete when webhook has no id', () => {
    const webhook = makeWebhook({ id: '' });

    render(() => (
      <WebhookConfig
        webhooks={[webhook]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Delete'));

    // The handler checks `webhook.id && props.onDelete(webhook.id)` — empty string is falsy
    expect(onDeleteMock).not.toHaveBeenCalled();
  });

  it('does not call onTest when webhook has no id', () => {
    const webhook = makeWebhook({ id: '' });

    render(() => (
      <WebhookConfig
        webhooks={[webhook]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Test'));

    expect(onTestMock).not.toHaveBeenCalled();
  });

  // --- Headers in saved webhook ---

  it('includes headers from form inputs when saving a new webhook', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    // Fill name and url
    const nameInput = screen.getByPlaceholderText('My Webhook');
    fireEvent.input(nameInput, { target: { value: 'Header Test' } });

    const urlInput = screen.getByPlaceholderText('https://example.com/webhook');
    fireEvent.input(urlInput, { target: { value: 'https://test.com' } });

    // Add a new header
    fireEvent.click(screen.getByText('+ Add header'));

    // Find the new empty header inputs (the ones with placeholder "Header name" / "Header value")
    const headerNameInputs = screen.getAllByPlaceholderText('Header name');
    const headerValueInputs = screen.getAllByPlaceholderText('Header value');

    // Fill the last (new) header
    const lastNameInput = headerNameInputs[headerNameInputs.length - 1];
    const lastValueInput = headerValueInputs[headerValueInputs.length - 1];

    fireEvent.input(lastNameInput, { target: { value: 'X-API-Key' } });
    fireEvent.input(lastValueInput, { target: { value: 'secret123' } });

    // Save
    fireEvent.click(screen.getByText('Add Webhook'));

    expect(onAddMock).toHaveBeenCalledWith(
      expect.objectContaining({
        headers: expect.objectContaining({
          'Content-Type': 'application/json',
          'X-API-Key': 'secret123',
        }),
      }),
    );
  });

  // --- No webhooks scenario ---

  it('does not render quick actions bar when there are no webhooks', () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    expect(screen.queryByText('Enable All')).not.toBeInTheDocument();
    expect(screen.queryByText('Disable All')).not.toBeInTheDocument();
  });

  // --- Mention and customFields in saved payloads ---

  it('includes mention in onAdd payload when set', async () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    // Wait for templates to be in state
    await waitFor(() => {
      expect(screen.getByPlaceholderText('Generic Webhook')).toBeInTheDocument();
    });

    // Switch to Discord to show mention field
    const serviceBtn = screen.getByText(/Generic →/);
    fireEvent.click(serviceBtn);

    const discordBtn = screen.getByText('Discord server webhook').closest('button')!;
    fireEvent.click(discordBtn);

    // Fill required fields — use the URL input via placeholder
    const urlInput = screen.getByPlaceholderText('https://discord.com/api/webhooks/...');
    fireEvent.input(urlInput, { target: { value: 'https://discord.com/api/webhooks/123/abc' } });

    // Fill mention
    const mentionInput = screen.getByPlaceholderText(/@everyone or/);
    fireEvent.input(mentionInput, { target: { value: '@everyone' } });

    // Save
    fireEvent.click(screen.getByText('Add Webhook'));

    expect(onAddMock).toHaveBeenCalledWith(
      expect.objectContaining({
        mention: '@everyone',
        service: 'discord',
      }),
    );
  });

  it('includes customFields in onUpdate payload', () => {
    const webhook = makeWebhook({
      id: 'wh-push',
      name: 'Pushover Alert',
      service: 'pushover',
      customFields: { app_token: 'old-token', user_token: 'old-user' },
    });

    render(() => (
      <WebhookConfig
        webhooks={[webhook]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Edit'));

    // Save without changes — customFields should still be present
    fireEvent.click(screen.getByText('Update Webhook'));

    expect(onUpdateMock).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 'wh-push',
        customFields: expect.objectContaining({
          app_token: 'old-token',
          user_token: 'old-user',
        }),
      }),
    );
  });

  // --- Form Test button uses editingId when editing ---

  it('uses editingId for form test when editing an existing webhook', () => {
    const webhook = makeWebhook({
      id: 'wh-edit-test',
      name: 'Edit Test WH',
      url: 'https://edit-test.com',
    });

    render(() => (
      <WebhookConfig
        webhooks={[webhook]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('Edit'));

    // There are multiple Test buttons: one on the webhook card and one in the form.
    // The form Test button is the last one since the form renders after the card.
    const testButtons = screen.getAllByText('Test');
    const formTestBtn = testButtons[testButtons.length - 1];
    fireEvent.click(formTestBtn);

    expect(onTestMock).toHaveBeenCalledWith(
      'wh-edit-test',
      expect.objectContaining({
        name: 'Edit Test WH',
        url: 'https://edit-test.com',
      }),
    );
  });

  // --- Payload template clearing when switching services ---

  it('clears payload template when switching from generic to non-generic service', async () => {
    render(() => (
      <WebhookConfig
        webhooks={[]}
        onAdd={onAddMock}
        onUpdate={onUpdateMock}
        onDelete={onDeleteMock}
        onTest={onTestMock}
      />
    ));

    fireEvent.click(screen.getByText('+ Add Webhook'));

    // Wait for templates to be in state
    await waitFor(() => {
      expect(screen.getByPlaceholderText('Generic Webhook')).toBeInTheDocument();
    });

    // Fill in a custom payload template while on generic
    const payloadTextarea = document.querySelector('textarea') as HTMLTextAreaElement;
    fireEvent.input(payloadTextarea, {
      target: { value: '{"custom": "{{.Message}}"}' },
    });

    // Fill name and url
    const nameInput = screen.getByPlaceholderText('Generic Webhook');
    fireEvent.input(nameInput, { target: { value: 'WH with payload' } });

    const urlInput = screen.getByPlaceholderText('https://example.com/webhook');
    fireEvent.input(urlInput, { target: { value: 'https://test.com/hook' } });

    // Switch to Discord
    const serviceBtn = screen.getByText(/Generic →/);
    fireEvent.click(serviceBtn);

    const discordBtn = screen.getByText('Discord server webhook').closest('button')!;
    fireEvent.click(discordBtn);

    // Switch back to generic
    const serviceBtn2 = screen.getByText(/Discord →/);
    fireEvent.click(serviceBtn2);

    const genericBtn = screen.getByText('Custom webhook endpoint').closest('button')!;
    fireEvent.click(genericBtn);

    // The payload template should have been cleared when we left generic
    const payloadTextareaAfter = document.querySelector('textarea') as HTMLTextAreaElement;
    expect(payloadTextareaAfter.value).toBe('');
  });
});
