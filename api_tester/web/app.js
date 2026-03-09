// API Host
const API_HOST = 'http://localhost:8080';

// ========== ПЕРЕКЛЮЧЕНИЕ ВКЛАДОК ==========
function switchTab(tabName) {
    // Скрыть все вкладки
    const tabs = document.querySelectorAll('.tab-content');
    tabs.forEach(tab => tab.classList.remove('active'));

    // Убрать активный класс у кнопок
    const buttons = document.querySelectorAll('.tab-btn');
    buttons.forEach(btn => btn.classList.remove('active'));

    // Показать нужную вкладку
    document.getElementById(tabName).classList.add('active');

    // Сделать нужную кнопку активной
    event.target.classList.add('active');
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

    const payload = {
        gtin,
        productGroup: group,
        quantity,
        expirationDays: expirationDays
    };

    try {
        showLoading(true);
        const response = await fetch(`${API_HOST}/v1/workflow/execute`, {
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
            showSuccess('Цикл выполнен успешно!');
            showResult(JSON.stringify(data, null, 2), 'success');
            loadHistory();
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

    // Создаем структуру агрегации
    const aggregationUnits = [];
    for (let i = 0; i < packageCount; i++) {
        const unitCodes = codes.slice(i * Math.ceil(codes.length / packageCount), (i + 1) * Math.ceil(codes.length / packageCount));
        
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

    const payload = {
        gtin,
        productGroup: group,
        quantity,
        businessPlaceId: businessPlaceId,
        productionOrderId: productionOrderId || "",
        expirationDays: 365
    };

    try {
        showLoading(true);
        const response = await fetch(`${API_HOST}/v1/workflow/complete`, {
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
            showSuccess('Полный цикл выполнен!');
            showResult(JSON.stringify(data, null, 2), 'success');
            loadHistory();
        }
    } catch (error) {
        showLoading(false);
        showError(`Ошибка подключения: ${error.message}`);
        showResult(error.message, 'error');
    }
}

// ========== ЗАГРУЗКА ИСТОРИИ ==========
async function loadHistory() {
    try {
        const response = await fetch(`${API_HOST}/v1/marking/history`);
        
        if (!response.ok) {
            document.getElementById('history').innerHTML = `<p class="placeholder">⚠️ История недоступна</p>`;
            return;
        }

        const data = await response.json();
        
        if (!data || data.length === 0) {
            document.getElementById('history').innerHTML = `<p class="placeholder">Истории операций нет</p>`;
            return;
        }

        let historyHtml = '';
        data.slice(0, 20).forEach(item => {
            const time = new Date(item.created_at).toLocaleString('ru-RU');
            const statusClass = item.status === 'success' || item.status === 'SUCCESS' ? 'success' : 'error';
            
            historyHtml += `
                <div class="history-item ${statusClass}">
                    <div class="history-time">⏰ ${time}</div>
                    <div class="history-op">
                        ${item.operation_type} (${item.product_group})
                    </div>
                    <div style="color: #666; font-size: 0.85em; margin-top: 3px;">
                        ID: ${item.external_id || 'N/A'}
                    </div>
                </div>
            `;
        });

        document.getElementById('history').innerHTML = historyHtml;
    } catch (error) {
        document.getElementById('history').innerHTML = `<p class="placeholder">❌ Ошибка загрузки: ${error.message}</p>`;
    }
}

// ========== ПРОВЕРКА СТАТУСА API ==========
async function checkApiStatus() {
    try {
        // Используем AbortController для таймаута (совместимо со старыми браузерами)
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 3000);

        const response = await fetch(`${API_HOST}/health`, {
            method: 'GET',
            signal: controller.signal
        });

        clearTimeout(timeoutId);

        const statusElement = document.getElementById('apiStatus');
        if (response.ok) {
            statusElement.innerHTML = `
                <div class="status-content online">
                    ✅ API Online
                    <div style="margin-top: 10px; font-size: 0.9em;">
                        ${API_HOST}
                    </div>
                </div>
            `;
        } else {
            statusElement.innerHTML = `
                <div class="status-content offline">
                    ❌ API Offline (${response.status})
                </div>
            `;
        }
    } catch (error) {
        document.getElementById('apiStatus').innerHTML = `
            <div class="status-content offline">
                ❌ API Недоступен<br>
                <small>${error.message}</small>
            </div>
        `;
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

function showLoading(show) {
    const spinner = document.getElementById('loadingSpinner');
    if (show) {
        spinner.classList.add('show');
    } else {
        spinner.classList.remove('show');
    }
}

function closeModal() {
    document.getElementById('errorModal').classList.remove('show');
    document.getElementById('successModal').classList.remove('show');
}

// ========== ИНИЦИАЛИЗАЦИЯ ==========
document.addEventListener('DOMContentLoaded', function() {
    // Установить фокус на первое поле ввода
    document.getElementById('executeGtin').focus();
    
    // Проверить статус API
    checkApiStatus();

    // Проверять статус каждые 10 секунд
    setInterval(checkApiStatus, 10000);

    // Загрузить историю при загрузке
    loadHistory();

    // Загружать историю каждые 5 секунд
    setInterval(loadHistory, 5000);

    // Закрыть модальные окна при клике вне их
    window.onclick = function(event) {
        const errorModal = document.getElementById('errorModal');
        const successModal = document.getElementById('successModal');

        if (event.target === errorModal) {
            errorModal.classList.remove('show');
        }
        if (event.target === successModal) {
            successModal.classList.remove('show');
        }
    };
});
