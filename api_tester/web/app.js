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

// ========== ЗАГРУЗКА GTIN ПРИ ВЫБОРЕ ТОВАРНОЙ ГРУППЫ ==========
let _cachedProductCards = {};

async function loadProductCards(productGroup) {
    if (!productGroup) {
        return [];
    }
    
    // Проверяем кэш
    if (_cachedProductCards[productGroup]) {
        return _cachedProductCards[productGroup];
    }
    
    try {
        const response = await fetch(`${API_HOST}/v1/marking/product-cards?productGroup=${productGroup}`);
        if (!response.ok) {
            console.error('Ошибка загрузки карточек:', response.status);
            return [];
        }
        const data = await response.json();
        const cards = data.cards || [];
        _cachedProductCards[productGroup] = cards;
        return cards;
    } catch (error) {
        console.error('Ошибка загрузки карточек товаров:', error);
        return [];
    }
}

function onProductGroupChange(selectElement) {
    const productGroup = selectElement.value;
    const gtinSelect = document.getElementById('orderGtin');
    
    if (!gtinSelect) return;
    
    // Очищаем текущий список
    gtinSelect.innerHTML = '<option value="">-- Выберите GTIN --</option>';
    
    if (!productGroup) {
        return;
    }
    
    // Показываем индикатор загрузки
    gtinSelect.innerHTML = '<option value="">Загрузка...</option>';
    
    // Загружаем карточки
    loadProductCards(productGroup).then(cards => {
        gtinSelect.innerHTML = '<option value="">-- Выберите GTIN --</option>';
        
        if (cards.length === 0) {
            gtinSelect.innerHTML += '<option value="" disabled>Нет доступных карточек</option>';
            return;
        }
        
        cards.forEach(card => {
            const option = document.createElement('option');
            option.value = card.gtin;
            
            // Формируем краткое описание
            let description = card.gtin;
            if (card.productName && card.productName.ru) {
                description = card.productName.ru.substring(0, 60);
                if (card.productName.ru.length > 60) description += '...';
            } else if (card.productName && card.productName.en) {
                description = card.productName.en.substring(0, 60);
                if (card.productName.en.length > 60) description += '...';
            }
            
            option.textContent = `${card.gtin} - ${description}`;
            gtinSelect.appendChild(option);
        });
    });
}

// Для workflow вкладок (execute, complete)
function onProductGroupChangeForWorkflow(selectElement, workflowType) {
    const productGroup = selectElement.value;
    const gtinSelect = document.getElementById(workflowType + 'Gtin');
    
    if (!gtinSelect) return;
    
    // Очищаем текущий список
    gtinSelect.innerHTML = '<option value="">-- Выберите GTIN --</option>';
    
    if (!productGroup) {
        return;
    }
    
    // Показываем индикатор загрузки
    gtinSelect.innerHTML = '<option value="">Загрузка...</option>';
    
    // Загружаем карточки
    loadProductCards(productGroup).then(cards => {
        gtinSelect.innerHTML = '<option value="">-- Выберите GTIN --</option>';
        
        if (cards.length === 0) {
            gtinSelect.innerHTML += '<option value="" disabled>Нет доступных карточек</option>';
            return;
        }
        
        cards.forEach(card => {
            const option = document.createElement('option');
            option.value = card.gtin;
            
            // Формируем краткое описание
            let description = card.gtin;
            if (card.productName && card.productName.ru) {
                description = card.productName.ru.substring(0, 60);
                if (card.productName.ru.length > 60) description += '...';
            } else if (card.productName && card.productName.en) {
                description = card.productName.en.substring(0, 60);
                if (card.productName.en.length > 60) description += '...';
            }
            
            option.textContent = `${card.gtin} - ${description}`;
            gtinSelect.appendChild(option);
        });
    });
}

// Глобальная переменная для хранения кодов из файла
let _loadedCodesFromFile = null;

// Загрузка кодов из файла для агрегации
function loadCodesFromFile(input) {
    const file = input.files[0];
    if (!file) return;
    
    const reader = new FileReader();
    reader.onload = function(e) {
        const content = e.target.result;
        // Сохраняем коды в память, НЕ показываем в поле
        _loadedCodesFromFile = content;
        // Показываем только количество загруженных кодов
        const lines = content.split(/\n/).filter(l => l.trim().length > 0);
        alert(`Загружено ${lines.length} кодов из файла`);
    };
    reader.onerror = function() {
        showError('Ошибка чтения файла');
    };
    reader.readAsText(file);
}

// ========== ЗАКАЗ КОДОВ (Создание заказа + проверка статуса) ==========
async function createOrderAndCheck() {
    const productGroup = document.getElementById('orderGroup').value;
    const businessPlaceId = parseInt(document.getElementById('orderBusinessPlaceId').value);
    const releaseMethodType = document.getElementById('orderReleaseMethodType').value;
    const isPaid = document.getElementById('orderIsPaid').value === 'true';
    const gtin = document.getElementById('orderGtin').value;
    const quantity = parseInt(document.getElementById('orderQuantity').value);
    const serialNumberType = document.getElementById('orderSerialNumberType').value;
    const cisType = document.getElementById('orderCisType').value;
    const expirationDays = parseInt(document.getElementById('orderExpirationDays').value);

    if (!productGroup || !gtin || !quantity) {
        showError('Заполните обязательные поля: Группа товара, GTIN, Количество');
        return;
    }

    const payload = {
        productGroup,
        businessPlaceId: businessPlaceId || 1,
        releaseMethodType,
        isPaid,
        products: [
            {
                gtin,
                quantity,
                serialNumberType,
                cisType
            }
        ],
        expirationDays: expirationDays || 365
    };

    try {
        showLoading(true);
        
        // Шаг 1: Создаём заказ
        const createResponse = await fetch(`${API_HOST}/v1/marking/orders`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        
        const createData = await createResponse.json();
        
        if (!createResponse.ok) {
            showLoading(false);
            showError(`Ошибка создания заказа: ${createData.error || createResponse.statusText}`);
            showResult('=== СОЗДАНИЕ ЗАКАЗА ===\n' + JSON.stringify(createData, null, 2), 'error');
            return;
        }
        
        // Получаем ID заказа
        const orderId = createData.orderId || createData.order_id || createData.id;
        if (!orderId) {
            showLoading(false);
            showError('Не удалось получить ID заказа из ответа');
            showResult('=== СОЗДАНИЕ ЗАКАЗА ===\n' + JSON.stringify(createData, null, 2), 'error');
            return;
        }
        
        // // Шаг 2: Проверяем статус заказа
        // const statusResponse = await fetch(`${API_HOST}/v1/marking/orders?orderId=${orderId}`);
        // const statusData = await statusResponse.json();
        
        // showLoading(false);
        
        // // Формируем результат
        // let resultText = '=== СОЗДАНИЕ ЗАКАЗА ===\n';
        // resultText += JSON.stringify(createData, null, 2) + '\n\n';
        // resultText += '=== ПРОВЕРКА СТАТУСА ЗАКАЗА ===\n';
        // resultText += 'OrderId: ' + orderId + '\n';
        // resultText += JSON.stringify(statusData, null, 2);
        // После получения orderId:
const panel = document.getElementById('orderStatusPanel');
const liveDiv = document.getElementById('orderStatusLive');
panel.style.display = 'block';
liveDiv.innerHTML = '<p>Ожидание статуса...</p>';

window._orderPollInterval = setInterval(async () => {
    try {
        const res = await fetch(`${API_HOST}/v1/marking/sub-orders?orderId=${orderId}&gtin=${gtin}`);
        if (!res.ok) {
            liveDiv.innerHTML += `<p style="color:red;">Ошибка HTTP: ${res.status}</p>`;
            return;
        }
        const data = await res.json();
        console.log('SubOrders response:', data);

        if (data.error) {
            liveDiv.innerHTML = `<p style="color:red;">Ошибка API: ${data.error}</p>`;
            clearInterval(window._orderPollInterval);
            return;
        }

        const info = data.subOrderInfos?.[0];
        if (info) {
            liveDiv.innerHTML = `
                <p>Статус буфера: <b>${info.bufferStatus}</b></p>
                <p>Кодов в буфере: ${info.leftInBuffer}</p>
                <p>Заказано: ${quantity}</p>
            `;
            if (['READY', 'EXHAUSTED', 'ACTIVE'].includes(info.bufferStatus)) {
                clearInterval(window._orderPollInterval);
                liveDiv.innerHTML += `<p style="color:green;"><b>✅ Коды готовы!</b></p>`;
            }
            if (info.bufferStatus === 'REJECTED') {
                clearInterval(window._orderPollInterval);
                liveDiv.innerHTML += `<p style="color:red;"><b>❌ Заказ отклонен: ${info.rejectionReason || 'нет причины'}</b></p>`;
            }
        } else {
            liveDiv.innerHTML = `<p>Ожидание подзаказа... (${new Date().toLocaleTimeString()})</p>`;
        }
    } catch(e) {
        liveDiv.innerHTML = `<p style="color:red;">Ошибка запроса: ${e.message}</p>`;
        console.error('Polling error:', e);
    }
}, 2000);

        showLoading(false);
        showSuccess(`Заказ создан! ID: ${orderId}. Отслеживание статуса...`);
        showResult('=== СОЗДАНИЕ ЗАКАЗА ===\n' + JSON.stringify(createData, null, 2) + '\n\nЗаказ: ' + orderId + '\nСтатус: отслеживается...', 'success');

        loadHistory();
    } catch (error) {
        showLoading(false);
        showError(`Ошибка подключения: ${error.message}`);
        showResult(error.message, 'error');
    }
}

function stopOrderPolling() {
    clearInterval(window._orderPollInterval);
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
    let packageCount = parseInt(document.getElementById('aggPackageCount').value);
    if (!packageCount || packageCount < 1) {
        packageCount = 1; // По умолчанию 1 упаковка (1 SSCC)
    }
    // Используем коды из файла если загружены, иначе из поля ввода
    let codesStr = _loadedCodesFromFile || document.getElementById('aggCodes').value;
    const serialNumber = document.getElementById('aggSerialNumber').value;

    if (!businessPlaceId || !packageCount || !codesStr) {
        showError('Заполните все обязательные поля');
        return;
    }

    // Разделяем коды ТОЛЬКО по переносу строки (не по спецсимволам!)
    let codes = codesStr.split(/\n/).map(c => c.trim()).filter(c => c && c.length > 0);
    
    // Убираем возможные BOM-символы
    codes = codes.map(c => c.replace(/^[\uFEFF]/, '')).filter(c => c.length > 0);
    
    console.log('Разобрано кодов:', codes.length);
    if (codes.length > 0) console.log('Первый код:', codes[0], 'длина:', codes[0].length);
    if (codes.length > 1) console.log('Второй код:', codes[1], 'длина:', codes[1].length);
    
    // Если кодов 0 - ошибка
    if (codes.length === 0) {
        showError('Введите хотя бы один код');
        return;
    }
    
    const aggregationUnits = [];
    const codesPerPack = Math.floor(codes.length / packageCount);
    const extraCodes = codes.length % packageCount;
    
    let codeIndex = 0;
    for (let i = 0; i < packageCount; i++) {
        // Распределяем коды: первым extraCodes упаковкам даём на 1 код больше
        const numCodes = codesPerPack + (i < extraCodes ? 1 : 0);
        const unitCodes = codes.slice(codeIndex, codeIndex + numCodes);
        codeIndex += numCodes;
        
        // Генерируем уникальный SSCC для каждой упаковки
        let sscc = '';
        if (serialNumber) {
            // Если указан базовый SSCC, добавляем индекс
            sscc = serialNumber + String(i).padStart(3, '0');
        } else {
            // Генерируем уникальный SSCC на основе времени и индекса
            sscc = '00' + String(Date.now() + i).padStart(16, '0');
        }
        
        aggregationUnits.push({
            aggregationItemsCount: unitCodes.length,
            aggregationUnitCapacity: codes.length,
            codes: unitCodes,
            shouldBeUnbundled: false,
            unitSerialNumber: sscc
        });
    }

    const payload = {
        aggregationUnits,
        businessPlaceId
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

// ========== АВТОМАТИЗИРОВАННЫЕ ТЕСТЫ ==========

async function runTestSuite(suiteName) {
    const progressPanel = document.getElementById('testProgressPanel');
    const progressBar = document.getElementById('testProgressBar');
    const progressText = document.getElementById('testProgressText');

    progressPanel.style.display = 'block';
    progressText.textContent = `Запуск ${suiteName === 'full' ? 'всех тестов' : suiteName}...`;
    progressBar.style.width = '10%';

    try {
        showLoading(true);

        const endpoint = {
            'orders': '/v1/test/orders-suite',
            'utilisations': '/v1/test/utilisations-suite',
            'aggregations': '/v1/test/aggregations-suite',
            'full': '/v1/test/full-suite'
        }[suiteName];

        const response = await fetch(`${API_HOST}${endpoint}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
        });

        progressBar.style.width = '50%';

        const data = await response.json();
        showLoading(false);

        progressBar.style.width = '100%';

        if (!response.ok) {
            showError(`Ошибка тестирования: ${data.error || response.statusText}`);
            progressText.textContent = `❌ Ошибка выполнения ${suiteName}`;
            progressText.style.color = 'red';
            return;
        }

        const runId = data.run_id;
        const passed = data.passed || 0;
        const failed = data.failed || 0;
        const total = data.total || 0;
        const duration = data.duration_sec || 0;
        const status = data.status || 'UNKNOWN';

        // Показываем результаты
        const resultHTML = `
            <div style="background: var(--surface); padding: 16px; border-radius: 8px; margin-top: 16px;">
                <h3 style="margin-top: 0;">✅ Тесты завершены</h3>
                <p><strong>Suite:</strong> ${suiteName}</p>
                <p><strong>Status:</strong> ${status === 'SUCCESS' ? '✅ SUCCESS' : '❌ FAILED'}</p>
                <p><strong>Всего тестов:</strong> ${total}</p>
                <p><strong>Пройдено:</strong> <span style="color: #4caf50;">${passed}</span></p>
                <p><strong>Ошибок:</strong> <span style="color: ${failed > 0 ? '#f44336' : '#4caf50'};">${failed}</span></p>
                <p><strong>Время:</strong> ${duration}с</p>
                <p><strong>ID запуска:</strong> <code>${runId}</code></p>
                <button type="button" class="secondary-btn" onclick="loadTestDetails(${runId})">
                    📊 Посмотреть детали
                </button>
            </div>
        `;

        showResult(resultHTML, status === 'SUCCESS' ? 'success' : 'error');
        progressText.textContent = `✅ Тесты ${suiteName} завершены: ${passed}/${total} пройдено`;
        progressText.style.color = failed === 0 ? '#4caf50' : '#f44336';

        // Обновить историю
        loadTestHistory();
        loadHistory();
    } catch (error) {
        showLoading(false);
        showError(`Ошибка подключения: ${error.message}`);
        progressText.textContent = `❌ Ошибка: ${error.message}`;
        progressText.style.color = 'red';
        progressPanel.style.display = 'block';
    }
}

async function loadTestHistory() {
    try {
        const response = await fetch(`${API_HOST}/v1/test/runs?limit=10`);
        if (!response.ok) return;

        const data = await response.json();
        const runs = data.test_runs || [];

        // Можно отобразить историю в отдельной панели
        log('Текущие тесты:', runs);
    } catch (error) {
        console.error('Ошибка загрузки истории тестов:', error);
    }
}

async function loadTestDetails(runId) {
    try {
        const response = await fetch(`${API_HOST}/v1/test/cases?run_id=${runId}`);
        if (!response.ok) return;

        const data = await response.json();
        const testCases = data.test_cases || [];

        // Формируем детальный отчет
        let detailsHTML = `<h3>Детали тестового запуска #${runId}</h3>`;
        detailsHTML += '<table style="width:100%; border-collapse:collapse;">';
        detailsHTML += '<tr style="background:var(--surface); border-bottom:1px solid var(--border);">';
        detailsHTML += '<th style="padding:8px; text-align:left;">Тест</th>';
        detailsHTML += '<th style="padding:8px; text-align:left;">Статус</th>';
        detailsHTML += '<th style="padding:8px; text-align:left;">Время (ms)</th>';
        detailsHTML += '</tr>';

        testCases.forEach(tc => {
            const statusIcon = tc.status === 'PASSED' ? '✅' : '❌';
            const statusColor = tc.status === 'PASSED' ? '#4caf50' : '#f44336';
            detailsHTML += `<tr style="border-bottom:1px solid var(--border);">`;
            detailsHTML += `<td style="padding:8px;">${tc.case_name}</td>`;
            detailsHTML += `<td style="padding:8px; color:${statusColor};">${statusIcon} ${tc.status}</td>`;
            detailsHTML += `<td style="padding:8px;">${tc.duration_milliseconds}ms</td>`;
            detailsHTML += `</tr>`;
        });

        detailsHTML += '</table>';

        showResult(detailsHTML, 'normal');
    } catch (error) {
        showError(`Ошибка загрузки деталей: ${error.message}`);
    }
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
