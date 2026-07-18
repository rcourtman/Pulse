import { expect, test as base, type Page } from "@playwright/test";

import {
  apiRequest,
  ensureSessionAuthenticated,
  getMockMode,
  setMockMode,
} from "./helpers";

type ConnectionFleetConfigDrift = {
  status?: string;
};

type ConnectionFleetRollout = {
  status?: string;
};

type Connection = {
  id: string;
  type?: string;
  name?: string;
  fleet?: {
    configDrift?: ConnectionFleetConfigDrift | null;
    rollout?: ConnectionFleetRollout | null;
  };
};

type ConnectionSystem = {
  id: string;
  components?: Array<{ connectionId?: string }>;
  members?: Array<{ id?: string; name?: string }>;
};

const test = base;

let mockModeWasEnabled: boolean | null = null;

async function ensureMockModeEnabled(page: Page): Promise<void> {
  const state = await getMockMode(page);
  if (mockModeWasEnabled === null) {
    mockModeWasEnabled = state.enabled;
  }
  if (!state.enabled) {
    await setMockMode(page, true);
  }
}

async function readConnections(page: Page): Promise<{
  connections: Connection[];
  systems: ConnectionSystem[];
}> {
  const response = await apiRequest(page, "/api/connections");
  expect(
    response.ok(),
    `Expected /api/connections to succeed, got ${response.status()}`,
  ).toBe(true);
  const payload = (await response.json()) as {
    connections?: Connection[];
    systems?: ConnectionSystem[];
  };
  return {
    connections: payload.connections ?? [],
    systems: payload.systems ?? [],
  };
}

async function assertSettingsInfrastructureDoesNotBlank(
  page: Page,
  durationMs: number,
): Promise<void> {
  const result = await page.evaluate(async (sampleDurationMs) => {
    const failures: Array<{ elapsedMs: number; bodyText: string }> = [];
    const startedAt = performance.now();
    let samples = 0;

    while (performance.now() - startedAt < sampleDurationMs) {
      const root = document.getElementById("root");
      const summary = document.querySelector<HTMLElement>(
        '[aria-label="Connection posture"]',
      );
      const table = document.querySelector("table");
      const bodyText = (document.body?.innerText ?? "")
        .replace(/\s+/g, " ")
        .trim();

      if (
        !root ||
        root.childElementCount === 0 ||
        !summary ||
        !table ||
        bodyText.includes("Config pending") ||
        bodyText.includes("Rollout pending")
      ) {
        failures.push({
          elapsedMs: Math.round(performance.now() - startedAt),
          bodyText: bodyText.slice(0, 160),
        });
      }

      samples += 1;
      await new Promise((resolve) => window.setTimeout(resolve, 250));
    }

    return { failures, samples };
  }, durationMs);

  expect(result.samples).toBeGreaterThan(0);
  expect(
    result.failures,
    "Expected Settings Infrastructure to stay mounted without passive config/rollout warnings",
  ).toEqual([]);
}

test.describe.serial("Settings Infrastructure fleet status coherence", () => {
  test.setTimeout(180_000);

  test.afterAll(async ({ browser }) => {
    if (mockModeWasEnabled === null) return;

    const context = await browser.newContext();
    const page = await context.newPage();
    try {
      const current = await getMockMode(page);
      if (current.enabled !== mockModeWasEnabled) {
        await setMockMode(page, mockModeWasEnabled);
      }
    } finally {
      await context.close();
    }
  });

  test("does not surface default agent config as pending rollout attention", async ({
    page,
  }, testInfo) => {
    test.skip(
      testInfo.project.name.startsWith("mobile-"),
      "Desktop settings runtime proof",
    );

    // getMockMode/apiRequest ride page.request cookies; the token-auth path
    // ensureAuthenticated can take leaves them unauthenticated.
    await ensureSessionAuthenticated(page);
    await ensureMockModeEnabled(page);

    const { connections, systems } = await readConnections(page);
    const agents = connections.filter((connection) => connection.type === "agent");

    expect(agents.length, "Expected mock/runtime data to include agents").toBeGreaterThan(0);
    for (const agent of agents) {
      expect(
        agent.fleet?.configDrift?.status,
        `Default agent ${agent.name ?? agent.id} must not look like a pending config rollout`,
      ).not.toBe("pending");
      expect(
        agent.fleet?.rollout?.status,
        `Default agent ${agent.name ?? agent.id} must not look like a pending rollout`,
      ).not.toBe("pending");
    }

    const systemIDs = new Set<string>();
    for (const system of systems) {
      expect(systemIDs.has(system.id), `Duplicate system id ${system.id}`).toBe(false);
      systemIDs.add(system.id);

      const componentIDs = (system.components ?? [])
        .map((component) => component.connectionId)
        .filter((id): id is string => Boolean(id));
      expect(
        new Set(componentIDs).size,
        `System ${system.id} should not duplicate component connection ids`,
      ).toBe(componentIDs.length);

      const memberKeys = (system.members ?? [])
        .map((member) => (member.name || member.id || "").trim().toLowerCase())
        .filter(Boolean);
      expect(
        new Set(memberKeys).size,
        `System ${system.id} should not duplicate member names`,
      ).toBe(memberKeys.length);
    }

    // The operations sub-route was retired with the legacy aliases; the
    // posture band lives on the consolidated infrastructure workspace.
    await page.goto("/settings/infrastructure", {
      waitUntil: "domcontentloaded",
    });
    await expect(
      page.getByRole("region", { name: "Connection posture" }),
    ).toBeVisible();
    await expect(page.getByText("Connected systems", { exact: true })).toBeVisible();
    await expect(page.getByText("Config pending", { exact: true })).toHaveCount(0);
    await expect(page.getByText("Rollout pending", { exact: true })).toHaveCount(0);
    await assertSettingsInfrastructureDoesNotBlank(page, 4_000);
  });
});
