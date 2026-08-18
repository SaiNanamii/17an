import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:3000';

// Real sample values from the live dataset (ws_user.user_id=4), so search/
// profile/duplicates endpoints exercise a real row instead of 404-ing on
// every request.
const SAMPLE_USER_ID = __ENV.SAMPLE_USER_ID || '4';
const SAMPLE_EMAIL = __ENV.SAMPLE_EMAIL || 'testdup_0@test.com';
const SAMPLE_PHONE = __ENV.SAMPLE_PHONE || '1929391047';

export const options = {
  scenarios: {
    load: {
      executor: 'constant-vus',
      vus: 100,
      duration: '60s',
    },
  },
  thresholds: {
    http_req_duration: ['avg<500'],
    http_req_failed: ['rate<0.05'],
  },
};

// k6 requires every metric to be declared in the init context (top-level,
// executed once at script load), not lazily on first use inside the VU
// loop -- doing it lazily threw a script exception on every request past
// the first, silently killing the rest of each iteration.
const ENDPOINT_NAMES = [
  'health_root', 'health_api', 'search_email', 'search_phone', 'search_user_id',
  'search_name', 'quality', 'metrics', 'duplicates_post', 'duplicates_find',
  'duplicates_by_user', 'user_profile', 'v1_users_list', 'v1_users_get',
];
const trends = {};
for (const name of ENDPOINT_NAMES) trends[name] = new Trend(`${name}_duration`, true);

function get(name, path, wantStatus) {
  const res = http.get(`${BASE_URL}${path}`, { timeout: '5s' });
  check(res, { [`${name} status ${wantStatus}`]: (r) => r.status === wantStatus });
  trends[name].add(res.timings.duration);
  return res;
}

export default function () {
  get('health_root', '/health', 200);
  get('health_api', '/api/health', 200);

  get('search_email', `/api/search?q=${encodeURIComponent(SAMPLE_EMAIL)}&type=email`, 200);
  get('search_phone', `/api/search?q=${encodeURIComponent(SAMPLE_PHONE)}&type=phone`, 200);
  get('search_user_id', `/api/search?q=${SAMPLE_USER_ID}&type=user_id`, 200);
  get('search_name', '/api/search?q=john&type=name', 200);

  get('quality', '/api/quality', 200);
  get('metrics', '/api/metrics', 200);

  {
    const res = http.post(`${BASE_URL}/api/duplicates`, JSON.stringify({ limit: 50 }), {
      headers: { 'Content-Type': 'application/json' },
      timeout: '5s',
    });
    check(res, { 'duplicates_post status 200': (r) => r.status === 200 });
    trends['duplicates_post'].add(res.timings.duration);
  }

  get('duplicates_find', '/api/duplicates/find?method=ip_address&limit=20', 200);
  get('duplicates_by_user', `/api/duplicates/${SAMPLE_USER_ID}`, 200);
  get('user_profile', `/api/user-profile/${SAMPLE_USER_ID}`, 200);

  get('v1_users_list', '/api/v1/users?page=1&limit=20', 200);
  get('v1_users_get', `/api/v1/users/${SAMPLE_USER_ID}`, 200);

  sleep(0.2);
}
