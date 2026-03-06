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
	);`

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
