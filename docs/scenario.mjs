import assert from "node:assert/strict";

const base = process.env.MOCK_URL ?? "http://127.0.0.1:4010";
const token = "test-token";
const trackId = "7c2f3d4b-8e5a-4b2c-ad9f-222222222222";

async function call(step, method, path, { body, auth, status }) {
  const response = await fetch(base + path, {
    method,
    headers: {
      accept: "application/json",
      ...(body ? { "content-type": "application/json" } : {}),
      ...(auth ? { authorization: `Bearer ${token}` } : {}),
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

const session = await call("login", "POST", "/v1/auth/login", {
  body: { email: "ada@example.com", password: "long-enough" },
  status: 200,
});
assert.equal(typeof session.token, "string");
assert.equal(session.user.email, "ada@example.com");

const upload = await call("initTrackUpload", "POST", "/v1/tracks/upload-init", {
  auth: true,
  body: { title: "Night Drive", artist: "Ada", size: 3200000 },
  status: 201,
});
assert.equal(upload.track.status, "pending");
assert.equal(typeof upload.upload_url, "string");

const published = await call(
  "completeTrackUpload",
  "POST",
  `/v1/tracks/${trackId}/upload-complete`,
  { auth: true, status: 200 },
);
assert.equal(published.status, "ready");
assert.equal(published.id, trackId);

const mine = await call("listMyTracks", "GET", "/v1/me/tracks", {
  auth: true,
  status: 200,
});
assert.ok(Array.isArray(mine.items));
assert.equal(typeof mine.limit, "number");

console.log("scenario ok");
