// tests/loadtest/scripts/auth_load.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import { htmlReport } from 'https://raw.githubusercontent.com/benc-uk/k6-reporter/main/dist/bundle.js';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.1/index.js';

export const options = {
  stages: [
    { duration: '30s', target: 10 },
    { duration: '1m', target: 50 },
    { duration: '20s', target: 0 },
  ],
  thresholds: {
    // ✅ Seuils réalistes pour environnement local avec 10+ VUs
    'http_req_duration': ['p(95) < 5000'],
    'http_req_failed': ['rate < 0.05'],     // 5% d'erreurs tolérées
    'checks': ['rate > 0.90'],              // 90% de réussite minimum
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  const timestamp = Date.now();
  const vuId = __VU;
  const iter = __ITER;
  // 🆕 UUID plus long (12 chars) pour éviter les collisions
  const uniqueId = uuidv4().replace(/-/g, '').slice(0, 12);

  // 🆕 Email ultra-unique avec UUID de 12 caractères
  const email = `loadtest_${timestamp}_${vuId}_${iter}_${uniqueId}@example.com`;
  const password = 'Password123!';

  const registerPayload = JSON.stringify({
    email: email,
    password: password,
    role: "merchant"
  });

  // 1. Inscription
  const registerRes = http.post(`${BASE_URL}/register`, registerPayload, {
    headers: { 'Content-Type': 'application/json' },
    tags: { endpoint: 'register' },
  });

  const registerOk = check(registerRes, {
    '✅ register status is 201': (r) => r.status === 201,
  });

  // ✅ Early return si l'inscription échoue
  if (!registerOk) {
    console.error(`❌ Register failed for ${email}: ${registerRes.status}`);
    sleep(0.1);
    return;
  }

  // 2. Connexion
  const loginPayload = JSON.stringify({
    email: email,
    password: password,
  });

  const loginRes = http.post(`${BASE_URL}/login`, loginPayload, {
    headers: { 'Content-Type': 'application/json' },
    tags: { endpoint: 'login' },
  });

  const loginOk = check(loginRes, {
    '✅ login status is 200': (r) => r.status === 200,
    '✅ login returns token': (r) => {
      try {
        return JSON.parse(r.body).token !== undefined;
      } catch {
        return false;
      }
    },
  });

  // ✅ Early return si la connexion échoue
  if (!loginOk) {
    console.error(`❌ Login failed for ${email}: ${loginRes.status}`);
    sleep(0.1);
    return;
  }

  // 3. Route protégée
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
      '✅ profile status is 200': (r) => r.status === 200,
      '✅ profile returns valid data': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body.id && body.email;
        } catch {
          return false;
        }
      },
    });
  } catch (e) {
    console.error(`❌ Profile request failed: ${e}`);
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