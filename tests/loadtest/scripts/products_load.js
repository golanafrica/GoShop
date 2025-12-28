// tests/loadtest/scripts/products_load.js
// tests/loadtest/scripts/products_load_fixed.js
import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { htmlReport } from 'https://raw.githubusercontent.com/benc-uk/k6-reporter/main/dist/bundle.js';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.1/index.js';

export const options = {
  stages: [
    { duration: '1m', target: 20 },
    { duration: '3m', target: 100 },
    { duration: '1m', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate < 0.02'],
    http_req_duration: ['p(95) < 3000'],
    checks: ['rate > 0.95'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// === SOLUTION 1 : Deux options pour le token ===
// Option A: Token passé en variable d'environnement
// Option B: Auto-login avec credentials

let AUTH_TOKEN = __ENV.ADMIN_TOKEN;
let USE_AUTO_LOGIN = false;

// === SETUP : Obtention du token si non fourni ===
export function setup() {
  console.log('🔧 Setup: Configuration du test...');
  
  // Si token fourni, vérifier qu'il est valide
  if (AUTH_TOKEN) {
    console.log(`✅ Token fourni (${AUTH_TOKEN.length} caractères)`);
    
    // Tester le token
    const testRes = http.get(`${BASE_URL}/auth/me`, {
      headers: {
        Authorization: `Bearer ${AUTH_TOKEN}`,
        'Content-Type': 'application/json',
      },
      timeout: '10s',
    });
    
    if (testRes.status === 200) {
      console.log('✅ Token valide détecté');
      return { token: AUTH_TOKEN, method: 'provided' };
    } else {
      console.log(`❌ Token invalide (${testRes.status}), passage en auto-login`);
      AUTH_TOKEN = null;
    }
  }
  
  // Auto-login si pas de token ou token invalide
  const ADMIN_EMAIL = __ENV.ADMIN_EMAIL || 'admin@example.com';
  const ADMIN_PASSWORD = __ENV.ADMIN_PASSWORD || 'admin123';
  
  console.log(`🔑 Tentative d'auto-login avec: ${ADMIN_EMAIL}`);
  
  // 1. Essayer de se connecter
  const loginRes = http.post(`${BASE_URL}/login`, JSON.stringify({
    email: ADMIN_EMAIL,
    password: ADMIN_PASSWORD,
  }), {
    headers: { 'Content-Type': 'application/json' },
    timeout: '30s',
  });
  
  if (loginRes.status === 200) {
    try {
      const token = JSON.parse(loginRes.body).token;
      console.log(`✅ Auto-login réussi (${token.length} caractères)`);
      USE_AUTO_LOGIN = true;
      return { token: token, method: 'auto-login' };
    } catch (e) {
      console.log(`❌ Erreur parsing login: ${e}`);
    }
  }
  
  // 2. Si login échoue, essayer de créer l'utilisateur
  console.log('🔄 Tentative de création utilisateur...');
  const registerRes = http.post(`${BASE_URL}/register`, JSON.stringify({
    email: ADMIN_EMAIL,
    password: ADMIN_PASSWORD,
  }), {
    headers: { 'Content-Type': 'application/json' },
    timeout: '30s',
  });
  
  if (registerRes.status === 201 || registerRes.status === 400) {
    // 400 = utilisateur existe déjà, c'est OK
    console.log('✅ Utilisateur créé ou existe déjà');
    
    // Re-tenter le login
    const retryLogin = http.post(`${BASE_URL}/login`, JSON.stringify({
      email: ADMIN_EMAIL,
      password: ADMIN_PASSWORD,
    }), {
      headers: { 'Content-Type': 'application/json' },
      timeout: '30s',
    });
    
    if (retryLogin.status === 200) {
      const token = JSON.parse(retryLogin.body).token;
      console.log(`✅ Login après création réussi (${token.length} caractères)`);
      USE_AUTO_LOGIN = true;
      return { token: token, method: 'register-then-login' };
    }
  }
  
  console.log('❌ Impossible d\'obtenir un token valide');
  return { token: null, method: 'failed' };
}

// === FONCTION POUR OBTENIR LES HEADERS ===
function getAuthHeaders(token) {
  return {
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
  };
}

// === TEST PRINCIPAL ===
export default function (data) {
  const { token, method } = data;
  
  if (!token) {
    console.log('❌ Aucun token disponible - skipping iteration');
    return;
  }
  
  // Rafraîchir le token périodiquement si en auto-login
  if (USE_AUTO_LOGIN && __ITER % 50 === 0) {
    console.log(`🔄 Rafraîchissement du token (itération ${__ITER})`);
  }
  
  const authHeaders = getAuthHeaders(token);
  
  group('📦 API Produits - CRUD Complet', () => {
    
    // === 1. LISTER LES PRODUITS ===
    // === 1. LISTER LES PRODUITS ===
// AJOUTE LA PAGINATION !
const limit = 20;
const page = Math.floor(Math.random() * 10); // 10 pages différentes
const offset = page * limit;

const listRes = http.get(`${BASE_URL}/api/products?limit=${limit}&offset=${offset}`, authHeaders);
    
    check(listRes, {
      '⚡ list products - status 200': (r) => r.status === 200,
      '⚡ list products - JSON valide': (r) => {
        try {
          JSON.parse(r.body);
          return true;
        } catch {
          return false;
        }
      },
    });
    
    // === 2. CRÉER UN PRODUIT ===
    const productData = {
      name: `LoadTest-${Date.now()}-${__VU}-${__ITER}`,
      description: `Produit de test créé par VU ${__VU} à ${new Date().toISOString()}`,
      price_cents: Math.floor(Math.random() * 100000) + 1000,
      stock: Math.floor(Math.random() * 100) + 1,
    };
    
    const createRes = http.post(
      `${BASE_URL}/api/products`,
      JSON.stringify(productData),
      authHeaders
    );
    
    // Log détaillé en cas d'erreur
    if (createRes.status !== 201) {
      console.log(`CREATE ERROR: ${createRes.status} - ${createRes.body}`);
      
      // Si token invalide, essayer de le rafraîchir
      if (createRes.status === 401 && USE_AUTO_LOGIN) {
        console.log('🔄 Token invalide détecté, besoin de rafraîchir');
      }
    }
    
    let productId = null;
    let createdProduct = null;
    
    if (createRes.status === 201) {
      try {
        createdProduct = JSON.parse(createRes.body);
        productId = createdProduct.id;
        
        // Vérifier que la réponse contient tous les champs
        if (!createdProduct.id || !createdProduct.name) {
          console.log('❌ Réponse de création incomplète:', createdProduct);
        }
      } catch (e) {
        console.log(`❌ Parse error: ${e} - Body: ${createRes.body}`);
      }
    }
    
    check(createRes, {
      '✅ create product - status 201': (r) => r.status === 201,
      '✅ create product - returns valid ID': () => productId !== null,
    });
    
    // === 3. RÉCUPÉRER LE PRODUIT (si création réussie) ===
    if (productId) {
      const getRes = http.get(`${BASE_URL}/api/products/${productId}`, authHeaders);
      
      check(getRes, {
        '✅ get product - status 200': (r) => r.status === 200,
        '✅ get product - matches created ID': (r) => {
          try {
            const body = JSON.parse(r.body);
            return body.id === productId;
          } catch {
            return false;
          }
        },
      });
      
      // === 4. METTRE À JOUR LE PRODUIT ===
      const updateData = {
        name: `${productData.name} [UPDATED]`,
        description: `${productData.description} - Mis à jour`,
        price_cents: productData.price_cents + 500,
        stock: Math.max(1, productData.stock - 3),
      };
      
      const updateRes = http.put(
        `${BASE_URL}/api/products/${productId}`,
        JSON.stringify(updateData),
        authHeaders
      );
      
      check(updateRes, {
        '✅ update product - status 200': (r) => r.status === 200,
        '✅ update product - valid response': (r) => {
          if (r.status !== 200) {
            console.log(`UPDATE ERROR: ${r.status} - ${r.body}`);
            return false;
          }
          return true;
        },
      });
    }
    
    // === 5. TEST DE PERFORMANCE SIMPLE ===
    // Test un endpoint public pour comparer
    const healthRes = http.get(`${BASE_URL}/health/live`);
    check(healthRes, {
      '📊 health check - status 200': (r) => r.status === 200,
    });
    
    // Pause réaliste entre les actions
    sleep(1 + Math.random() * 2);
  });
}

// === CLEANUP AMÉLIORÉ ===
const createdProductIds = [];
const CLEAN_UP = __ENV.CLEAN_UP !== 'false'; // true par défaut

export function teardown(data) {
  const { token } = data;
  
  if (!CLEAN_UP || createdProductIds.length === 0 || !token) {
    return;
  }
  
  console.log(`🧹 Nettoyage: ${createdProductIds.length} produits à supprimer...`);
  
  const authHeaders = getAuthHeaders(token);
  let deletedCount = 0;
  
  // Supprimer les produits créés
  createdProductIds.forEach((id, index) => {
    if (index < 100) { // Limiter à 100 max pour ne pas surcharger
      const delRes = http.del(`${BASE_URL}/api/products/${id}`, null, authHeaders);
      
      if (delRes.status === 204 || delRes.status === 200) {
        deletedCount++;
      } else {
        console.log(`❌ Échec suppression ${id}: ${delRes.status}`);
      }
      
      // Petite pause entre les suppressions
      if (index % 10 === 0) sleep(0.1);
    }
  });
  
  console.log(`✅ ${deletedCount}/${createdProductIds.length} produits supprimés`);
}

// === RAPPORTS ===
export function handleSummary(data) {
  const timestamp = new Date().toISOString()
    .replace(/[:.]/g, '-')
    .slice(0, 19);
  
  // Créer le dossier results s'il n'existe pas
  // (k6 ne le fait pas automatiquement)
  
  return {
    stdout: textSummary(data, { indent: ' ', enableColors: true }),
    [`results/products_load_${timestamp}.html`]: htmlReport(data),
    [`results/products_load_${timestamp}.json`]: JSON.stringify(data, null, 2),
  };
}

// === FONCTION UTILITAIRE : Valider un token ===
function validateToken(token) {
  if (!token || token.length < 10) {
    return { valid: false, reason: 'Token trop court ou nul' };
  }
  
  // Vérifier le format JWT basique (3 parties séparées par des points)
  const parts = token.split('.');
  if (parts.length !== 3) {
    return { valid: false, reason: 'Format JWT invalide' };
  }
  
  return { valid: true, reason: 'Format OK' };
}