# ASL BELGISI Open API v1.21.1 Summary

**Base URL:** `https://{server}/api/`  
**Auth:** `Bearer {token}`  
**Content-Type:** `application/json;charset=UTF-8`

---

## 📋 1. ЗАКАЗЫ (Orders)

### 1.1 POST /orders - Зарегистрировать заказ на эмиссию КМ
Создание нового заказа на получение кодов маркировки.

**Параметры запроса (тело):**
```json
{
  "productGroup": "string (Да)",           // tobacco, water, alcohol, appliances, medicine, perfumes
  "businessPlaceId": "int (Да)",           // ID места деятельности
  "releaseMethodType": "string (Да)",      // PRIMARY, CONTRACT, IMPORT, OWN_PRODUCTION
  "isPaid": "boolean (Нет)",               // true/false - взимание платы
  "poNumber": "string (Нет)",              // Номер производственного заказа
  "products": [
    {
      "gtin": "string (Да)",               // 12-14 цифр
      "quantity": "int (Да)",              // Количество КМ
      "cisType": "string (Да)",            // UNIT, GROUP, SET
      "serialNumberType": "string (Да)",   // OPERATOR, MANUFACTURER, SELF_MADE
      "serialNumbers": ["string"]          // Обязательно если serialNumberType=SELF_MADE
    }
  ],
  "contractorInfo": {                      // Опционально, если СП
    "contractorTin": "string (Да)",        // ИНН
    "contractorCountryCode": "string (Да)" // Код страны
  }
}
```

**Ответ:**
```json
{
  "orderId": "uuid"
}
```

---

### 1.2 GET /orders - Получить информацию о заказах
Получение списка заказов с фильтрацией и информацией об отдельном заказе.

**Query параметры:**
| Параметр | Тип | Обязательный | Описание |
|----------|-----|-------------|---------|
| orderId | string | Нет | ID заказа (UUID) |
| status | string | Нет | PENDING, READY, REJECTED, CLOSED |
| productGroup | string | Нет | Товарная группа |
| contractorTin | string | Нет | ИНН сервис-провайдера |
| poNumber | string | Нет | Номер производственного заказа |
| dateFrom | ISO8601 | Нет | Дата начала (включительно) |
| dateTo | ISO8601 | Нет | Дата конца (до указанной даты) |
| limit | int | Нет | Лимит записей на страницу (по умолчанию 100) |
| cursor | string | Нет | ID последнего документа для пагинации |

**Ответ:**
```json
{
  "orderInfos": [
    {
      "orderId": "uuid",
      "productGroup": "string",
      "orderStatus": "string",
      "releaseMethodType": "string",
      "poNumber": "string",
      "createDate": "ISO8601",
      "rejectionReason": "string (если отклонено)"
    }
  ]
}
```

---

### 1.3 POST /order/close - Закрыть заказ
Закрытие активного заказа или подзаказа. Невыбранные КМ аннулируются.

**Query параметры:**
| Параметр | Тип | Описание |
|----------|-----|---------|
| orderId | string | ID заказа (обязательный) |
| gtin | string | Код товара (опционально, если указан - закроет только подзаказ) |

**Ответ:**
```json
{
  "orderId": "uuid",
  "gtin": "string (если закрывался подзаказ)"
}
```

---

## 📦 2. ПОДЗАКАЗЫ (Sub-orders)

### 2.1 GET /orders/sub-orders - Получить информацию о подзаказах
Информация о состоянии и прогрессе подзаказов.

**Query параметры:**
| Параметр | Тип | Обязательный | Описание |
|----------|-----|-------------|---------|
| orderId | string | Нет | ID заказа |
| gtin | string | Нет | Код товара |
| status | string | Нет | PENDING, READY, REJECTED, EXHAUSTED |
| cisType | string | Нет | UNIT, GROUP, SET |
| dateFrom | ISO8601 | Нет | Дата начала |
| dateTo | ISO8601 | Нет | Дата конца |
| limit | int | Нет | Лимит записей (по умолчанию 100) |
| cursor | string | Нет | ID последнего документа |

**Ответ:**
```json
{
  "subOrderInfos": [
    {
      "parentOrderId": "uuid",
      "gtin": "string",
      "bufferStatus": "PENDING|READY|REJECTED|EXHAUSTED",
      "cisType": "string",
      "availableCodes": "int",         // Всего генерировано
      "leftInBuffer": "int",           // Осталось в буфере
      "totalPassed": "int",            // Выгружено всего
      "lastPackId": "uuid",            // ID последнего пакета
      "createDate": "ISO8601",
      "rejectionReason": "string"
    }
  ]
}
```

---

## 🔐 3. КОДЫ МАРКИРОВКИ (Codes)

### 3.1 GET /codes - Получить (выгрузить) КМ из подзаказа
Выгрузка кодов маркировки пакетами.

**Query параметры:**
| Параметр | Тип | Обязательный | Описание |
|----------|-----|-------------|---------|
| orderId | string | Да | ID заказа |
| gtin | string | Да | Код товара |
| quantity | int | Да | Количество кодов (1 <= quantity <= доступно) |
| lastPackId | uuid | Нет | ID последнего пакета (для получения следующего) |

**Логика:**
- Если из подзаказа не выгружались КМ → выдается запрошенное количество
- Если указан lastPackId → выдается следующий пакет после указанного
- Если lastPackId = последний полученный пакет → создается новый пакет

**Ответ:**
```json
{
  "packId": "uuid",
  "codes": [
    "010489921512237121U&U1+<cfOUoZf93UehU"
  ]
}
```

**Примечание:** Коды содержат разделитель `` (GS character - 0x1d)

---

### 3.2 GET /codes/packs - Получить список пакетов КМ
Список всех пакетов, выгруженных из подзаказа.

**Query параметры:**
| Параметр | Тип | Обязательный |
|----------|-----|-------------|
| orderId | string | Да |
| gtin | string | Да |

**Ответ:**
```json
{
  "orderId": "uuid",
  "gtin": "string",
  "packs": [
    {
      "packId": "uuid",
      "packDateTime": "ISO8601",
      "quantity": "int"
    }
  ]
}
```

---

### 3.3 POST /public/api/cod/public/codes - Получить публичную информацию о кодах
Информация о статусе и реквизитах кодов маркировки.

**Параметры запроса:**
```json
{
  "codes": ["string", "string"]  // Массив полных кодов КМ
}
```

**Ответ:**
```json
[
  {
    "code": "string",
    "gtin": "string",
    "status": "RECEIVED|APPLIED|IN_CIRCULATION|UTILIZED|REFURBISHED|FORMATTED|WRITTEN_OFF",
    "productInfo": {
      "name": "string",
      "description": "string"
    }
  }
]
```

---

## 📤 4. ОТЧЕТЫ (Reports)

### 4.1 POST /utilisation - Отчет о нанесении КМ
Подтверждение факта производства/нанесения КМ на товар.

**Query параметр:**
- `productGroup` (обязательный) - товарная группа

**Параметры запроса (тело):**
```json
{
  "sntins": [
    "010489921512237121U&U1+<cfOUoZf93UehU"  // Полные КМ с разделителем
  ],
  "businessPlaceId": "int (Да)",                    // ID МОД
  "releaseType": "string (Да)",                     // PRODUCTION, IMPORT, TAKE_OVER
  "manufacturerCountry": "string (Да)",             // ISO код страны
  "productionOrderId": "string (Нет)",              // ID производственного заказа
  "productionDate": "ISO8601 (Да, кроме appliances)", // Дата производства
  "expirationDate": "ISO8601 (Да, кроме appliances)", // Дата истечения
  "seriesNumber": "string (Нет, обязательно для medicine)"  // Серия партии (1-20 символов)
}
```

**Ответ:**
```json
{
  "reportId": "uuid"
}
```

**Ограничения:**
- Количество кодов ограничено (см. настройки системы)
- Должны быть коды со статусом "RECEIVED"
- Все коды должны быть одной товарной группы
- Дата производства > дата заказа
- Дата истечения > текущая дата системы

---

### 4.2 POST /public/api/v1/doc/aggregation - Отчет об агрегации

Регистрация формирования упаковок (агрегация на разные уровни).

**Body параметры:**
```json
{
  "documentBody": "base64(JSON)",  // Закодированное тело отчета
  "signature": "string (Нет)"      // Открепленная подпись
}
```

**Структура documentBody (перед кодированием в base64):**
```json
{
  "aggregationUnits": [
    {
      "unitSerialNumber": "string (Да)",              // Код родительской упаковки (SSCC)
      "aggregationItemsCount": "int (Да)",            // Количество вложенных упаковок
      "aggregationUnitCapacity": "int (Да)",          // Плановая емкость
      "codes": ["string", "string"],                  // Вложенные коды (дочерних упаковок)
      "shouldBeUnbundled": "boolean (Нет)"            // Признак необходимости извлечения
    }
  ],
  "businessPlaceId": "int (Да)",
  "documentDate": "ISO8601 (Да)",    // Дата упаковки > дата производства
  "productionOrderId": "string (Нет)"
}
```

**Ответ:**
```json
{
  "documentId": "uuid"
}
```

---

### 4.3 POST /public/api/v1/doc/validation - Отчет о валидации печати
Отчет об оценке качества печати кодов DataMatrix.

**Параметры:**
```json
{
  "documentBody": "base64(JSON)",  // Закодированный отчет
  "signature": "string (Нет)"
}
```

**Структура documentBody:**
```json
{
  "productGroup": "string (Да)",
  "businessPlaceId": "int (Да)",
  "packageType": "string (Да)",     // UNIT, GROUP, SET
  "documentDate": "ISO8601 (Да)",
  "productionOrderId": "string (Нет)",
  "codes": [
    {
      "code": "string (Да)",        // КИ без контрольной цифры
      "printQualityClass": "string (Да)"  // A, B, C, F
    }
  ]
}
```

**Ответ:**
```json
{
  "documentId": "uuid"
}
```

---

### 4.4 POST /public/api/v1/doc/correction - Корректировка данных КМ

Изменение сведений о кодах маркировки.

**Параметры:**
```json
{
  "documentBody": "base64(JSON)",  // Закодированное тело (JSON должен быть отсортирован по ключам A-Z)
  "signature": "string (Нет)"
}
```

**Структура documentBody:**
```json
{
  "businessDatetime": "ISO8601 (Да)",
  "updatedFields": {
    "productionDatetime": "ISO8601 (Нет)",
    "expirationDatetime": "ISO8601 (Нет)",
    "manufacturerCountry": "string (Нет)",
    "seriesNumber": "string (Нет)"
  },
  "codes": ["string", "string"]  // КМ для корректировки (макс. зависит от системы)
}
```

**Ответ:**
```json
{
  "documentId": "uuid"
}
```

---

### 4.5 POST /public/api/v1/doc/transport-code-disaggregation - Расформирование упаковки

Расформирование групповых (КИГУ) и транспортных (КИТУ) упаковок.

**Параметры:**
```json
{
  "documentBody": "base64(JSON)",
  "signature": "string (Нет)"
}
```

**Структура documentBody:**
```json
{
  "businessDatetime": "ISO8601 (Да)",
  "codes": ["string", "string"]  // SSCC коды транспортных/групповых упаковок
}
```

**Ответ:**
```json
{
  "documentId": "uuid"
}
```

---

## 📊 5. СПРАВОЧНИКИ И ПЕРЕЧИСЛЕНИЯ

### Товарные группы (productGroup):
- `tobacco` - Табак
- `alcohol` - Алкоголь
- `beer` - Пиво
- `water` - Вода и напитки
- `appliances` - Бытовая техника
- `medicine` - Лекарства (фарма)
- `perfumes` - Парфюмерия

### Статусы заказов (orderStatus):
- `PENDING` - Ожидание обработки
- `READY` - Готов к выгрузке
- `REJECTED` - Отклонен
- `CLOSED` - Закрыт

### Статусы подзаказов (bufferStatus):
- `PENDING` - В ожидании
- `READY` - Готов
- `REJECTED` - Отклонен
- `EXHAUSTED` - Исчерпан

### Методы выпуска (releaseMethodType):
- `PRIMARY` - Первичная маркировка (производство)
- `CONTRACT` - Контрактная маркировка
- `IMPORT` - Маркировка импорта
- `OWN_PRODUCTION` - Собственное производство

### Типы упаковки (cisType):
- `UNIT` - Потребительская упаковка (КИ)
- `GROUP` - Групповая упаковка (КИГУ)
- `SET` - Набор

### Способ генерации серийных номеров (serialNumberType):
- `OPERATOR` - Генерируется оператором
- `MANUFACTURER` - Генерируется производителем
- `SELF_MADE` - Передаются пользователем

### Способ ввода в оборот (releaseType):
- `PRODUCTION` - Производство
- `IMPORT` - Импорт
- `TAKE_OVER` - Передача прав

### Статусы КМ:
- `RECEIVED` - Получен (выпущен оператором)
- `APPLIED` - Нанесен на товар
- `IN_CIRCULATION` - В обороте (продажи)
- `UTILIZED` - Использован (проданы товары)
- `REFURBISHED` - Восстановлен
- `FORMATTED` - Переформатирован
- `WRITTEN_OFF` - Списан

### Классы качества печати:
- `A` - Отличное качество
- `B` - Хорошее качество
- `C` - Приемлемое качество
- `F` - Неудовлетворительное качество

---

## 🔍 6. ВАЖНЫЕ ЗАМЕТКИ

### Обработка специальных символов:
- КМ содержат разделитель групп данных (GS character) `` (0x1d)
- При отправке в JSON их нужно экранировать как ``
- При парсировании JSON их нужно восстанавливать

### Пагинация:
- По умолчанию возвращается 100 записей
- Для получения следующей страницы используйте `cursor` из последней записи

### Даты:
- Формат ISO 8601: `2025-01-30T11:02:02.492Z` или `2025-01-30T11:02:02+01:00`
- Таймзоны обязательны
- Дата производства < дата истечения (обычно)
- Дата истечения > текущая системная дата

### Таймауты:
- Рекомендуется 15+ секунд для операций с большим количеством кодов
- Рекомендуется polling для проверки статуса заказов

### Лимиты:
- Максимальное количество товаров в одном заказе: зависит от настроек
- Максимальное количество кодов в одном отчете: зависит от настроек (обычно 100к)
- Максимальное количество активных заказов: ограничено

---

## 🧪 ПРИМЕРЫ ИСПОЛЬЗОВАНИЯ

### Полный flow: Заказ → Нанесение → Агрегация

```
1. POST /api/orders → orderId
2. GET /api/orders?orderId={orderId} → проверить статус (READY)
3. GET /api/orders/sub-orders?orderId={orderId} → информация о подзаказах
4. GET /api/codes?orderId={orderId}&gtin={gtin}&quantity=100 → packId + codes[]
5. POST /api/utilisation?productGroup=water → reportId (отчет о нанесении)
6. POST /public/api/v1/doc/aggregation → documentId (отчет об агрегации)
```

### Для тестирования используй:
- **Base URL:** `https://xtrace.stage.aslbelgski.uz`
- **Token:** Из .env файла
- **Productional GTINs:** Из чек-листа (вода, табак, пиво, алкоголь, фарма, техника)
