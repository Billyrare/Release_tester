const API_HOST = 'http://localhost:8080';

// ========== ТЕМА ==========
(function initTheme() {
    const saved = localStorage.getItem('theme');
    if (saved === 'dark') {
        document.body.classList.add('dark');
        const btn = document.getElementById('themeToggle');
        if (btn) btn.textContent = '☀️ Светлая тема';
    }
})();

function toggleTheme() {
    const body = document.body;
    const btn = document.getElementById('themeToggle');
    if (body.classList.contains('dark')) {
        body.classList.remove('dark');
        localStorage.setItem('theme', 'light');
        btn.textContent = '🌙 Тёмная тема';
    } else {
        body.classList.add('dark');
        localStorage.setItem('theme', 'dark');
        btn.textContent = '☀️ Светлая тема';
    }
}

// ========== ВКЛАДКИ ==========
function switchTab(tabName, evt) {
    document.querySelectorAll('.tab-content').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
    document.getElementById(tabName).classList.add('active');
    (evt ? evt.target : event.target).classList.add('active');
}

// ========== GTIN LOADER ==========
let productCardsCache = {};

async function updateGtinOptions() {
    const productGroup = document.getElementById('wfProductGroup').value;
    const gtinSelect = document.getElementById('wfGtin');

    gtinSelect.innerHTML = '<option value="">-- Выбрать GTIN --</option>';

    if (!productGroup) return;

    try {
        if (!productCardsCache[productGroup]) {
            const resp = await fetch(`${API_HOST}/v1/marking/product-cards?productGroup=${productGroup}`);
            if (!resp.ok) throw new Error(`Status ${resp.status}`);
            const data = await resp.json();
            productCardsCache[productGroup] = data.cards || [];
        }

        productCardsCache[productGroup].forEach(card => {
            const opt = document.createElement('option');
            opt.value = card.gtin;
            opt.textContent = `${card.gtin} - ${card.productName?.ru?.substring(0, 50) || 'N/A'}`;
            gtinSelect.appendChild(opt);
        });
    } catch (error) {
        showError('Ошибка загрузки GTIN: ' + error.message);
    }
}

// ========== WORKFLOW ==========
let isOperationRunning = false;
const estimatedDurations = { 'workflow': 30000, 'orders': 60000, 'utilisations': 45000, 'aggregations': 35000, 'full': 180000 };
let operationStartTime = 0;

function disableAllButtons(disable) {
    document.querySelectorAll('button').forEach(btn => {
        btn.disabled = disable;
    });
}

function updateProgressBar(progressBar, progressText, currentTime, startTime, estimatedDuration) {
    const elapsed = currentTime - startTime;
    const progress = Math.min(90, (elapsed / estimatedDuration) * 100);
    progressBar.style.width = progress + '%';

    const remaining = Math.max(0, estimatedDuration - elapsed);
    const remainingSeconds = Math.ceil(remaining / 1000);
    const minutes = Math.floor(remainingSeconds / 60);
    const seconds = remainingSeconds % 60;

    let timeText = '';
    if (minutes > 0) timeText = `${minutes}м ${seconds}с`;
    else timeText = `${seconds}с`;

    progressText.textContent = `⏳ Процесс выполняется... ~${timeText} осталось (${Math.round(progress)}%)`;
}

async function runWorkflow() {
    const productGroup = document.getElementById('wfProductGroup').value;
    const gtin = document.getElementById('wfGtin').value;
    const quantity = parseInt(document.getElementById('wfQuantity').value);
    const businessPlaceId = parseInt(document.getElementById('wfBusinessPlaceId').value);
    const expirationDays = parseInt(document.getElementById('wfExpirationDays').value);

    if (!productGroup || !gtin) {
        showError('Выберите группу товара и GTIN');
        return;
    }

    if (isOperationRunning) {
        showError('Операция уже выполняется, пожалуйста подождите');
        return;
    }

    isOperationRunning = true;
    disableAllButtons(true);

    const progressDiv = document.getElementById('workflowProgress');
    const progressBar = document.getElementById('wfProgressBar');
    const progressText = document.getElementById('wfProgressText');

    progressDiv.style.display = 'block';
    progressBar.style.width = '0%';
    progressText.textContent = 'Запуск workflow...';

    operationStartTime = Date.now();
    const estimatedDuration = estimatedDurations['workflow'];
    const progressInterval = setInterval(() => {
        if (isOperationRunning && progressBar.style.width !== '100%') {
            updateProgressBar(progressBar, progressText, Date.now(), operationStartTime, estimatedDuration);
        }
    }, 200);

    try {
        const resp = await fetch(`${API_HOST}/v1/workflow/execute`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                gtin, productGroup, quantity, businessPlaceId, expirationDays
            })
        });

        const data = await resp.json();
        clearInterval(progressInterval);

        if (!resp.ok) {
            progressBar.style.width = '100%';
            progressText.textContent = '❌ Ошибка выполнения';
            showError(data.error || 'Ошибка выполнения workflow');
            isOperationRunning = false;
            disableAllButtons(false);
            return;
        }

        progressBar.style.width = '100%';
        progressText.textContent = '✅ Успешно завершено за ' + Math.round((Date.now() - operationStartTime) / 1000) + 'с';

        showResult({
            title: '✅ Workflow успешно завершён',
            reportId: data.reportId,
            codesCount: data.codesForAggregation?.length || 0
        });

        setTimeout(() => {
            progressDiv.style.display = 'none';
            loadHistory();
            isOperationRunning = false;
            disableAllButtons(false);
        }, 2000);
    } catch (error) {
        clearInterval(progressInterval);
        progressBar.style.width = '100%';
        progressText.textContent = '❌ Ошибка сети';
        showError('Ошибка: ' + error.message);
        isOperationRunning = false;
        disableAllButtons(false);
    }
}

// ========== ТЕСТЫ ==========
async function runTests(suiteType) {
    const endpoints = {
        'orders': '/v1/test/orders-suite',
        'utilisations': '/v1/test/utilisations-suite',
        'aggregations': '/v1/test/aggregations-suite',
        'full': '/v1/test/full-suite'
    };

    const endpoint = endpoints[suiteType];
    if (!endpoint) {
        showError('Неизвестный тип теста');
        return;
    }

    if (isOperationRunning) {
        showError('Операция уже выполняется, пожалуйста подождите');
        return;
    }

    isOperationRunning = true;
    disableAllButtons(true);

    const progressDiv = document.getElementById('testProgressPanel');
    const progressBar = document.getElementById('testProgressBar');
    const progressText = document.getElementById('testProgressText');

    progressDiv.style.display = 'block';
    progressBar.style.width = '0%';
    progressText.textContent = `Запуск ${suiteType} тестов...`;

    operationStartTime = Date.now();
    const estimatedDuration = estimatedDurations[suiteType] || 60000;
    const progressInterval = setInterval(() => {
        if (isOperationRunning && progressBar.style.width !== '100%') {
            updateProgressBar(progressBar, progressText, Date.now(), operationStartTime, estimatedDuration);
        }
    }, 200);

    try {
        const resp = await fetch(`${API_HOST}${endpoint}`, { method: 'POST' });
        const data = await resp.json();

        clearInterval(progressInterval);

        if (!resp.ok) {
            progressBar.style.width = '100%';
            progressText.textContent = '❌ Ошибка выполнения';
            showError(data.error || 'Ошибка выполнения тестов');
            isOperationRunning = false;
            disableAllButtons(false);
            return;
        }

        progressBar.style.width = '100%';
        const actualDuration = Math.round((Date.now() - operationStartTime) / 1000);
        progressText.textContent = `✅ Завершено за ${actualDuration}с: ${data.passed}/${data.total} успешно, ${data.failed} ошибок`;

        showResult({
            title: `📊 Результаты ${suiteType}`,
            total: data.total,
            passed: data.passed,
            failed: data.failed,
            duration: actualDuration + 'сек'
        });

        setTimeout(() => {
            progressDiv.style.display = 'none';
            loadHistory();
            isOperationRunning = false;
            disableAllButtons(false);
        }, 3000);
    } catch (error) {
        clearInterval(progressInterval);
        progressBar.style.width = '100%';
        progressText.textContent = '❌ Ошибка сети';
        showError('Ошибка: ' + error.message);
        isOperationRunning = false;
        disableAllButtons(false);
    }
}

// ========== ИСТОРИЯ ==========
let allHistory = [];

async function loadHistory() {
    try {
        const resp = await fetch(`${API_HOST}/v1/operations/history?limit=50`);
        const data = await resp.json();
        allHistory = data.operations || [];
        displayHistory(allHistory);
    } catch (error) {
        console.error('Ошибка загрузки истории:', error);
        document.getElementById('history').innerHTML = '<p class="placeholder">Ошибка загрузки</p>';
    }
}

function displayHistory(ops) {
    const historyDiv = document.getElementById('history');
    if (!ops || ops.length === 0) {
        historyDiv.innerHTML = '<p class="placeholder">История пуста</p>';
        return;
    }

    historyDiv.innerHTML = ops.map(op => `
        <div class="history-item" style="padding:10px; border-bottom:1px solid var(--border);">
            <div style="display:flex; justify-content:space-between; align-items:center;">
                <strong>${op.operationType} - ${op.productGroup}</strong>
                <span style="color:var(--text-muted); font-size:0.8em;">${new Date(op.timestamp).toLocaleString('ru-RU')}</span>
            </div>
            <p style="margin:4px 0; color:var(--text-muted); font-size:0.85em;">${op.details}</p>
        </div>
    `).join('');
}

function filterHistory() {
    const query = document.getElementById('historyFilter').value.toLowerCase();
    const filtered = allHistory.filter(op =>
        op.operationType.toLowerCase().includes(query) ||
        op.productGroup.toLowerCase().includes(query) ||
        op.details.toLowerCase().includes(query)
    );
    displayHistory(filtered);
}

// ========== РЕЗУЛЬТАТЫ ==========
function showResult(data) {
    const resultDiv = document.getElementById('result');
    let html = `<div style="padding:15px; background:var(--surface); border-radius:8px;">`;

    if (data.title) html += `<h3>${data.title}</h3>`;

    if (data.reportId) html += `<p><strong>Report ID:</strong> ${data.reportId}</p>`;
    if (data.codesCount) html += `<p><strong>Кодов:</strong> ${data.codesCount}</p>`;
    if (data.total) html += `<p><strong>Тестов:</strong> ${data.total}</p>`;
    if (data.passed) html += `<p style="color:green;"><strong>✅ Пройдено:</strong> ${data.passed}</p>`;
    if (data.failed) html += `<p style="color:red;"><strong>❌ Ошибок:</strong> ${data.failed}</p>`;
    if (data.duration) html += `<p><strong>Время:</strong> ${data.duration}</p>`;

    html += '</div>';
    resultDiv.innerHTML = html;

    if (data.codesCount > 0) {
        document.getElementById('downloadBtn').style.display = 'block';
    }
}

function downloadCodesFromResult() {
    showSuccess('Скачивание кодов в разработке');
}

// ========== МОДАЛКИ ==========
function showError(msg) {
    document.getElementById('errorMessage').textContent = msg;
    document.getElementById('errorModal').style.display = 'flex';
}

function showSuccess(msg) {
    document.getElementById('successMessage').textContent = msg;
    document.getElementById('successModal').style.display = 'flex';
}

function closeModal() {
    document.getElementById('errorModal').style.display = 'none';
    document.getElementById('successModal').style.display = 'none';
}

// ========== ИНИЦИАЛИЗАЦИЯ ==========
document.addEventListener('DOMContentLoaded', () => {
    loadHistory();

    // Проверка статуса API
    fetch(`${API_HOST}/health`)
        .then(r => {
            document.getElementById('apiStatus').innerHTML = '<p style="color:green;">✅ API доступен</p>';
        })
        .catch(() => {
            document.getElementById('apiStatus').innerHTML = '<p style="color:red;">❌ API недоступен</p>';
        });
});

// Закрытие модалей при нажатии вне
window.onclick = (event) => {
    const errorModal = document.getElementById('errorModal');
    const successModal = document.getElementById('successModal');
    if (event.target === errorModal) errorModal.style.display = 'none';
    if (event.target === successModal) successModal.style.display = 'none';
};
