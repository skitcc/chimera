import assert from "node:assert/strict";

const base = process.env.MOCK_URL ?? "http://127.0.0.1:4010";
const userId = "6b1e2c3a-7d4f-4a1b-9c8e-111111111111";
const trackId = "7c2f3d4b-8e5a-4b2c-ad9f-222222222222";
const graceId = "9e4b5f6d-a07c-4d4e-cf1b-444444444444";
const otherId = "b16d718f-c29e-4f60-e13d-666666666666";

async function call(step, method, path, { body, auth, status }) {
  const response = await fetch(base + path, {
    method,
    headers: {
      accept: "application/json",
      ...(body ? { "content-type": "application/json" } : {}),
      ...(auth ? { authorization: "Bearer test-token" } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
    redirect: "manual",
  });
  const text = await response.text();
  assert.equal(response.status, status, `${step}: ${response.status} ${text}`);
  const json = text ? JSON.parse(text) : null;
  console.log(`${step}: ${response.status}`);
  return json;
}

const session = await call("createSession", "POST", "/v1/sessions", {
  body: { email: "ada@example.com", password: "long-enough" },
  status: 201,
});
assert.equal(typeof session.token, "string");
assert.equal(session.user_id, userId);

const upload = await call("createTrack", "POST", "/v1/tracks", {
  auth: true,
  body: { title: "Night Drive", artist: "Ada", size: 3200000 },
  status: 201,
});
assert.equal(upload.track.id, trackId);
assert.equal(upload.track.status, "pending");
assert.equal(typeof upload.upload_url, "string");

const published = await call("updateTrack", "PATCH", `/v1/tracks/${trackId}`, {
  auth: true,
  body: { status: "ready" },
  status: 200,
});
assert.equal(published.id, trackId);
assert.equal(published.status, "ready");

const mine = await call("listTracks", "GET", `/v1/tracks?user_id=${userId}`, {
  auth: true,
  status: 200,
});
assert.ok(Array.isArray(mine.items));
assert.equal(typeof mine.limit, "number");

const playlists = await call("listPlaylists", "GET", "/v1/playlists", {
  status: 200,
});
assert.ok(Array.isArray(playlists.items));
assert.equal(typeof playlists.limit, "number");

await call("createFollows", "POST", "/v1/follows", {
  auth: true,
  body: { follower_id: userId, following_ids: [graceId, otherId] },
  status: 204,
});

await call("putFollow", "PUT", `/v1/follows/${userId}/${graceId}`, {
  auth: true,
  status: 204,
});

const follows = await call("listFollows", "GET", `/v1/follows?follower_id=${userId}`, {
  status: 200,
});
assert.ok(Array.isArray(follows.items));
assert.equal(typeof follows.limit, "number");

await call(
  "deleteFollows",
  "DELETE",
  `/v1/follows?follower_id=${userId}&following_id=${graceId}&following_id=${otherId}`,
  { auth: true, status: 204 },
);

console.log("scenario ok");
