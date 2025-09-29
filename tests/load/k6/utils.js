// Utility functions for k6 load tests

// Generate a random string of specified length
export function randomString(length) {
  const characters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  let result = '';
  for (let i = 0; i < length; i++) {
    result += characters.charAt(Math.floor(Math.random() * characters.length));
  }
  return result;
}

// Get a random item from an array
export function randomItem(array) {
  return array[Math.floor(Math.random() * array.length)];
}

// Generate random JSON data of approximate size (in bytes)
export function randomJSON(approximateSize) {
  const baseObject = {
    id: randomString(16),
    timestamp: new Date().toISOString(),
    type: randomItem(['user', 'product', 'order', 'session', 'analytics']),
    status: randomItem(['active', 'pending', 'completed', 'archived']),
    metadata: {},
    data: {}
  };
  
  // Add fields until we reach approximate size
  let currentSize = JSON.stringify(baseObject).length;
  let fieldCount = 0;
  
  while (currentSize < approximateSize) {
    const fieldName = `field_${fieldCount}`;
    const fieldValue = randomString(Math.min(100, approximateSize - currentSize));
    
    if (fieldCount % 2 === 0) {
      baseObject.metadata[fieldName] = fieldValue;
    } else {
      baseObject.data[fieldName] = fieldValue;
    }
    
    currentSize = JSON.stringify(baseObject).length;
    fieldCount++;
  }
  
  return baseObject;
}

// Generate a random integer between min and max (inclusive)
export function randomInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

// Format bytes to human readable string
export function formatBytes(bytes) {
  if (bytes === 0) return '0 Bytes';
  
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// Calculate percentile from array of numbers
export function percentile(arr, p) {
  if (arr.length === 0) return 0;
  
  const sorted = arr.slice().sort((a, b) => a - b);
  const index = (p / 100) * (sorted.length - 1);
  
  if (Math.floor(index) === index) {
    return sorted[index];
  } else {
    const lower = sorted[Math.floor(index)];
    const upper = sorted[Math.ceil(index)];
    const weight = index % 1;
    return lower * (1 - weight) + upper * weight;
  }
}

// Generate realistic cache key patterns
export function generateCacheKey(pattern = 'default') {
  const patterns = {
    'user-session': () => `session:user:${randomInt(1, 100000)}:${randomString(8)}`,
    'api-cache': () => `api:v1:${randomItem(['users', 'products', 'orders'])}:${randomInt(1, 10000)}`,
    'feature-flag': () => `feature:${randomItem(['dark-mode', 'new-ui', 'beta-feature'])}:user:${randomInt(1, 100000)}`,
    'rate-limit': () => `ratelimit:${randomItem(['api', 'login', 'download'])}:${randomString(16)}`,
    'analytics': () => `analytics:${randomItem(['pageview', 'click', 'conversion'])}:${new Date().toISOString().split('T')[0]}`,
    'default': () => `cache:${randomString(32)}`
  };
  
  return patterns[pattern] ? patterns[pattern]() : patterns.default();
}

// Batch operations helper
export function batchRequests(items, batchSize) {
  const batches = [];
  for (let i = 0; i < items.length; i += batchSize) {
    batches.push(items.slice(i, i + batchSize));
  }
  return batches;
}

// Sleep with random jitter
export function sleepWithJitter(baseSeconds, jitterPercent = 0.1) {
  const jitter = baseSeconds * jitterPercent;
  const actualSleep = baseSeconds + (Math.random() * 2 - 1) * jitter;
  return actualSleep;
}

// Generate test data based on scenario
export function generateTestData(scenario, count = 1000) {
  const data = [];
  
  switch(scenario) {
    case 'user-data':
      for (let i = 0; i < count; i++) {
        data.push({
          userId: randomInt(1, 100000),
          username: `user_${randomString(8)}`,
          email: `${randomString(10)}@test.com`,
          profile: randomJSON(500),
          lastActive: new Date().toISOString()
        });
      }
      break;
      
    case 'product-catalog':
      for (let i = 0; i < count; i++) {
        data.push({
          productId: `PROD-${randomString(8)}`,
          name: `Product ${randomString(12)}`,
          price: randomInt(10, 1000) / 10,
          inventory: randomInt(0, 1000),
          categories: Array(randomInt(1, 5)).fill(null).map(() => randomItem(['electronics', 'clothing', 'food', 'toys', 'books'])),
          metadata: randomJSON(300)
        });
      }
      break;
      
    case 'time-series':
      const now = Date.now();
      for (let i = 0; i < count; i++) {
        data.push({
          timestamp: new Date(now - i * 1000).toISOString(),
          metric: randomItem(['cpu', 'memory', 'disk', 'network']),
          value: Math.random() * 100,
          tags: {
            host: `server-${randomInt(1, 10)}`,
            region: randomItem(['us-east', 'us-west', 'eu-central', 'ap-south']),
            environment: randomItem(['prod', 'staging', 'dev'])
          }
        });
      }
      break;
      
    default:
      for (let i = 0; i < count; i++) {
        data.push(randomJSON(randomInt(100, 1000)));
      }
  }
  
  return data;
}
