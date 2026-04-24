import test from "node:test";
import assert from "node:assert/strict";

import { buildAuthHeaders } from "./api.ts";

test("buildAuthHeaders returns oauth token header for oauth users", () => {
  const headers = buildAuthHeaders({
    address: "oauth:google:abc",
    joinedAt: Date.now(),
    level: 0,
    authMethod: "oauth",
    oauthToken: "abc-token",
  });

  assert.deepEqual(headers, {
    "X-Mock-OAuth-Token": "abc-token",
    "X-Mock-Trust-Tier": "0",
  });
});

test("buildAuthHeaders returns did header for passkey users", () => {
  const headers = buildAuthHeaders({
    address: "did:vflow:abc123",
    joinedAt: Date.now(),
    level: 1,
    authMethod: "passkey",
  });

  assert.deepEqual(headers, {
    "X-Mock-DID": "did:vflow:abc123",
    "X-Mock-Trust-Tier": "1",
  });
});

test("buildAuthHeaders returns empty object for anonymous state", () => {
  assert.deepEqual(buildAuthHeaders(null), {});
});

test("buildAuthHeaders uses did header when oauth token is missing", () => {
  const headers = buildAuthHeaders({
    address: "did:vflow:fallback",
    joinedAt: Date.now(),
    level: 1,
    authMethod: "oauth",
  });

  assert.deepEqual(headers, {
    "X-Mock-DID": "did:vflow:fallback",
    "X-Mock-Trust-Tier": "1",
  });
});
