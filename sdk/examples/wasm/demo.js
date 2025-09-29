// 🦜 Birb Nest SDK - Interactive WASM Demo
// This demo showcases the SDK's WebAssembly capabilities with an engaging UI

// ===== Global State =====
const state = {
    sdk: null,
    operationCount: 0,
    score: 0,
    streak: 0,
    successCount: 0,
    failureCount: 0,
    totalResponseTime: 0,
    cacheHits: 0,
    cacheMisses: 0,
    circuitState: 'closed',
    achievements: new Set(),
    theme: localStorage.getItem('theme') || 'light'
};

// ===== Achievement Definitions =====
const ACHIEVEMENTS = {
    firstOperation: { icon: '🎯', name: 'First Steps', description: 'Complete your first operation' },
    streak5: { icon: '🔥', name: 'On Fire', description: 'Achieve a 5 operation streak' },
    streak10: { icon: '💎', name: 'Diamond Hands', description: 'Achieve a 10 operation streak' },
    batchMaster: { icon: '📦', name: 'Batch Master', description: 'Complete a batch operation' },
    errorHandler: { icon: '🛡️', name: 'Error Handler', description: 'Successfully handle 10 errors' },
    speedDemon: { icon: '⚡', name: 'Speed Demon', description: 'Complete 10 operations under 50ms' },
    nightOwl: { icon: '🦉', name: 'Night Owl', description: 'Use dark mode' },
    cacheExpert: { icon: '🎯', name: 'Cache Expert', description: 'Achieve 90% cache hit rate' },
    persistent: { icon: '💪', name: 'Persistent', description: 'Complete 100 operations' },
    circuitBreaker: { icon: '⚡', name: 'Circuit Breaker', description: 'Trigger the circuit breaker' }
};

// ===== Sample Data Generators =====
const sampleData = {
    keys: [
        'user:profile:123',
        'session:abc123',
        'product:sku-456',
        'config:app-settings',
        'cache:temp-data',
        'api:response:789',
        'metrics:daily',
        'feature:flags',
        'auth:token:xyz',
        'data:analytics'
    ],
    
    generateKey() {
        return this.keys[Math.floor(Math.random() * this.keys.length)] + ':' + Date.now();
    },
    
    generateValue() {
        const types = [
            () => ({ // User object
                id: Math.floor(Math.random() * 10000),
                name: ['Alice', 'Bob', 'Charlie', 'Diana'][Math.floor(Math.random() * 4)],
                email: `user${Math.floor(Math.random() * 100)}@example.com`,
                active: Math.random() > 0.5,
                lastLogin: new Date().toISOString()
            }),
            () => ({ // Config object
                version: `${Math.floor(Math.random() * 3)}.${Math.floor(Math.random() * 10)}.${Math.floor(Math.random() * 20)}`,
                features: {
                    darkMode: Math.random() > 0.5,
                    betaFeatures: Math.random() > 0.7,
                    analytics: true
                },
                timestamp: Date.now()
            }),
            () => ({ // Metrics object
                requests: Math.floor(Math.random() * 10000),
                errors: Math.floor(Math.random() * 100),
                avgResponseTime: Math.floor(Math.random() * 200) + 50,
                uptime: '99.' + Math.floor(Math.random() * 100) + '%'
            }),
            () => `Simple string value ${Math.floor(Math.random() * 1000)}`, // String
            () => Math.floor(Math.random() * 10000), // Number
            () => ['item1', 'item2', 'item3', 'item4'].slice(0, Math.floor(Math.random() * 4) + 1) // Array
        ];
        
        const generator = types[Math.floor(Math.random() * types.length)];
        return JSON.stringify(generator(), null, 2);
    }
};

// ===== Initialize Demo =====
async function initializeDemo() {
    // Apply saved theme
    document.body.setAttribute('data-theme', state.theme);
    updateThemeIcon();
    
    // Create particles
    createParticles();
    
    // Initialize event listeners
    setupEventListeners();
    
    // Initialize keyboard shortcuts
    setupKeyboardShortcuts();
    
    // Initialize WASM and SDK
    await initializeSDK();
    
    // Start metrics update
    setInterval(updateMetrics, 1000);
    
    // Show welcome achievement
    setTimeout(() => {
        showToast('🦜 Welcome to Birb Nest SDK Demo!', 'info');
    }, 500);
}

// ===== SDK Initialization =====
async function initializeSDK() {
    try {
        updateConnectionStatus('Initializing WASM...');
        
        // Initialize Go WASM runtime
        const go = new Go();
        const result = await WebAssembly.instantiateStreaming(
            fetch('main.wasm'),
            go.importObject
        );
        
        go.run(result.instance);
        
        // Wait for SDK to be available
        await new Promise(resolve => {
            const checkSDK = setInterval(() => {
                if (window.BirbNestSDK) {
                    clearInterval(checkSDK);
                    resolve();
                }
            }, 100);
        });
        
        // Initialize SDK client
        state.sdk = new window.BirbNestSDK.Client({
            baseURL: 'http://localhost:8080',
            timeout: 5000,
            retries: document.getElementById('enable-retries').checked ? 3 : 0
        });
        
        updateConnectionStatus('Connected', 'success');
        logOperation('SDK initialized successfully', 'success', '✅');
        
    } catch (error) {
        updateConnectionStatus('Failed to initialize', 'error');
        logOperation(`Initialization error: ${error.message}`, 'error', '❌');
        showToast('Failed to initialize SDK', 'error');
    }
}

// ===== Particle Effects =====
function createParticles() {
    const particlesContainer = document.getElementById('particles');
    const particleCount = 50;
    
    for (let i = 0; i < particleCount; i++) {
        const particle = document.createElement('div');
        particle.className = 'particle';
        particle.style.left = Math.random() * 100 + '%';
        particle.style.animationDelay = Math.random() * 20 + 's';
        particle.style.animationDuration = (Math.random() * 20 + 20) + 's';
        particlesContainer.appendChild(particle);
    }
}

// ===== Event Listeners =====
function setupEventListeners() {
    // Theme toggle
    document.querySelector('.theme-toggle').addEventListener('click', toggleTheme);
    
    // Action buttons
    document.querySelectorAll('.action-btn').forEach(btn => {
        btn.addEventListener('click', () => handleAction(btn.dataset.action));
    });
    
    // Generate buttons
    document.querySelectorAll('.generate-btn').forEach((btn, index) => {
        btn.addEventListener('click', () => {
            if (index === 0) {
                document.getElementById('key-input').value = sampleData.generateKey();
            } else {
                document.getElementById('value-input').value = sampleData.generateValue();
            }
        });
    });
    
    // Modal
    const modal = document.getElementById('help-modal');
    const closeBtn = modal.querySelector('.close');
    closeBtn.addEventListener('click', () => modal.style.display = 'none');
    window.addEventListener('click', (e) => {
        if (e.target === modal) modal.style.display = 'none';
    });
}

// ===== Keyboard Shortcuts =====
function setupKeyboardShortcuts() {
    document.addEventListener('keydown', (e) => {
        // Ignore if typing in input
        if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') {
            return;
        }
        
        switch (e.key.toLowerCase()) {
            case 's':
                handleAction('set');
                break;
            case 'g':
                handleAction('get');
                break;
            case 'd':
                handleAction('delete');
                break;
            case 'b':
                handleAction('batch');
                break;
            case 'c':
                clearLog();
                break;
            case 't':
                toggleTheme();
                break;
            case '?':
                document.getElementById('help-modal').style.display = 'block';
                break;
        }
    });
}

// ===== Theme Management =====
function toggleTheme() {
    state.theme = state.theme === 'light' ? 'dark' : 'light';
    document.body.setAttribute('data-theme', state.theme);
    localStorage.setItem('theme', state.theme);
    updateThemeIcon();
    
    if (state.theme === 'dark') {
        unlockAchievement('nightOwl');
    }
}

function updateThemeIcon() {
    const icon = document.querySelector('.theme-icon');
    icon.textContent = state.theme === 'light' ? '🌙' : '☀️';
}

// ===== Action Handlers =====
async function handleAction(action) {
    const key = document.getElementById('key-input').value.trim();
    const value = document.getElementById('value-input').value.trim();
    const ttl = document.getElementById('ttl-input').value;
    
    if (!state.sdk) {
        showToast('SDK not initialized', 'error');
        return;
    }
    
    // Validate inputs
    if (action !== 'batch' && !key) {
        showToast('Please enter a key', 'warning');
        return;
    }
    
    if ((action === 'set' || action === 'batch') && !value) {
        showToast('Please enter a value', 'warning');
        return;
    }
    
    // Simulate errors if enabled
    if (document.getElementById('simulate-errors').checked && Math.random() < 0.3) {
        handleError(action, new Error('Simulated error'));
        return;
    }
    
    const startTime = performance.now();
    
    try {
        let result;
        
        switch (action) {
            case 'set':
                result = await performSet(key, value, ttl);
                break;
            case 'get':
                result = await performGet(key);
                break;
            case 'delete':
                result = await performDelete(key);
                break;
            case 'batch':
                result = await performBatch();
                break;
        }
        
        const duration = performance.now() - startTime;
        handleSuccess(action, result, duration);
        
    } catch (error) {
        handleError(action, error);
    }
}

// ===== SDK Operations =====
async function performSet(key, value, ttl) {
    const options = {};
    if (ttl) {
        options.ttl = parseInt(ttl);
    }
    
    // Parse value if it's JSON
    let parsedValue;
    try {
        parsedValue = JSON.parse(value);
    } catch {
        parsedValue = value;
    }
    
    await state.sdk.set(key, parsedValue, options);
    return { key, value: parsedValue };
}

async function performGet(key) {
    const value = await state.sdk.get(key);
    if (value !== null) {
        state.cacheHits++;
    } else {
        state.cacheMisses++;
    }
    return { key, value };
}

async function performDelete(key) {
    await state.sdk.delete(key);
    return { key };
}

async function performBatch() {
    const operations = [];
    for (let i = 0; i < 3; i++) {
        operations.push({
            key: `batch:item:${i}`,
            value: { index: i, timestamp: Date.now() }
        });
    }
    
    // Set multiple values
    for (const op of operations) {
        await state.sdk.set(op.key, op.value);
    }
    
    unlockAchievement('batchMaster');
    return { operations };
}

// ===== Success/Error Handling =====
function handleSuccess(action, result, duration) {
    state.operationCount++;
    state.successCount++;
    state.streak++;
    state.totalResponseTime += duration;
    
    // Update score
    let points = 10;
    if (state.streak >= 5) points *= 2;
    if (duration < 50) points += 5;
    updateScore(points);
    
    // Log operation
    const icon = {
        set: '💾',
        get: '📤',
        delete: '🗑️',
        batch: '📦'
    }[action];
    
    let message = `${action.toUpperCase()} operation successful`;
    if (action === 'get' && result.value) {
        message += `: ${JSON.stringify(result.value)}`;
    }
    
    if (document.getElementById('show-timing').checked) {
        message += ` (${duration.toFixed(2)}ms)`;
    }
    
    logOperation(message, 'success', icon);
    
    // Check achievements
    if (state.operationCount === 1) unlockAchievement('firstOperation');
    if (state.streak === 5) unlockAchievement('streak5');
    if (state.streak === 10) unlockAchievement('streak10');
    if (state.operationCount === 100) unlockAchievement('persistent');
    if (duration < 50 && state.successCount % 10 === 0) unlockAchievement('speedDemon');
    
    // Update UI
    updateOperationCount();
    updateStreak();
}

function handleError(action, error) {
    state.failureCount++;
    state.streak = 0;
    
    // Log operation
    logOperation(`${action.toUpperCase()} failed: ${error.message}`, 'error', '❌');
    showToast(`Operation failed: ${error.message}`, 'error');
    
    // Check circuit breaker
    if (state.failureCount % 5 === 0) {
        updateCircuitState('open');
        unlockAchievement('circuitBreaker');
        setTimeout(() => updateCircuitState('half-open'), 5000);
        setTimeout(() => updateCircuitState('closed'), 10000);
    }
    
    // Check error handler achievement
    if (state.failureCount === 10) unlockAchievement('errorHandler');
    
    // Update UI
    updateStreak();
}

// ===== Circuit Breaker Visualization =====
function updateCircuitState(newState) {
    state.circuitState = newState;
    
    const states = document.querySelectorAll('.circuit-state');
    states.forEach(el => el.classList.remove('active'));
    
    const activeState = document.querySelector(`.circuit-state.${newState.replace('-', '-')}`);
    if (activeState) {
        activeState.classList.add('active');
    }
    
    logOperation(`Circuit breaker state: ${newState.toUpperCase()}`, 'warning', '⚡');
}

// ===== UI Updates =====
function updateConnectionStatus(status, type = 'info') {
    const element = document.getElementById('connection-status');
    element.textContent = status;
    element.className = `status-value ${type}`;
}

function updateOperationCount() {
    document.getElementById('operation-count').textContent = state.operationCount;
}

function updateScore(points) {
    state.score += points;
    const scoreElement = document.getElementById('score');
    scoreElement.textContent = state.score;
    
    // Animate score update
    scoreElement.style.transform = 'scale(1.2)';
    setTimeout(() => {
        scoreElement.style.transform = 'scale(1)';
    }, 300);
}

function updateStreak() {
    document.getElementById('streak').textContent = state.streak;
}

function updateMetrics() {
    // Success rate
    const successRate = state.operationCount > 0 
        ? (state.successCount / state.operationCount * 100).toFixed(1) 
        : 100;
    document.getElementById('success-rate').textContent = `${successRate}%`;
    document.getElementById('success-bar').style.width = `${successRate}%`;
    
    // Average response time
    const avgResponse = state.successCount > 0 
        ? (state.totalResponseTime / state.successCount).toFixed(1) 
        : 0;
    document.getElementById('avg-response').textContent = `${avgResponse}ms`;
    const timingPercent = Math.min((avgResponse / 200) * 100, 100);
    document.getElementById('timing-bar').style.width = `${timingPercent}%`;
    
    // Cache hit rate
    const totalCacheOps = state.cacheHits + state.cacheMisses;
    const hitRate = totalCacheOps > 0 
        ? (state.cacheHits / totalCacheOps * 100).toFixed(1) 
        : 0;
    document.getElementById('hit-rate').textContent = `${hitRate}%`;
    document.getElementById('hit-bar').style.width = `${hitRate}%`;
    
    // Check cache expert achievement
    if (hitRate >= 90 && totalCacheOps >= 10) {
        unlockAchievement('cacheExpert');
    }
}

// ===== Logging =====
function logOperation(message, type = 'info', icon = '📝') {
    const container = document.getElementById('operation-log');
    const entry = document.createElement('div');
    entry.className = `log-entry ${type}`;
    
    const time = new Date().toLocaleTimeString();
    entry.innerHTML = `
        <span class="log-time">${time}</span>
        <span class="log-icon">${icon}</span>
        <span class="log-message">${message}</span>
    `;
    
    container.appendChild(entry);
    container.scrollTop = container.scrollHeight;
    
    // Limit log entries
    if (container.children.length > 50) {
        container.removeChild(container.firstChild);
    }
}

function clearLog() {
    const container = document.getElementById('operation-log');
    container.innerHTML = `
        <div class="log-entry welcome">
            <span class="log-time">${new Date().toLocaleTimeString()}</span>
            <span class="log-icon">🦜</span>
            <span class="log-message">Log cleared</span>
        </div>
    `;
    showToast('Log cleared', 'info');
}

// ===== Achievements =====
function unlockAchievement(id) {
    if (state.achievements.has(id)) return;
    
    state.achievements.add(id);
    const achievement = ACHIEVEMENTS[id];
    
    // Add to achievement list
    const list = document.getElementById('achievement-list');
    const element = document.createElement('div');
    element.className = 'achievement unlocked';
    element.innerHTML = `
        <div class="achievement-icon">${achievement.icon}</div>
        <div class="achievement-name">${achievement.name}</div>
    `;
    element.title = achievement.description;
    list.appendChild(element);
    
    // Show toast
    showToast(`🏆 Achievement Unlocked: ${achievement.name}`, 'success');
    
    // Award bonus points
    updateScore(50);
}

// ===== Toast Notifications =====
function showToast(message, type = 'info') {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    
    const icon = {
        success: '✅',
        error: '❌',
        warning: '⚠️',
        info: 'ℹ️'
    }[type];
    
    toast.innerHTML = `
        <span class="toast-icon">${icon}</span>
        <span class="toast-message">${message}</span>
    `;
    
    container.appendChild(toast);
    
    // Auto remove after 3 seconds
    setTimeout(() => {
        toast.style.animation = 'slideInToast 0.3s ease-out reverse';
        setTimeout(() => toast.remove(), 300);
    }, 3000);
}

// ===== Initialize on DOM ready =====
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initializeDemo);
} else {
    initializeDemo();
}

// ===== Export for debugging =====
window.BirbNestDemo = {
    state,
    showToast,
    unlockAchievement,
    logOperation
};
