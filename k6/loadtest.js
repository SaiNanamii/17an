import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:3000';

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

const healthTrend = new Trend('health_duration', true);
const searchPhoneTrend = new Trend('search_phone_duration', true);
const searchNameTrend = new Trend('search_name_duration', true);

export default function () {
  {
    // CHALLENGE.md Round 5: 100 concurrent for 60s, 5s per-request timeout.
    const res = http.get(`${BASE_URL}/health`, { timeout: '5s' });
    check(res, {
      'GET /health status 200': (r) => r.status === 200,
      'GET /health status=ready': (r) => {
        if (r.status !== 200 || !r.body) return false;
        const body = r.json();
        return body && body.status === 'ready';
      },
    });
    healthTrend.add(res.timings.duration);
  }

  {
    const res = http.get(`${BASE_URL}/api/search?q=81234567890&type=phone`, { timeout: '5s' });
    check(res, { 'GET /api/search (phone) status 200': (r) => r.status === 200 });
    searchPhoneTrend.add(res.timings.duration);
  }

  {
    const res = http.get(`${BASE_URL}/api/search?q=john&type=name`, { timeout: '5s' });
    check(res, { 'GET /api/search (name) status 200': (r) => r.status === 200 });
    searchNameTrend.add(res.timings.duration);
  }

  sleep(0.2);
}
