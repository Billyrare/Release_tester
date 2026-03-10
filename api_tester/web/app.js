// API Host
const API_HOST = 'http://localhost:8080';

// ========== ТЁМНАЯ ТЕМА ==========
(function initTheme() {
    const saved = localStorage.getItem('theme');
    if (saved === 'dark') {
        document.body.classList.add('dark');
        document.addEventListener('DOMContentLoaded', () => {
            const btn = document.getElementById('themeToggle');
            if (btn) btn.textContent = '☀️ Светлая тема';
        });
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

// ========== ПЕРЕКЛЮЧЕНИЕ ВКЛАДОК ==========
function switchTab(tabName, evt) {
    document.querySelectorAll('.tab-content').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
    document.getElementById(tabName).classList.add('active');
    (evt ? evt.target : event.target).classList.add('active');
}

// ========== ПРОГРЕСС ШАГОВ WORKFLOW ==========
let _lastCodesForAggregation = [];

function showWorkflowProgress(show) {
    const steps = document.getElementById('workflowSteps');
    if (steps) steps.style.display = show ? 'block' : 'none';
    if (show) {
        ['step-order', 'step-wait', 'step-utilisation', 'step-done'].forEach(id => {
            const el = document.getElementById(id);
            if (el) el.className = 'step';
        });
    }
}

function setStep(stepId) {
    const steps = ['step-order', 'step-wait', 'step-utilisation', 'step-done'];
    const idx = steps.indexOf(stepId);
    steps.forEach((id, i) => {
        const el = document.getElementById(id);
        if (!el) return;
        if (i < idx) el.className = 'step done';
        else if (i === idx) el.className = 'step active';
        else el.className = 'step';
    });
}

// ========== БЫСТРЫЙ ЦИКЛ (ExecuteWorkflow) ==========
async function executeWorkflow() {
    const gtin = document.getElementById('executeGtin').value;
    const group = document.getElementById('executeGroup').value;
    const quantity = parseInt(document.getElementById('executeQuantity').value);
    const expirationDays = parseInt(document.getElementById('executeExpirationDays').value);

    if (!gtin || !group || !quantity) {
        showError('Заполните все обязательные поля');
        return;
    }

    const payload = { gtin, productGroup: group, quantity, expirationDays };

    try {
        showLoading(true, true);
        setStep('step-order');

        // Имитируем показ шагов через задержки (сервер блокирующий, шаги не real-time)
        const stepTimer1 = setTimeout(() => setStep('step-wait'), 1500);
        const stepTimer2 = setTimeout(() => setStep('step-utilisation'), 5000);

        const response = await fetch(`${API_HOST}/v1/workflow/execute`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });

        clearTimeout(stepTimer1);
        clearTimeout(stepTimer2);
        setStep('step-done');
        await new Promise(r => setTimeout(r, 400));

        const data = await response.json();
        showLoading(false);

        if (!response.ok) {
            showError(`Ошибка: ${data.error || response.statusText}`);
            showResult(JSON.stringify(data, null, 2), 'error');
        } else {
            _lastCodesForAggregation = data.codes_for_aggregation || [];
            showSuccess('Цикл выполнен успешно!');
            showResult(JSON.stringify(data, null, 2), 'success');
            showDownloadBtn(_lastCodesForAggregation.length > 0);
            loadHistory();
            loadCodeFiles();
        }
    } catch (error) {
        showLoading(false);
        showError(`Ошибка подключения: ${error.message}`);
        showResult(error.message, 'error');
    }
}

// ========== ОТЧЕТ ОБ АГРЕГАЦИИ ==========
async function reportAggregation() {
    const businessPlaceId = parseInt(document.getElementById('aggBusinessPlaceId').value);
    const packageCount = parseInt(document.getElementById('aggPackageCount').value);
    const codesStr = document.getElementById('aggCodes').value;
    const serialNumber = document.getElementById('aggSerialNumber').value;

    if (!businessPlaceId || !packageCount || !codesStr) {
        showError('Заполните все обязательные поля');
        return;
    }

    const codes = codesStr.split(',').map(c => c.trim()).filter(c => c);
    const aggregationUnits = [];
    for (let i = 0; i < packageCount; i++) {
        const perPack = Math.ceil(codes.length / packageCount);
        const unitCodes = codes.slice(i * perPack, (i + 1) * perPack);
        aggregationUnits.push({
            aggregationItemsCount: unitCodes.length,
            aggregationUnitCapacity: codes.length,
            codes: unitCodes,
            shouldBeUnbundled: false,
            unitSerialNumber: serialNumber || `SSCC_${i}`
        });
    }

    const payload = {
        aggregationUnits,
        businessPlaceId,
        documentDate: new Date().toISOString(),
        productionOrderId: ""
    };

    try {
        showLoading(true);
        const response = await fetch(`${API_HOST}/v1/workflow/report-aggregation`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        const data = await response.json();
        showLoading(false);

        if (!response.ok) {
            showError(`Ошибка: ${data.error || response.statusText}`);
            showResult(JSON.stringify(data, null, 2), 'error');
        } else {
            showSuccess(`Отчет подан успешно! DocumentId: ${data.document_id}`);
            showResult(JSON.stringify(data, null, 2), 'success');
            loadHistory();
        }
    } catch (error) {
        showLoading(false);
        showError(`Ошибка подключения: ${error.message}`);
        showResult(error.message, 'error');
    }
}

// ========== ПОЛНЫЙ ЦИКЛ (CompleteWorkflow) ==========
async function completeWorkflow() {
    const gtin = document.getElementById('completeGtin').value;
    const group = document.getElementById('completeGroup').value;
    const quantity = parseInt(document.getElementById('completeQuantity').value);
    const businessPlaceId = parseInt(document.getElementById('completeBusinessPlaceId').value);
    const productionOrderId = document.getElementById('completeProductionOrderId').value;

    if (!gtin || !group || !quantity || !businessPlaceId) {
        showError('Заполните все обязательные поля');
        return;
    }

    const payload = { gtin, productGroup: group, quantity, businessPlaceId, productionOrderId: productionOrderId || "", expirationDays: 365 };

    try {
        showLoading(true, true);
        setStep('step-order');
        const t1 = setTimeout(() => setStep('step-wait'), 1500);
        const t2 = setTimeout(() => setStep('step-utilisation'), 5000);

        const response = await fetch(`${API_HOST}/v1/workflow/complete`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });

        clearTimeout(t1); clearTimeout(t2);
        setStep('step-done');
        await new Promise(r => setTimeout(r, 400));

        const data = await response.json();
        showLoading(false);

        if (!response.ok) {
            showError(`Ошибка: ${data.error || response.statusText}`);
            showResult(JSON.stringify(data, null, 2), 'error');
        } else {
            _lastCodesForAggregation = data.codes_for_aggregation || [];
            showSuccess('Полный цикл выполнен!');
            showResult(JSON.stringify(data, null, 2), 'success');
            showDownloadBtn(_lastCodesForAggregation.length > 0);
            loadHistory();
            loadCodeFiles();
        }
    } catch (error) {
        showLoading(false);
        showError(`Ошибка подключения: ${error.message}`);
        showResult(error.message, 'error');
    }
}

// ========== СКАЧАТЬ КОДЫ ИЗ РЕЗУЛЬТАТА ==========
function downloadCodesFromResult() {
    if (!_lastCodesForAggregation || _lastCodesForAggregation.length === 0) {
        showError('Нет кодов для скачивания');
        return;
    }
    const content = _lastCodesForAggregation.join('\n');
    const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `codes_${Date.now()}.txt`;
    a.click();
    URL.revokeObjectURL(url);
}

function showDownloadBtn(show) {
    const btn = document.getElementById('downloadCodesBtn');
    if (btn) btn.style.display = show ? 'block' : 'none';
}

// ========== СПИСОК ФАЙЛОВ КОДОВ ==========
async function loadCodeFiles() {
    const container = document.getElementById('codeFiles');
    try {
        const response = await fetch(`${API_HOST}/v1/codes/files`);
        if (!response.ok) {
            container.innerHTML = `<p class="placeholder">⚠️ Недоступно</p>`;
            return;
        }
        const data = await response.json();
        const files = data.files || [];
        if (files.length === 0) {
            container.innerHTML = `<p class="placeholder">Нет сохранённых файлов</p>`;
            return;
        }
        container.innerHTML = files.map(f => {
            const kb = (f.size / 1024).toFixed(1);
            const dt = new Date(f.created_at).toLocaleString('ru-RU');
            return `<div class="history-item success" style="display:flex;justify-content:space-between;align-items:center;">
                <div>
                    <div class="history-op" style="word-break:break-all;">${f.name}</div>
                    <div class="history-time">${dt} · ${kb} KB</div>
                </div>
                <a href="${API_HOST}/v1/codes/files/${encodeURIComponent(f.name)}" download="${f.name}"
                   style="text-decoration:none;">
                    <button class="secondary-btn" style="margin:0;">⬇️</button>
                </a>
            </div>`;
        }).join('');
    } catch (e) {
        container.innerHTML = `<p class="placeholder">❌ Ошибка: ${e.message}</p>`;
    }
}

// ========== ИСТОРИЯ ==========
let _allHistory = [];

async function loadHistory() {
    try {
        const response = await fetch(`${API_HOST}/v1/marking/history`);
        
        if (!response.ok) {
            document.getElementById('history').innerHTML = `<p class="placeholder">⚠️ История недоступна</p>`;
            return;
        }

        const data = await response.json();
        _allHistory = data || [];
        renderHistory(_allHistory);
    } catch (error) {
        document.getElementById('history').innerHTML = `<p class="placeholder">❌ Ошибка загрузки: ${error.message}</p>`;
    }
}

function filterHistory() {
    const q = (document.getElementById('historyFilter').value || '').toLowerCase();
    if (!q) {
        renderHistory(_allHistory);
        return;
    }
    const filtered = _allHistory.filter(item =>
        (item.operation_type || '').toLowerCase().includes(q) ||
        (item.product_group || '').toLowerCase().includes(q) ||
        (item.external_id || '').toLowerCase().includes(q) ||
        (item.status || '').toLowerCase().includes(q)
    );
    renderHistory(filtered);
}

function renderHistory(items) {
    if (!items || items.length === 0) {
        document.getElementById('history').innerHTML = `<p class="placeholder">Нет записей</p>`;
        return;
    }
    let html = '';
    items.slice(0, 30).forEach(item => {
        const time = new Date(item.created_at).toLocaleString('ru-RU');
        const statusClass = (item.status || '').toUpperCase() === 'SUCCESS' ? 'success' : 'error';
        html += `
            <div class="history-item ${statusClass}">
                <div class="history-time">⏰ ${time}</div>
                <div class="history-op">${item.operation_type} (${item.product_group})</div>
                <div style="color:var(--text-muted); font-size:0.85em; margin-top:3px;">ID: ${item.external_id || 'N/A'}</div>
            </div>
        `;
    });
    document.getElementById('history').innerHTML = html;
}

// ========== СТАТУС API ==========
async function checkApiStatus() {
    try {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 3000);
        const response = await fetch(`${API_HOST}/health`, { signal: controller.signal });
        clearTimeout(timeoutId);

        const statusElement = document.getElementById('apiStatus');
        if (response.ok) {
            statusElement.innerHTML = `<div class="status-content online">✅ API Online<div style="margin-top:10px;font-size:0.9em;">${API_HOST}</div></div>`;
        } else {
            statusElement.innerHTML = `<div class="status-content offline">❌ API Offline (${response.status})</div>`;
        }
    } catch (error) {
        document.getElementById('apiStatus').innerHTML = `<div class="status-content offline">❌ API Недоступен<br><small>${error.message}</small></div>`;
    }
}

// ========== UI ФУНКЦИИ ==========
function showResult(content, type = 'normal') {
    const resultDiv = document.getElementById('result');
    resultDiv.innerHTML = content;
    resultDiv.className = 'result-output ' + (type === 'error' ? 'error' : type === 'success' ? 'success' : '');
}

function showError(message) {
    const modal = document.getElementById('errorModal');
    document.getElementById('errorMessage').textContent = message;
    modal.classList.add('show');
}

function showSuccess(message) {
    const modal = document.getElementById('successModal');
    document.getElementById('successMessage').textContent = message;
    modal.classList.add('show');
}

function showLoading(show, withSteps = false) {
    const spinner = document.getElementById('loadingSpinner');
    const text = document.getElementById('loadingText');
    if (show) {
        spinner.style.display = 'flex';
        if (withSteps) {
            if (text) text.textContent = 'Выполнение workflow...';
            showWorkflowProgress(true);
        } else {
            if (text) text.textContent = 'Обработка...';
            showWorkflowProgress(false);
        }
    } else {
        spinner.style.display = 'none';
        showWorkflowProgress(false);
    }
}

function closeModal() {
    document.getElementById('errorModal').classList.remove('show');
    document.getElementById('successModal').classList.remove('show');
}

// ========== ИНИЦИАЛИЗАЦИЯ ==========
document.addEventListener('DOMContentLoaded', function() {
    // Применить сохранённую тему
    const savedTheme = localStorage.getItem('theme');
    const btn = document.getElementById('themeToggle');
    if (savedTheme === 'dark' && btn) btn.textContent = '☀️ Светлая тема';

    document.getElementById('executeGtin').focus();
    checkApiStatus();
    setInterval(checkApiStatus, 10000);
    loadHistory();
    loadCodeFiles();
    setInterval(loadHistory, 5000);

    window.onclick = function(event) {
        const errorModal = document.getElementById('errorModal');
        const successModal = document.getElementById('successModal');
        if (event.target === errorModal) errorModal.classList.remove('show');
        if (event.target === successModal) successModal.classList.remove('show');
    };
});
