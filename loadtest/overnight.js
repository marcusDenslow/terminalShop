import http from 'k6/http';

const BASE = __ENV.BASE_URL || 'https://api.sshops.uk';
const SECRET = __ENV.CLIENT_SECRET || '';

// Per-VU JWT cache. JWTs are valid for 30 minutes; we refresh ~5 min early.
let cachedToken = null;
let cachedExpiresAt = 0;
let cachedFingerprint = null;

// SSHPublicKey has a uniqueIndex constraint, so each VU needs a distinct
// (fake but unique) public key string. Suffixing with VU+timestamp keeps
// the full text unique without affecting the rest of the body.
let cachedPubkey = null;

function getToken() {
  const now = Date.now();
  if (cachedToken && now < cachedExpiresAt) return cachedToken;

  if (!cachedFingerprint) {
    cachedFingerprint = `SHA256:k6-${__VU}-${now.toString(36)}`;
    cachedPubkey = `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILoadTest${__VU}-${now.toString(36)} k6@vu${__VU}`;
  }

  const res = http.post(
    `${BASE}/api/v1/auth/token`,
    JSON.stringify({
      client_secret: SECRET,
      fingerprint: cachedFingerprint,
      ssh_public_key: cachedPubkey,
    }),
    { headers: { 'Content-Type': 'application/json' }, tags: { name: 'auth_token_refresh' } },
  );

  if (res.status !== 200) return null;
  const body = JSON.parse(res.body);
  cachedToken = body.access_token;
  cachedExpiresAt = now + (body.expires_in - 300) * 1000;
  return cachedToken;
}

function authHeaders() {
  const tok = getToken();
  return tok ? { Authorization: `Bearer ${tok}`, 'Content-Type': 'application/json' } : null;
}

export const options = {
  discardResponseBodies: true,
  scenarios: {
    browse: {
      executor: 'ramping-arrival-rate',
      startRate: 6,
      timeUnit: '1m',
      preAllocatedVUs: 10,
      maxVUs: 50,
      stages: [
        { target: 12,  duration: '1h' },
        { target: 30,  duration: '2h' },
        { target: 90,  duration: '2h' },
        { target: 60,  duration: '2h' },
        { target: 120, duration: '3h' },
        { target: 30,  duration: '2h' },
      ],
      exec: 'browse',
    },
    authed_browse: {
      executor: 'ramping-arrival-rate',
      startRate: 2,
      timeUnit: '1m',
      preAllocatedVUs: 5,
      maxVUs: 20,
      stages: [
        { target: 10, duration: '4h' },
        { target: 30, duration: '4h' },
        { target: 8,  duration: '4h' },
      ],
      exec: 'authedBrowse',
    },
    convert_fail: {
      // /cart/convert is rate-limited 10/min per user. We share one loadtest
      // user across all VUs so ceiling is 10/min; we stay under at 4/min.
      executor: 'ramping-arrival-rate',
      startRate: 1,
      timeUnit: '1m',
      preAllocatedVUs: 3,
      maxVUs: 5,
      stages: [
        { target: 2, duration: '4h' },
        { target: 4, duration: '4h' },
        { target: 2, duration: '4h' },
      ],
      exec: 'convertFail',
    },
    address_bad: {
      executor: 'ramping-arrival-rate',
      startRate: 1,
      timeUnit: '1m',
      preAllocatedVUs: 3,
      maxVUs: 5,
      stages: [
        { target: 3, duration: '12h' },
      ],
      exec: 'addressBad',
    },
    auth_warn: {
      executor: 'ramping-arrival-rate',
      startRate: 1,
      timeUnit: '1m',
      preAllocatedVUs: 5,
      maxVUs: 10,
      stages: [
        { target: 3,  duration: '4h' },
        { target: 10, duration: '4h' },
        { target: 5,  duration: '4h' },
      ],
      exec: 'authBad',
    },
    auth_lookup: {
      executor: 'ramping-arrival-rate',
      startRate: 1,
      timeUnit: '1m',
      preAllocatedVUs: 5,
      maxVUs: 10,
      stages: [
        { target: 4, duration: '6h' },
        { target: 8, duration: '6h' },
      ],
      exec: 'authLookup',
    },
    err_404: {
      executor: 'ramping-arrival-rate',
      startRate: 1,
      timeUnit: '1m',
      preAllocatedVUs: 3,
      maxVUs: 5,
      stages: [
        { target: 5, duration: '12h' },
      ],
      exec: 'notFound',
    },
  },
};

export default function () {
  // Smoke-test entry point for `--vus N --duration T` runs.
  browse();
  authBad();
  authLookup();
  notFound();
  if (SECRET) {
    authedBrowse();
    convertFail();
    addressBad();
  }
}

export function browse() {
  const r = Math.random();
  if (r < 0.6)       http.get(`${BASE}/api/v1/products`,   { tags: { name: 'list_products' } });
  else if (r < 0.8)  http.get(`${BASE}/api/v1/products/1`, { tags: { name: 'get_product' } });
  else if (r < 0.95) http.get(`${BASE}/api/v1/health`,     { tags: { name: 'health' } });
  else               http.get(`${BASE}/api/v1/ping`,       { tags: { name: 'ping' } });
}

export function authedBrowse() {
  const h = authHeaders();
  if (!h) return;
  const r = Math.random();
  if (r < 0.4)       http.get(`${BASE}/api/v1/cart`,      { headers: h, tags: { name: 'authed_get_cart' } });
  else if (r < 0.6)  http.get(`${BASE}/api/v1/cards`,     { headers: h, tags: { name: 'authed_list_cards' } });
  else if (r < 0.8)  http.get(`${BASE}/api/v1/addresses`, { headers: h, tags: { name: 'authed_list_addresses' } });
  else               http.get(`${BASE}/api/v1/orders`,    { headers: h, tags: { name: 'authed_list_orders' } });
}

export function convertFail() {
  const h = authHeaders();
  if (!h) return;
  // Reset cart, drop one item in, attempt convert. No card/address selected
  // → ConvertCart hits validation_missing_address → cart_conversion_total
  // outcome=validation_missing_address + audit.OrderFailed warn.
  http.del(`${BASE}/api/v1/cart`, null, { headers: h, tags: { name: 'cart_clear' } });
  http.put(
    `${BASE}/api/v1/cart/item`,
    JSON.stringify({ coffee_id: 1, quantity: 1 }),
    { headers: h, tags: { name: 'cart_add_item' } },
  );
  http.post(`${BASE}/api/v1/cart/convert`, null, { headers: h, tags: { name: 'cart_convert_fail' } });
}

export function addressBad() {
  const h = authHeaders();
  if (!h) return;
  // Junk US zip + street → shippo validation fails → addressLog.Warn.
  http.post(
    `${BASE}/api/v1/addresses`,
    JSON.stringify({
      name: 'Load Test',
      street: 'asdfasdfasdf',
      city: 'Nowhere',
      state: 'XX',
      zip: '00000',
      country: 'US',
      phone: '+15555550000',
    }),
    { headers: h, tags: { name: 'address_create_bad' } },
  );
}

export function authBad() {
  http.post(
    `${BASE}/api/v1/auth/token`,
    JSON.stringify({ client_secret: 'definitely_wrong', fingerprint: 'SHA256:fake' }),
    { headers: { 'Content-Type': 'application/json' }, tags: { name: 'auth_bad_creds' } },
  );
}

export function authLookup() {
  const fp = `SHA256:${Math.random().toString(36).slice(2, 14)}`;
  http.get(`${BASE}/api/v1/auth/user?fingerprint=${fp}`, { tags: { name: 'auth_user_lookup' } });
}

export function notFound() {
  const id = 99000 + Math.floor(Math.random() * 1000);
  http.get(`${BASE}/api/v1/products/${id}`, { tags: { name: 'product_not_found' } });
}
