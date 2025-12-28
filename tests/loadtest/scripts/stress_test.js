// tests/loadtest/scripts/stress_test.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { htmlReport } from 'https://raw.githubusercontent.com/benc-uk/k6-reporter/main/dist/bundle.js';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.1/index.js';

// Configuration réaliste pour un stress test
export const options = {
  stages: [
    { duration: '1m', target: 50 },    // Montée rapide
    { duration: '2m', target: 150 },   // Charge élevée
    { duration: '1m', target: 200 },   // Pic maximum
    { duration: '1m', target: 0 },     // Descente
  ],
  thresholds: {
    // ✅ Seuils adaptés au stress test
    http_req_failed: ['rate < 0.40'],  // Jusqu'à 40% d'erreurs acceptables
    http_req_duration: ['p(95) < 5000'], // p95 < 5 secondes
    checks: ['rate > 0.85'],          // 85% des checks OK
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function() {
  const action = Math.random();
  
  if (action < 0.2) {
    // 20%: Inscription (emails uniques)
    const registerRes = http.post(`${BASE_URL}/register`, JSON.stringify({
      email: `stresstest_${Date.now()}_${__VU}_${__ITER}@example.com`,
      password: 'Password123!'
    }), {
      headers: { 'Content-Type': 'application/json' },
      tags: { endpoint: 'register' }
    });
    
    check(registerRes, {
      'register succeeds': (r) => r.status === 201,
    });
    
  } else if (action < 0.5) {
    // 30%: Connexion (emails aléatoires - souvent inexistants → 401 OK)
    const userIndex = Math.floor(Math.random() * 100);
    const loginRes = http.post(`${BASE_URL}/login`, JSON.stringify({
      email: `loadtest_user_${userIndex}@example.com`,
      password: 'Password123!'
    }), {
      headers: { 'Content-Type': 'application/json' },
      tags: { endpoint: 'login' }
    });
    
    check(loginRes, {
      'login attempt': (r) => [200, 401].includes(r.status),
    });
    
  } else if (action < 0.8) {
    // 30%: Health check (très léger)
    const healthRes = http.get(`${BASE_URL}/health/live`, {
      tags: { endpoint: 'health' }
    });
    
    check(healthRes, {
      'health is live': (r) => r.status === 200,
    });
    
  } else {
    // 20%: Help endpoint
    const helpRes = http.get(`${BASE_URL}/help`, {
      tags: { endpoint: 'help' }
    });
    
    check(helpRes, {
      'help works': (r) => r.status === 200,
    });
  }
  
  // Pause aléatoire courte pour simuler un usage réel
  sleep(Math.random() * 0.3);
}

// Génération de rapport
export function handleSummary(data) {
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
  return {
    stdout: textSummary(data, { indent: ' ', enableColors: true }),
    [`./results/stress_${timestamp}.html`]: htmlReport(data),
    [`./results/stress_${timestamp}.json`]: JSON.stringify(data, null, 2),
  };
}