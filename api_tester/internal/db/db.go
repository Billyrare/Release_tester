package db

import (
	"database/sql"
	"log"

	_ "github.com/glebarez/go-sqlite" // Импорт драйвера
)

var DB *sql.DB

// InitDB создает файл базы и нужные таблицы
func InitDB() {
	var err error
	// Создаем файл database.db в корне проекта
	DB, err = sql.Open("sqlite", "./api_tester.db")
	if err != nil {
		log.Fatalf("Ошибка открытия базы данных: %v", err)
	}

	createLogsTable := `
	CREATE TABLE IF NOT EXISTS operation_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		operation_type TEXT,    -- ORDER, UTILISATION, AGGREGATION
		product_group  TEXT,
		external_id    TEXT,    -- orderId, reportId или documentId
		status         TEXT,    -- SUCCESS, FAILED
		details        TEXT,    -- Доп. инфо (например, количество кодов)
		created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS test_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		suite_name TEXT,        -- 'orders', 'utilisations', 'aggregations', 'full'
		status TEXT,            -- 'RUNNING', 'SUCCESS', 'FAILED'
		total_cases INTEGER DEFAULT 0,
		passed_cases INTEGER DEFAULT 0,
		failed_cases INTEGER DEFAULT 0,
		skipped_cases INTEGER DEFAULT 0,
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME,
		duration_seconds INTEGER,
		error_message TEXT
	);

	CREATE TABLE IF NOT EXISTS test_cases (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id INTEGER,
		case_name TEXT,         -- 'order_tobacco', 'utilisation_water', и т.д.
		description TEXT,
		status TEXT,            -- 'PASSED', 'FAILED', 'SKIPPED'
		request_body TEXT,      -- JSON запроса
		response_body TEXT,     -- JSON ответа
		error_message TEXT,
		duration_milliseconds INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (run_id) REFERENCES test_runs(id)
	);
	`

	_, err = DB.Exec(createLogsTable)
	if err != nil {
		log.Fatalf("Ошибка создания таблицы логов: %v", err)
	}

	// Таблица для хранения настроек (например, счетчик SSCC)
	query := `
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT
	);

	CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		order_id TEXT,
		gtin TEXT,
		product_group TEXT,
		status TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err = DB.Exec(query)
	if err != nil {
		log.Fatalf("Ошибка создания таблиц: %v", err)
	}

	// Инициализируем счетчик SSCC, если его нет
	DB.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES ('sscc_counter', '1')")

	log.Println("INFO: База данных инициализирована (api_tester.db)")
}
func LogOperation(opType, group, id, status, details string) {
	log.Printf("DEBUG: Попытка записи лога: %s, %s, %s", opType, group, id)

	_, err := DB.Exec(
		"INSERT INTO operation_logs (operation_type, product_group, external_id, status, details) VALUES (?, ?, ?, ?, ?)",
		opType, group, id, status, details,
	)
	if err != nil {
		log.Printf("ERROR: Ошибка записи в БД: %v", err)
	} else {
		log.Println("DEBUG: Лог успешно записан в БД")
	}
}

// GetOperationHistory получает последние операции из логов (для фронта)
func GetOperationHistory(limit int) ([]map[string]interface{}, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := DB.Query(`
		SELECT id, operation_type, product_group, external_id, status, details, created_at
		FROM operation_logs
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []map[string]interface{}

	for rows.Next() {
		var id int
		var opType, group, externalID, status, details, createdAt string

		if err := rows.Scan(&id, &opType, &group, &externalID, &status, &details, &createdAt); err != nil {
			log.Printf("ERROR: Ошибка сканирования строки: %v", err)
			continue
		}

		item := map[string]interface{}{
			"id":              id,
			"operation_type":  opType,
			"product_group":   group,
			"external_id":     externalID,
			"status":          status,
			"details":         details,
			"created_at":      createdAt,
		}

		history = append(history, item)
	}

	return history, nil
}

// ========== ФУНКЦИИ ДЛЯ ЛОГИРОВАНИЯ ТЕСТОВ ==========

// StartTestRun создает новый запуск набора тестов
func StartTestRun(suiteName string) (int64, error) {
	result, err := DB.Exec(`
		INSERT INTO test_runs (suite_name, status, total_cases, passed_cases, failed_cases, skipped_cases)
		VALUES (?, ?, ?, ?, ?, ?)
	`, suiteName, "RUNNING", 0, 0, 0, 0)
	if err != nil {
		log.Printf("ERROR: Ошибка создания test_run: %v", err)
		return 0, err
	}
	return result.LastInsertId()
}

// LogTestCase логирует результат одного тестового случая
func LogTestCase(runID int64, caseName, description, status, requestBody, responseBody, errorMessage string, durationMs int64) error {
	_, err := DB.Exec(`
		INSERT INTO test_cases
		(run_id, case_name, description, status, request_body, response_body, error_message, duration_milliseconds)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, runID, caseName, description, status, requestBody, responseBody, errorMessage, durationMs)
	if err != nil {
		log.Printf("ERROR: Ошибка логирования test_case: %v", err)
		return err
	}
	return nil
}

// UpdateTestRunStats обновляет статистику запуска тестов
func UpdateTestRunStats(runID int64, totalCases, passedCases, failedCases, skippedCases, durationSeconds int) error {
	status := "SUCCESS"
	if failedCases > 0 {
		status = "FAILED"
	}

	_, err := DB.Exec(`
		UPDATE test_runs
		SET status = ?, total_cases = ?, passed_cases = ?, failed_cases = ?,
			skipped_cases = ?, duration_seconds = ?, completed_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, totalCases, passedCases, failedCases, skippedCases, durationSeconds, runID)
	if err != nil {
		log.Printf("ERROR: Ошибка обновления test_run: %v", err)
		return err
	}
	return nil
}

// FailTestRun помечает запуск как неудавшийся с сообщением об ошибке
func FailTestRun(runID int64, errorMessage string) error {
	_, err := DB.Exec(`
		UPDATE test_runs
		SET status = ?, error_message = ?, completed_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, "FAILED", errorMessage, runID)
	if err != nil {
		log.Printf("ERROR: Ошибка отмечания test_run как failed: %v", err)
		return err
	}
	return nil
}

// GetTestRunHistory получает историю запусков тестов
func GetTestRunHistory(limit int) ([]map[string]interface{}, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	rows, err := DB.Query(`
		SELECT id, suite_name, status, total_cases, passed_cases, failed_cases, skipped_cases,
		       started_at, completed_at, duration_seconds, error_message
		FROM test_runs
		ORDER BY started_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []map[string]interface{}

	for rows.Next() {
		var id, totalCases, passedCases, failedCases, skippedCases int
		var durationSec sql.NullInt64
		var suiteName, status, startedAt, completedAt, errorMsg string
		var completedAtNull sql.NullString

		if err := rows.Scan(&id, &suiteName, &status, &totalCases, &passedCases,
			&failedCases, &skippedCases, &startedAt, &completedAtNull, &durationSec, &errorMsg); err != nil {
			log.Printf("ERROR: Ошибка сканирования: %v", err)
			continue
		}

		completedAt = ""
		if completedAtNull.Valid {
			completedAt = completedAtNull.String
		}

		item := map[string]interface{}{
			"id":              id,
			"suite_name":      suiteName,
			"status":          status,
			"total_cases":     totalCases,
			"passed_cases":    passedCases,
			"failed_cases":    failedCases,
			"skipped_cases":   skippedCases,
			"started_at":      startedAt,
			"completed_at":    completedAt,
			"duration_seconds": durationSec.Int64,
			"error_message":   errorMsg,
		}

		history = append(history, item)
	}

	return history, nil
}

// GetTestCasesByRunID получает все тестовые случаи для конкретного запуска
func GetTestCasesByRunID(runID int64) ([]map[string]interface{}, error) {
	rows, err := DB.Query(`
		SELECT id, run_id, case_name, description, status, request_body, response_body,
		       error_message, duration_milliseconds, created_at
		FROM test_cases
		WHERE run_id = ?
		ORDER BY created_at ASC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cases []map[string]interface{}

	for rows.Next() {
		var id, runID int64
		var durationMs int
		var caseName, description, status, requestBody, responseBody, errorMsg, createdAt string

		if err := rows.Scan(&id, &runID, &caseName, &description, &status, &requestBody,
			&responseBody, &errorMsg, &durationMs, &createdAt); err != nil {
			log.Printf("ERROR: Ошибка сканирования test_case: %v", err)
			continue
		}

		item := map[string]interface{}{
			"id":                       id,
			"run_id":                   runID,
			"case_name":                caseName,
			"description":              description,
			"status":                   status,
			"request_body":             requestBody,
			"response_body":            responseBody,
			"error_message":            errorMsg,
			"duration_milliseconds":    durationMs,
			"created_at":               createdAt,
		}

		cases = append(cases, item)
	}

	return cases, nil
}
