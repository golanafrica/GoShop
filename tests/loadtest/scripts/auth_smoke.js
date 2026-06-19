// tests/loadtest/scripts/auth_smoke.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.1/index.js';

// 🔧 Configuration réaliste pour un smoke test
export const options = {
  vus: 5,
  duration: '30s',
  thresholds: {
    // ✅ Seuils réalistes en dev local
    http_req_failed: ['rate < 0.01'],    // < 1% d'erreurs
    http_req_duration: ['p(95) < 2000'], // < 2s
    checks: ['rate > 0.95'],             // > 95% de réussite
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  const timestamp = Date.now();
  const vuId = __VU;
  const iter = __ITER;
  const uniqueId = uuidv4().slice(0, 8);

  const email = `smoketest_${timestamp}_${vuId}_${iter}_${uniqueId}@example.com`;
  const password = 'Password123!';

  // 1. Inscription
  const registerRes = http.post(
    `${BASE_URL}/register`,
    JSON.stringify({ email, password }),
    { headers: { 'Content-Type': 'application/json' } }
  );

  const registerOk = check(registerRes, {
    '✅ register status is 201': (r) => r.status === 201,
  });

  if (!registerOk) {
    console.error(`❌ Register failed for ${email}: ${registerRes.status}`);
    sleep(0.1);
    return;
  }

  sleep(0.5);

  // 2. Connexion
  const loginRes = http.post(
    `${BASE_URL}/login`,
    JSON.stringify({ email, password }),
    { headers: { 'Content-Type': 'application/json' } }
  );

  const loginOk = check(loginRes, {
    '✅ login status is 200': (r) => r.status === 200,
    '✅ login returns valid token': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.token && typeof body.token === 'string';
      } catch {
        return false;
      }
    },
  });

  if (!loginOk) {
    console.error(`❌ Login failed for ${email}: ${loginRes.status}`);
    sleep(0.1);
    return;
  }

  sleep(0.5);
}

export function handleSummary(data) {
  return {
    stdout: textSummary(data, { indent: ' ', enableColors: true }),
  };
}