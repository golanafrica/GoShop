// tests/loadtest/scripts/auth_load.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { htmlReport } from 'https://raw.githubusercontent.com/benc-uk/k6-reporter/main/dist/bundle.js';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.1/index.js';

export const options = {
  stages: [
    { duration: '30s', target: 10 },
    { duration: '1m', target: 50 },
    { duration: '20s', target: 0 },
  ],
  thresholds: {
    'http_req_duration': ['p(95) < 4000'],
    'http_req_failed': ['rate < 0.02'],
    'checks': ['rate > 0.85'],
  },
  // ⚠️ gracefulStop supprimé (invalide ici)
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  const timestamp = Date.now();
  const vuId = __VU;
  const iter = __ITER;

  const registerPayload = JSON.stringify({
    email: `loadtest_${timestamp}_${vuId}_${iter}@example.com`,
    password: 'Password123!',
  });

  // 1. Inscription
  const registerRes = http.post(`${BASE_URL}/register`, registerPayload, {
    headers: { 'Content-Type': 'application/json' },
    tags: { endpoint: 'register' },
  });

  check(registerRes, {
    'register status is 201': (r) => r.status === 201,
  });

  // 2. Connexion
  const loginRes = http.post(`${BASE_URL}/login`, registerPayload, {
    headers: { 'Content-Type': 'application/json' },
    tags: { endpoint: 'login' },
  });

  check(loginRes, {
    'login status is 200': (r) => r.status === 200,
    'login returns token': (r) => {
      try {
        return JSON.parse(r.body).token !== undefined;
      } catch {
        return false;
      }
    },
  });

  // 3. Route protégée
  if (loginRes.status === 200) {
    try {
      const token = JSON.parse(loginRes.body).token;
      const profileRes = http.get(`${BASE_URL}/auth/me`, {
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        tags: { endpoint: 'auth-me' },
      });

      check(profileRes, {
        'profile status is 200': (r) => r.status === 200,
        'profile returns valid data': (r) => {
          try {
            const body = JSON.parse(r.body);
            return body.id && body.email;
          } catch {
            return false;
          }
        },
      });
    } catch (e) {
      // ignore
    }
  }

  sleep(0.5);
}

export function handleSummary(data) {
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
  return {
    stdout: textSummary(data, { indent: ' ', enableColors: true }),
    [`./results/auth_load_report_${timestamp}.html`]: htmlReport(data),
    [`./results/auth_load_summary_${timestamp}.json`]: JSON.stringify(data, null, 2),
  };
}