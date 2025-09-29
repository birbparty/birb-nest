import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';
import { randomItem, randomString } from './utils.js';

// Custom metrics
const cacheHitRate = new Rate('cache_hit_rate');
const cacheMissRate = new Rate('cache_miss_rate');
const readLatency = new Trend('read_latency', true);
const writeLatency = new Trend('write_latency', true);

// Configuration
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_VERSION = 'v1';

// Test data
const testKeys = [];
const numKeys = 10000;

// Generate test keys during setup
export function setup() {
  console.log(`Generating ${numKeys} test keys...`);
  for (let i = 0; i < numKeys; i++) {
    testKeys.push(`test-key-${i}-${randomString(8)}`);
  }
  
  // Pre-populate some keys for cache hit testing
  const prePopulateCount = Math.floor(numKeys * 0.3); // 30% pre-populated
  console.log(`Pre-populating ${prePopulateCount} keys...`);
  
  for (let i = 0; i < prePopulateCount; i++) {
    const key = testKeys[i];
    const value = {
      data: `test-value-${i}`,
      timestamp: new Date().toISOString(),
      metadata: {
        source: 'k6-load-test',
        iteration: i
      }
    };
    
    http.post(`${BASE_URL}/${API_VERSION}/cache/${key}`, JSON.stringify({ value }), {
      headers: { 'Content-Type': 'application/json' },
    });
  }
  
  console.log('Setup complete!');
  return { testKeys };
}

// Scenario 1: Read-heavy workload (90% reads, 10% writes)
export function readHeavyWorkload(data) {
  const key = randomItem(data.testKeys);
  
  if (Math.random() < 0.9) {
    // Read operation
    const start = new Date();
    const res = http.get(`${BASE_URL}/${API_VERSION}/cache/${key}`);
    const duration = new Date() - start;
    
    readLatency.add(duration);
    
    check(res, {
      'read status is 200 or 404': (r) => r.status === 200 || r.status === 404,
    });
    
    if (res.status === 200) {
      cacheHitRate.add(1);
      cacheMissRate.add(0);
    } else if (res.status === 404) {
      cacheHitRate.add(0);
      cacheMissRate.add(1);
    }
  } else {
    // Write operation
    const value = {
      data: randomString(256),
      timestamp: new Date().toISOString(),
      workload: 'read-heavy'
    };
    
    const start = new Date();
    const res = http.post(`${BASE_URL}/${API_VERSION}/cache/${key}`, JSON.stringify({ value }), {
      headers: { 'Content-Type': 'application/json' },
    });
    const duration = new Date() - start;
    
    writeLatency.add(duration);
    
    check(res, {
      'write status is 200 or 201': (r) => r.status === 200 || r.status === 201,
    });
  }
}

// Scenario 2: Write-heavy workload (10% reads, 90% writes)
export function writeHeavyWorkload(data) {
  const key = randomItem(data.testKeys);
  
  if (Math.random() < 0.1) {
    // Read operation
    const start = new Date();
    const res = http.get(`${BASE_URL}/${API_VERSION}/cache/${key}`);
    const duration = new Date() - start;
    
    readLatency.add(duration);
    
    check(res, {
      'read status is 200 or 404': (r) => r.status === 200 || r.status === 404,
    });
  } else {
    // Write operation
    const value = {
      data: randomString(512),
      timestamp: new Date().toISOString(),
      workload: 'write-heavy',
      iteration: __ITER
    };
    
    const start = new Date();
    const res = http.post(`${BASE_URL}/${API_VERSION}/cache/${key}`, JSON.stringify({ value }), {
      headers: { 'Content-Type': 'application/json' },
    });
    const duration = new Date() - start;
    
    writeLatency.add(duration);
    
    check(res, {
      'write status is 200 or 201': (r) => r.status === 200 || r.status === 201,
    });
  }
}

// Scenario 3: Mixed workload (50% reads, 50% writes)
export function mixedWorkload(data) {
  const key = randomItem(data.testKeys);
  
  if (Math.random() < 0.5) {
    // Read operation
    const start = new Date();
    const res = http.get(`${BASE_URL}/${API_VERSION}/cache/${key}`);
    const duration = new Date() - start;
    
    readLatency.add(duration);
    
    check(res, {
      'read status is 200 or 404': (r) => r.status === 200 || r.status === 404,
    });
    
    if (res.status === 200) {
      cacheHitRate.add(1);
      cacheMissRate.add(0);
    } else if (res.status === 404) {
      cacheHitRate.add(0);
      cacheMissRate.add(1);
    }
  } else {
    // Write operation
    const value = {
      data: randomString(384),
      timestamp: new Date().toISOString(),
      workload: 'mixed'
    };
    
    const start = new Date();
    const res = http.post(`${BASE_URL}/${API_VERSION}/cache/${key}`, JSON.stringify({ value }), {
      headers: { 'Content-Type': 'application/json' },
    });
    const duration = new Date() - start;
    
    writeLatency.add(duration);
    
    check(res, {
      'write status is 200 or 201': (r) => r.status === 200 || r.status === 201,
    });
  }
}

// Scenario 4: Cache miss storm (force rehydration)
export function cacheMissStorm(data) {
  // Generate new keys that are guaranteed to miss
  const missKey = `miss-key-${__VU}-${__ITER}-${randomString(8)}`;
  
  const start = new Date();
  const res = http.get(`${BASE_URL}/${API_VERSION}/cache/${missKey}`);
  const duration = new Date() - start;
  
  readLatency.add(duration);
  cacheMissRate.add(1);
  cacheHitRate.add(0);
  
  check(res, {
    'cache miss returns 404': (r) => r.status === 404,
  });
}

// Scenario 5: Concurrent key access (hot key testing)
export function hotKeyAccess(data) {
  // Focus on a small subset of keys to create hot spots
  const hotKeyCount = 100;
  const hotKeyIndex = Math.floor(Math.random() * hotKeyCount);
  const key = data.testKeys[hotKeyIndex];
  
  if (Math.random() < 0.7) {
    // Read hot key
    const start = new Date();
    const res = http.get(`${BASE_URL}/${API_VERSION}/cache/${key}`);
    const duration = new Date() - start;
    
    readLatency.add(duration);
    
    check(res, {
      'hot key read successful': (r) => r.status === 200 || r.status === 404,
    });
  } else {
    // Update hot key
    const value = {
      data: randomString(256),
      timestamp: new Date().toISOString(),
      hotKey: true,
      updateCount: __ITER
    };
    
    const start = new Date();
    const res = http.post(`${BASE_URL}/${API_VERSION}/cache/${key}`, JSON.stringify({ value }), {
      headers: { 'Content-Type': 'application/json' },
    });
    const duration = new Date() - start;
    
    writeLatency.add(duration);
    
    check(res, {
      'hot key write successful': (r) => r.status === 200 || r.status === 201,
    });
  }
}

// Default function (can be overridden by --env SCENARIO=xxx)
export default function(data) {
  const scenario = __ENV.SCENARIO || 'mixed';
  
  switch(scenario) {
    case 'read-heavy':
      readHeavyWorkload(data);
      break;
    case 'write-heavy':
      writeHeavyWorkload(data);
      break;
    case 'mixed':
      mixedWorkload(data);
      break;
    case 'cache-miss':
      cacheMissStorm(data);
      break;
    case 'hot-key':
      hotKeyAccess(data);
      break;
    default:
      mixedWorkload(data);
  }
  
  // Small sleep between requests
  sleep(0.01); // 10ms
}

// Load test stages
export const options = {
  stages: [
    { duration: '30s', target: 100 },   // Ramp up to 100 users
    { duration: '1m', target: 500 },    // Ramp up to 500 users
    { duration: '2m', target: 1000 },   // Ramp up to 1000 users
    { duration: '3m', target: 1000 },   // Stay at 1000 users
    { duration: '1m', target: 500 },    // Ramp down to 500 users
    { duration: '30s', target: 0 },     // Ramp down to 0 users
  ],
  thresholds: {
    http_req_duration: ['p(95)<100', 'p(99)<200'], // 95% of requests under 100ms, 99% under 200ms
    http_req_failed: ['rate<0.01'],                 // Error rate under 1%
    read_latency: ['p(99)<10'],                     // 99% of reads under 10ms
    write_latency: ['p(99)<50'],                    // 99% of writes under 50ms
    cache_hit_rate: ['rate>0.8'],                   // Cache hit rate above 80%
  },
};

// Teardown function
export function teardown(data) {
  console.log('Load test complete!');
  console.log(`Total keys used: ${data.testKeys.length}`);
}
