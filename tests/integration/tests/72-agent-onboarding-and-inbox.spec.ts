import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, test as base } from "@playwright/test";
import {
  E2E_CREDENTIALS,
  createAuthenticatedStorageState,
} from "./helpers";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

type WorkerFixtures = {
  authStorageStatePath: string;
};

const test = base.extend<{}, WorkerFixtures>({
  storageState: async ({ authStorageStatePath }, use) => {
    await use(authStorageStatePath);
  },
  authStorageStatePath: [
    async ({ browser }, use, workerInfo) => {
      const storageStatePath = path.resolve(
        __dirname,
        "..",
        "..",
        "tmp",
        "playwright-auth",
        `agent-onboarding-and-inbox-${workerInfo.project.name}.json`,
      );
      fs.mkdirSync(path.dirname(storageStatePath), { recursive: true });
      await createAuthenticatedStorageState(browser, storageStatePath);
      try {
        await use(storageStatePath);
      } finally {
        fs.rmSync(storageStatePath, { force: true });
      }
    },
    { scope: "worker" },
  ],
});

type ManifestCapability = {
  name: string;
  method: string;
  path: string;
  scope?: string;
};

type ManifestSurface = {
  id: string;
  label: string;
  externalAdapter?: boolean;
};

type ManifestSurfaceToolContract = {
  surfaceId: string;
  toolNames?: string[];
};

type AgentCapabilitiesManifest = {
  version: string;
  mcpAdapter?: {
    serverName?: string;
    command?: string;
    configFamilies?: unknown[];
  };
  surfaceContract?: {
    operatorSurfaces?: ManifestSurface[];
  };
  surfaceToolContracts?: ManifestSurfaceToolContract[];
  capabilities?: ManifestCapability[];
};

const EXTERNAL_AGENT_ACTION_ID = "ext-agent-action-1";
const EXTERNAL_AGENT_CREDENTIAL_ID = "tok-ext-99-internal";
const EXTERNAL_AGENT_CREDENTIAL_LABEL = "Claude Code Propose Token";
const EXTERNAL_AGENT_REQUESTER = "agent:claude-code";

const externalAgentApprovedAction = {
  id: EXTERNAL_AGENT_ACTION_ID,
  createdAt: "2026-11-20T00:00:00Z",
  updatedAt: "2026-11-20T00:01:00Z",
  state: "approved",
  decisionRevision: 1,
  origin: {
    surface: "mcp",
  },
  request: {
    requestId: "req-ext-agent-1",
    resourceId: "docker:container:edge-proxy",
    capabilityName: "restart",
    reason: "Restart edge proxy after repeated upstream timeouts",
    requestedBy: EXTERNAL_AGENT_REQUESTER,
    actor: {
      subjectId: `token:${EXTERNAL_AGENT_CREDENTIAL_ID}`,
      kind: "api_token",
      credentialId: EXTERNAL_AGENT_CREDENTIAL_ID,
      credentialLabel: EXTERNAL_AGENT_CREDENTIAL_LABEL,
      orgId: "default",
    },
  },
  resource: {
    id: "docker:container:edge-proxy",
    name: "Edge proxy",
    type: "app-container",
  },
  plan: {
    actionId: EXTERNAL_AGENT_ACTION_ID,
    requestId: "req-ext-agent-1",
    allowed: true,
    requiresApproval: true,
    approvalPolicy: "admin",
    approvalRequirement: {
      version: 1,
      floor: "admin",
      quorum: 1,
      disallowRequester: false,
    },
    rollbackAvailable: false,
    plannedAt: "2026-11-20T00:00:00Z",
    expiresAt: "2099-11-20T00:10:00Z",
    resourceVersion: "resource:sha256:ext-1",
    policyVersion: "policy:sha256:ext-1",
    planHash: "sha256:ext-plan-1",
    policyDecision: {
      version: 1,
      status: "resolved",
      decisionId: "policy-decision:sha256:ext-1",
      actionId: EXTERNAL_AGENT_ACTION_ID,
      scope: {
        orgId: "default",
        resourceId: "docker:container:edge-proxy",
        capabilityName: "restart",
      },
      approvalRequirement: {
        version: 1,
        floor: "admin",
        quorum: 1,
        disallowRequester: false,
      },
      planningAllowed: true,
      requiresApproval: true,
      authorities: [
        {
          kind: "capability_registry",
          sourceId: "capability-registry:restart",
          revision: "policy:sha256:ext-1",
          status: "consulted",
          scope: {
            orgId: "default",
            resourceId: "docker:container:edge-proxy",
            capabilityName: "restart",
          },
          approvalFloor: "admin",
          reasonCodes: [
            "capability_approval_admin",
            "capability_auto_low_risk",
          ],
        },
      ],
    },
  },
  verificationOutcome: { status: "unknown" },
};

function collectPlaintextSecretLeaks(
  node: unknown,
  currentPath = "$",
  leaks: string[] = [],
): string[] {
  if (Array.isArray(node)) {
    node.forEach((item, index) => {
      collectPlaintextSecretLeaks(item, `${currentPath}[${index}]`, leaks);
    });
    return leaks;
  }

  if (!node || typeof node !== "object") {
    return leaks;
  }

  const record = node as Record<string, unknown>;
  const isSecretField =
    "secretRef" in record ||
    "redactedHint" in record ||
    ("configured" in record && "value" in record);

  if (isSecretField) {
    if (typeof record.value === "string" && record.value.trim().length > 0) {
      leaks.push(`${currentPath}.value`);
    }
    if (
      typeof record.secretRef === "string" &&
      record.secretRef.trim().length > 0 &&
      record.secretRef !== "preserved"
    ) {
      leaks.push(`${currentPath}.secretRef=${record.secretRef}`);
    }
    return leaks;
  }

  const secretKeyNames = new Set([
    "password",
    "bearerToken",
    "signingSecret",
    "tokenValue",
    "apiToken",
  ]);

  for (const [key, value] of Object.entries(record)) {
    const childPath = `${currentPath}.${key}`;
    if (
      secretKeyNames.has(key) &&
      typeof value === "string" &&
      value.trim().length > 0
    ) {
      leaks.push(childPath);
      continue;
    }
    collectPlaintextSecretLeaks(value, childPath, leaks);
  }

  return leaks;
}

test.describe("Agent onboarding, Decision Inbox, and API contracts", () => {
  test("shows 3-step pulse CLI onboarding by default, switches install and client tabs, keeps manual config token-free, and highlights only the read-only preset", async ({
    page,
  }) => {
    const manifestResponsePromise = page.waitForResponse(
      (response) =>
        response.url().includes("/api/agent/capabilities") &&
        response.request().method() === "GET",
    );

    await page.goto("/settings/pulse-intelligence/agents", {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(/\/settings\/pulse-intelligence\/agents/, {
      timeout: 15_000,
    });
    await expect(
      page.getByRole("heading", { level: 2, name: "External agents" }),
    ).toBeVisible();

    const manifestResponse = await manifestResponsePromise;
    expect(manifestResponse.ok()).toBeTruthy();

    // The 3-step pulse CLI onboarding is immediately visible without a collapsed wrapper.
    const setupDetails = page.locator("#external-agent-setup-details");
    await expect(setupDetails).toBeVisible();
    await expect(
      page.locator("details #external-agent-setup-details"),
    ).toHaveCount(0);
    await expect(
      setupDetails.getByRole("heading", { level: 3, name: /Install the pulse CLI/ }),
    ).toBeVisible();
    await expect(
      setupDetails.getByRole("heading", { level: 3, name: "Sign in read-only" }),
    ).toBeVisible();
    await expect(
      setupDetails.getByRole("heading", {
        level: 3,
        name: "Connect your agent and verify",
      }),
    ).toBeVisible();

    // Step 1: switch between all CLI install methods.
    const installMethodGroup = setupDetails.getByRole("group", {
      name: "CLI install method",
    });
    const homebrewTab = installMethodGroup.getByRole("button", {
      name: "Homebrew",
      exact: true,
    });
    const unixTab = installMethodGroup.getByRole("button", {
      name: "macOS / Linux (verified)",
      exact: true,
    });
    const windowsTab = installMethodGroup.getByRole("button", {
      name: "Windows (verified)",
      exact: true,
    });

    await expect(homebrewTab).toHaveAttribute("aria-pressed", "true");
    await expect(
      setupDetails.getByText("brew install rcourtman/tap/pulse", {
        exact: true,
      }),
    ).toBeVisible();

    await unixTab.click();
    await expect(unixTab).toHaveAttribute("aria-pressed", "true");
    await expect(homebrewTab).toHaveAttribute("aria-pressed", "false");
    await expect(
      setupDetails.getByText("install-cli.sh", { exact: false }).first(),
    ).toBeVisible();
    await expect(
      setupDetails.getByText("gh release verify-asset", { exact: false }),
    ).toBeVisible();

    await windowsTab.click();
    await expect(windowsTab).toHaveAttribute("aria-pressed", "true");
    await expect(unixTab).toHaveAttribute("aria-pressed", "false");
    await expect(
      setupDetails.getByText("install-cli.ps1", { exact: false }).first(),
    ).toBeVisible();

    await homebrewTab.click();
    await expect(homebrewTab).toHaveAttribute("aria-pressed", "true");
    await expect(
      setupDetails.getByText("brew install rcourtman/tap/pulse", {
        exact: true,
      }),
    ).toBeVisible();

    // Step 2: sign-in command uses pulse auth login with the active instance origin.
    await expect(
      setupDetails.getByText("pulse auth login", { exact: false }).first(),
    ).toBeVisible();

    // Step 3: switch across all supported agent client tabs.
    const clientGroup = setupDetails.getByRole("group", {
      name: "External agent client",
    });
    const clientCases = [
      {
        label: "Claude Code",
        command: "pulse connect claude-code",
        configHint: "~/.claude.json",
      },
      {
        label: "Codex",
        command: "pulse connect codex",
        configHint: "~/.codex/config.toml",
      },
      {
        label: "OpenCode",
        command: "pulse connect opencode",
        configHint: "~/.config/opencode/opencode.json",
      },
      {
        label: "Claude Desktop",
        command: "pulse connect claude-desktop",
        configHint: "claude_desktop_config.json",
      },
    ];

    await expect(
      clientGroup.getByRole("button", { name: "Claude Code", exact: true }),
    ).toHaveAttribute("aria-pressed", "true");

    for (const clientCase of clientCases) {
      const tab = clientGroup.getByRole("button", {
        name: clientCase.label,
        exact: true,
      });
      await tab.click();
      await expect(tab).toHaveAttribute("aria-pressed", "true");
      await expect(
        setupDetails.getByText(clientCase.command, { exact: false }),
      ).toBeVisible();
      await expect(
        setupDetails.getByText("pulse doctor", { exact: false }),
      ).toBeVisible();
      await expect(
        setupDetails.getByText(clientCase.configHint, { exact: false }).first(),
      ).toBeVisible();
    }

    // Manual config expands to show pulse mcp snippets with zero PULSE_API_TOKEN placeholders.
    const manualConfigDetails = setupDetails
      .locator("details")
      .filter({ has: page.locator("summary", { hasText: "Manual config" }) });
    await expect(manualConfigDetails).not.toHaveAttribute("open", "");
    await manualConfigDetails
      .locator("summary")
      .filter({ hasText: "Manual config" })
      .click();
    await expect(manualConfigDetails).toHaveAttribute("open", "");
    await expect(
      manualConfigDetails.getByText("pulse mcp", { exact: false }).first(),
    ).toBeVisible();
    await expect(
      manualConfigDetails.getByRole("heading", {
        level: 4,
        name: /Claude Code & Claude Desktop/,
      }),
    ).toBeVisible();
    await expect(
      manualConfigDetails.getByRole("heading", {
        level: 4,
        name: /Codex.*~\/\.codex\/config\.toml/,
      }),
    ).toBeVisible();
    await expect(
      manualConfigDetails.getByRole("heading", {
        level: 4,
        name: /OpenCode.*opencode\.json/,
      }),
    ).toBeVisible();

    const manualCodeSnippets = (
      await manualConfigDetails.locator("code").allTextContents()
    ).join("\n");
    expect(manualCodeSnippets).toContain('"command": "pulse"');
    expect(manualCodeSnippets).toContain('"mcp"');
    expect(manualCodeSnippets).toContain("[mcp_servers.pulse]");
    expect(manualCodeSnippets).toContain('command = "pulse"');
    expect(manualCodeSnippets).toContain('args = ["mcp"]');
    expect(manualCodeSnippets).toContain("https://opencode.ai/config.json");
    expect(manualCodeSnippets).not.toContain("PULSE_API_TOKEN");
    await expect(manualConfigDetails).not.toContainText("PULSE_API_TOKEN");
    await expect(setupDetails).not.toContainText("PULSE_API_TOKEN");

    // Follow the read-only token preset deep-link and verify only the read-only preset is active.
    const createTokenLink = setupDetails.getByRole("link", {
      name: "Create token",
      exact: true,
    });
    await expect(createTokenLink).toHaveAttribute(
      "href",
      "/settings/security/api?tokenPreset=pulse-intelligence-agent#api-token-create",
    );
    await createTokenLink.click();

    await expect(page).toHaveURL(
      /\/settings\/security\/api\?tokenPreset=pulse-intelligence-agent#api-token-create$/,
    );

    const createCard = page.locator("#api-token-create");
    await expect(
      createCard.getByRole("heading", { level: 2, name: "Create token" }),
    ).toBeVisible();

    const readOnlyPreset = createCard.getByRole("button", {
      name: "External agent (read-only)",
      exact: true,
    });
    const kioskPreset = createCard.getByRole("button", {
      name: "Kiosk / Monitoring",
      exact: true,
    });
    const proposePreset = createCard.getByRole("button", {
      name: "External agent (propose fixes)",
      exact: true,
    });
    const operatorCliPreset = createCard.getByRole("button", {
      name: "Operator CLI",
      exact: true,
    });
    const fullAccessPreset = createCard.getByRole("button", {
      name: "Full access",
      exact: true,
    });

    await expect(readOnlyPreset).toHaveAttribute("aria-pressed", "true");
    await expect(kioskPreset).toHaveAttribute("aria-pressed", "false");
    await expect(proposePreset).toHaveAttribute("aria-pressed", "false");
    await expect(operatorCliPreset).toHaveAttribute("aria-pressed", "false");
    await expect(fullAccessPreset).toHaveAttribute("aria-pressed", "false");

    const pressedVisiblePresets = createCard.locator(
      'button[aria-pressed="true"]:visible',
    );
    await expect(pressedVisiblePresets).toHaveCount(1);
    await expect(pressedVisiblePresets.first()).toHaveText(
      "External agent (read-only)",
    );
    await expect(page.locator("#api-token-name")).toHaveValue(
      "External agent (read-only)",
    );
  });

  test("renders Decision Inbox cleanly on a fresh production build including empty states", async ({
    page,
  }) => {
    await page.goto("/actions", { waitUntil: "domcontentloaded" });
    await expect(
      page.getByRole("heading", { level: 1, name: "Actions" }),
    ).toBeVisible({ timeout: 20_000 });

    const openTab = page.getByRole("tab", { name: "Open", exact: true });
    const historyTab = page.getByRole("tab", { name: "History", exact: true });
    await expect(openTab).toHaveAttribute("aria-selected", "true");
    await expect(historyTab).toBeVisible();

    const openSurface = page
      .getByTestId("actions-calm-state")
      .or(page.getByTestId("actions-queue"));
    await expect(openSurface).toBeVisible({ timeout: 20_000 });
    await expect(
      page.getByText("Actions could not be loaded."),
    ).toHaveCount(0);

    if (await page.getByTestId("actions-calm-state").isVisible()) {
      await expect(
        page.getByRole("heading", {
          level: 2,
          name: "No actions need attention",
        }),
      ).toBeVisible();
    }

    await historyTab.click();
    await expect(page).toHaveURL(/\/actions\/history$/);
    await expect(historyTab).toHaveAttribute("aria-selected", "true");

    const historySurface = page
      .getByTestId("actions-calm-state")
      .or(page.getByTestId("actions-queue"));
    await expect(historySurface).toBeVisible({ timeout: 20_000 });
    await expect(
      page.getByText("Actions could not be loaded."),
    ).toHaveCount(0);

    if (await page.getByTestId("actions-calm-state").isVisible()) {
      await expect(
        page.getByRole("heading", {
          level: 2,
          name: "No action history yet",
        }),
      ).toBeVisible();
    }
  });

  test("shows external-agent attribution and the Pulse Pro execution gate in Decision Inbox", async ({
    page,
  }) => {
    const actionDetailPayload = {
      audit: externalAgentApprovedAction,
      events: [],
      readiness: {
        ready: true,
        code: "ready",
        message: "Action is ready for dispatch.",
        refreshable: false,
        checkedAt: externalAgentApprovedAction.updatedAt,
      },
    };

    await page.route(/\/api\/actions\?/, async (route) => {
      const view = new URL(route.request().url()).searchParams.get("view");
      const actions =
        view === "pending" ? [externalAgentApprovedAction] : [];
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ view, actions, count: actions.length }),
      });
    });

    await page.route(
      new RegExp(`/api/actions/${EXTERNAL_AGENT_ACTION_ID}$`),
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(actionDetailPayload),
        });
      },
    );

    await page.route(
      new RegExp(`/api/actions/${EXTERNAL_AGENT_ACTION_ID}/execute$`),
      async (route) => {
        await route.fulfill({
          status: 402,
          contentType: "application/json",
          body: JSON.stringify({
            error: "license_required",
            code: "license_required",
            feature: "ai_autofix",
            message: "Governed external-agent execution requires Pulse Pro.",
          }),
        });
      },
    );

    await page.goto("/actions", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("actions-queue")).toBeVisible({
      timeout: 20_000,
    });

    const queueRow = page.getByRole("button", {
      name: /Review Restart on docker:container:edge-proxy/,
    });
    await expect(queueRow).toBeVisible();
    await expect(queueRow).toContainText("From MCP");
    await expect(queueRow).toContainText(
      `Requested by ${EXTERNAL_AGENT_CREDENTIAL_LABEL}`,
    );
    await expect(queueRow).not.toContainText(EXTERNAL_AGENT_CREDENTIAL_ID);

    await queueRow.click();

    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText("From MCP");

    const requesterField = dialog.getByTestId("action-plan-requester");
    await expect(requesterField).toBeVisible();
    await expect(requesterField).toContainText(EXTERNAL_AGENT_CREDENTIAL_LABEL);
    await expect(requesterField).toContainText("API token");
    await expect(requesterField).toContainText(EXTERNAL_AGENT_REQUESTER);
    await expect(dialog).not.toContainText(EXTERNAL_AGENT_CREDENTIAL_ID);

    await dialog.getByRole("button", { name: "Run action", exact: true }).click();

    const licenseGateAlert = dialog.getByTestId("action-review-license-gate");
    await expect(licenseGateAlert).toBeVisible();
    await expect(licenseGateAlert).toHaveAttribute("role", "alert");
    await expect(licenseGateAlert).toContainText(
      "Governed external-agent execution requires Pulse Pro.",
    );
  });

  test("advertises MCP adapter, action queue, and declarative config contracts without leaking secrets", async ({
    page,
  }) => {
    const capabilitiesRes = await page.request.get("/api/agent/capabilities");
    expect(capabilitiesRes.status()).toBe(200);

    const manifest =
      (await capabilitiesRes.json()) as AgentCapabilitiesManifest;
    expect(manifest.version).toBe("v1");
    expect(manifest.mcpAdapter?.serverName).toBe("pulse");
    expect(["pulse", "pulse-mcp"]).toContain(manifest.mcpAdapter?.command);
    expect(manifest.mcpAdapter?.configFamilies?.length ?? 0).toBeGreaterThan(0);

    const mcpSurface = manifest.surfaceContract?.operatorSurfaces?.find(
      (surface) => surface.id === "pulse_mcp",
    );
    expect(mcpSurface).toBeDefined();
    expect(mcpSurface?.label).toBe("Pulse MCP");
    expect(mcpSurface?.externalAdapter).toBe(true);

    const expectedTools: Array<{
      name: string;
      method: string;
      path: string;
    }> = [
      { name: "list_actions", method: "GET", path: "/api/actions" },
      { name: "get_action", method: "GET", path: "/api/actions/{actionId}" },
      {
        name: "refresh_action",
        method: "POST",
        path: "/api/actions/{actionId}/refresh",
      },
      {
        name: "get_declarative_config",
        method: "GET",
        path: "/api/config/declarative",
      },
      {
        name: "plan_declarative_config",
        method: "POST",
        path: "/api/config/declarative/plan",
      },
      {
        name: "apply_declarative_config",
        method: "POST",
        path: "/api/config/declarative/apply",
      },
    ];

    const mcpToolContract = manifest.surfaceToolContracts?.find(
      (contract) => contract.surfaceId === "pulse_mcp",
    );
    expect(mcpToolContract).toBeDefined();

    const capabilitiesByName = new Map(
      (manifest.capabilities ?? []).map((cap) => [cap.name, cap]),
    );
    for (const expected of expectedTools) {
      expect(mcpToolContract?.toolNames ?? []).toContain(expected.name);
      const capability = capabilitiesByName.get(expected.name);
      expect(capability).toBeDefined();
      expect(capability?.method).toBe(expected.method);
      expect(capability?.path).toBe(expected.path);
    }

    const declarativeJsonRes = await page.request.get("/api/config/declarative");
    expect(declarativeJsonRes.status()).toBe(200);
    expect(declarativeJsonRes.headers()["content-type"] ?? "").toContain(
      "application/json",
    );

    const declarativeDoc = (await declarativeJsonRes.json()) as Record<
      string,
      unknown
    >;
    expect(declarativeDoc.apiVersion).toBe("pulse.monitoring/v1");
    expect(declarativeDoc.kind).toBe("MonitoringConfig");
    expect(collectPlaintextSecretLeaks(declarativeDoc)).toEqual([]);

    const declarativeJsonText = JSON.stringify(declarativeDoc);
    const sensitiveValues = [
      E2E_CREDENTIALS.password,
      E2E_CREDENTIALS.bootstrapToken,
      E2E_CREDENTIALS.primaryApiToken,
    ].filter((secret) => secret.trim().length >= 8);
    for (const secret of sensitiveValues) {
      expect(declarativeJsonText).not.toContain(secret);
    }

    const declarativeYamlRes = await page.request.get(
      "/api/config/declarative?format=yaml",
    );
    expect(declarativeYamlRes.status()).toBe(200);
    expect(declarativeYamlRes.headers()["content-type"] ?? "").toContain(
      "yaml",
    );

    const yamlText = await declarativeYamlRes.text();
    expect(yamlText).toContain("apiVersion: pulse.monitoring/v1");
    expect(yamlText).toContain("kind: MonitoringConfig");
    expect(yamlText).not.toMatch(
      /^\s*(?:password|bearerToken|signingSecret|tokenValue|apiToken):\s*\S+/m,
    );
    const yamlSecretRefs = Array.from(
      yamlText.matchAll(/^\s*secretRef:\s*(\S+)/gm),
      (match) => match[1],
    );
    for (const ref of yamlSecretRefs) {
      expect(ref).toBe("preserved");
    }
    for (const secret of sensitiveValues) {
      expect(yamlText).not.toContain(secret);
    }
  });
});
