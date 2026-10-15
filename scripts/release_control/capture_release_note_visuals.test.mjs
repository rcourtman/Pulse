import { test } from "node:test";
import assert from "node:assert/strict";
import {
  downloadPrImage,
  verifyPrImage,
} from "./capture_release_note_visuals.mjs";

const image =
  "https://github.com/user-attachments/assets/f3e1870e-7f12-4ab6-8961-cab68e1f1725";
const source = {
  image_url: image,
  pull_request_url: "https://github.com/rcourtman/Pulse/pull/1194",
};
const sha = "a".repeat(40);
const response = (value) =>
  new Response(JSON.stringify(value), {
    headers: { "content-type": "application/json" },
  });

test("binds committed screenshot evidence to the PR source history", async () => {
  const rawImage = `https://raw.githubusercontent.com/rcourtman/Pulse/${sha}/frontend-modern/browser-verification/screenshots/release-highlights-desktop.png`;
  const commands = [];
  await verifyPrImage({ ...source, image_url: rawImage }, "v6.5.0", {
    fetch: async () =>
      response({
        merged: true,
        merge_commit_sha: sha,
        head: { sha },
        body: `![View](${rawImage})`,
      }),
    git: async (...args) => {
      commands.push(args);
      return args[0] === "rev-list" ? sha : "";
    },
  });
  assert.deepEqual(commands[1], ["merge-base", "--is-ancestor", sha, sha]);
  await assert.rejects(
    downloadPrImage(rawImage.replace(sha, "main")),
    /Unsupported/,
  );
});
test("copies a public image through a GitHub storage redirect without repository credentials", async () => {
  const calls = [];
  const result = await downloadPrImage(image, async (url, options) => {
    calls.push({ url, options });
    return calls.length === 1
      ? new Response(null, {
          status: 302,
          headers: {
            location:
              "https://github-production-user-asset-6210df.s3.amazonaws.com/image?signature=example",
          },
        })
      : new Response(new Uint8Array([1, 2, 3]), {
          headers: { "content-type": "image/png" },
        });
  });
  assert.deepEqual([...result.bytes], [1, 2, 3]);
  assert.equal(result.type, "image/png");
  assert.equal(calls.length, 2);
  assert.equal(calls[1].options.headers.Authorization, undefined);
});

test("rejects arbitrary image hosts, redirects to a private network, non-images and oversized images", async () => {
  await assert.rejects(
    downloadPrImage("https://example.com/image.png"),
    /Unsupported/,
  );
  await assert.rejects(
    downloadPrImage(
      image,
      async () =>
        new Response(null, {
          status: 302,
          headers: { location: "http://127.0.0.1/private" },
        }),
    ),
    /left GitHub/,
  );
  await assert.rejects(
    downloadPrImage(
      image,
      async () =>
        new Response("<svg></svg>", {
          headers: { "content-type": "image/svg+xml" },
        }),
    ),
    /PNG, JPEG or WebP/,
  );
  await assert.rejects(
    downloadPrImage(
      image,
      async () =>
        new Response("large", {
          headers: {
            "content-type": "image/png",
            "content-length": String(11 * 1024 * 1024),
          },
        }),
    ),
    /exceeds 10 MiB/,
  );
  await assert.rejects(
    downloadPrImage(
      image,
      async () =>
        new Response(new Uint8Array(10 * 1024 * 1024 + 1), {
          headers: { "content-type": "image/png" },
        }),
    ),
    /exceeds 10 MiB/,
  );
  await assert.rejects(
    downloadPrImage(
      image,
      async () =>
        new Response("", { headers: { "content-type": "image/png" } }),
    ),
    /empty/,
  );
});

test("requires an actual screenshot attachment in a merged PR within the release range", async () => {
  const commands = [];
  const dependencies = {
    fetch: async () =>
      response({
        merged: true,
        merge_commit_sha: sha,
        body: `![Now](${image})`,
      }),
    git: async (...args) => {
      commands.push(args);
      return args[0] === "rev-list" ? sha : "";
    },
  };
  await verifyPrImage(source, "v6.5.0", dependencies);
  assert.deepEqual(commands[0], ["merge-base", "--is-ancestor", sha, "HEAD"]);
  assert.deepEqual(commands[1], ["rev-list", "v6.5.0..HEAD"]);
  await assert.rejects(
    verifyPrImage(source, "v6.5.0", { ...dependencies, git: async () => "" }),
    /outside the release range/,
  );
  await assert.rejects(
    verifyPrImage(source, "v6.5.0", {
      ...dependencies,
      fetch: async () => response({ merged: false }),
    }),
    /already be merged/,
  );
  await assert.rejects(
    verifyPrImage(source, "v6.5.0", {
      ...dependencies,
      fetch: async (url) =>
        response(
          url.includes("/pulls/")
            ? { merged: true, merge_commit_sha: sha, body: "" }
            : [],
        ),
    }),
    /absent from the source PR/,
  );
});

test("finds evidence in PR discussion beyond the first page", async () => {
  const urls = [];
  await verifyPrImage(source, "v6.5.0", {
    git: async () => sha,
    fetch: async (url) => {
      urls.push(url);
      if (url.includes("/pulls/"))
        return response({ merged: true, merge_commit_sha: sha, body: "" });
      return response(
        url.endsWith("page=1")
          ? Array.from({ length: 100 }, () => ({ body: "Discussion" }))
          : [{ body: `<img src="${image}">` }],
      );
    },
  });
  assert.equal(urls.length, 3);
});
