// scripts/products_smoke.js - Version avec débogage
import http from 'k6/http';
import { check, sleep } from 'k6';

// Configuration
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const ADMIN_TOKEN = __ENV.ADMIN_TOKEN || '';

export const options = {
  vus: 5,  // Réduit pour le debug
  duration: '30s',
};

// Headers
function getHeaders() {
  return {
    'Content-Type': 'application/json',
    'Authorization': ADMIN_TOKEN ? `Bearer ${ADMIN_TOKEN}` : '',
  };
}

export default function () {
  const headers = getHeaders();
  
  // 1. Tester d'abord le token avec un endpoint simple
  const testRes = http.get(`${BASE_URL}/auth/me`, { headers });
  
  if (testRes.status === 200) {
    console.log(`Token valide, utilisateur: ${JSON.stringify(testRes.json())}`);
    
    // 2. Maintenant tester les produits
    const listRes = http.get(`${BASE_URL}/api/products?limit=5`, { headers });
    
    check(listRes, {
      'list products - status 200': (r) => r.status === 200,
      'list products - has JSON': (r) => {
        try {
          JSON.parse(r.body);
          return true;
        } catch {
          return false;
        }
      },
    });
    
    // Afficher le résultat pour debug
    if (listRes.status !== 200) {
      console.log(`Échec produits: ${listRes.status} - ${listRes.body}`);
    }
  } else {
    console.log(`Token invalide: ${testRes.status} - ${testRes.body}`);
  }
  
  // 3. Health check
  const healthRes = http.get(`${BASE_URL}/health/live`);
  check(healthRes, {
    'health check - status 200': (r) => r.status === 200,
  });
  
  sleep(2);
}

export function setup() {
  console.log('=== DEBUG MODE ===');
  console.log(`BASE_URL: ${BASE_URL}`);
  console.log(`ADMIN_TOKEN length: ${ADMIN_TOKEN.length}`);
  console.log(`ADMIN_TOKEN preview: ${ADMIN_TOKEN.substring(0, 50)}...`);
  
  // Tester le token immédiatement
  if (ADMIN_TOKEN) {
    const testRes = http.get(`${BASE_URL}/auth/me`, {
      headers: {
        'Authorization': `Bearer ${ADMIN_TOKEN}`,
        'Content-Type': 'application/json',
      },
    });
    
    console.log(`Token test: ${testRes.status}`);
    if (testRes.status === 200) {
      try {
        const user = testRes.json();
        console.log(`User: ${JSON.stringify(user)}`);
      } catch (e) {
        console.log(`Parse error: ${e}`);
      }
    } else {
      console.log(`Error body: ${testRes.body}`);
    }
  } else {
    console.log('No ADMIN_TOKEN provided');
  }
  
  return {};
}