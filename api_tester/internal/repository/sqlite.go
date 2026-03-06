package repository

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type Storage struct {
	db *sql.DB
}

func NewStorage(path string) (*Storage, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Создаем таблицу для счетчиков, если её нет
	query := `
	CREATE TABLE IF NOT EXISTS counters (
		name TEXT PRIMARY KEY,
		last_value INTEGER
	);`
	_, err = db.Exec(query)
	return &Storage{db: db}, err
}

// GetNextSequence увеличивает счетчик в базе и возвращает новый номер
func (s *Storage) GetNextSequence(name string) (int, error) {
	// 1. Пытаемся вставить начальное значение 0, если ключа нет
	s.db.Exec("INSERT OR IGNORE INTO counters (name, last_value) VALUES (?, 0)", name)

	// 2. Увеличиваем и возвращаем
	var nextVal int
	err := s.db.QueryRow("UPDATE counters SET last_value = last_value + 1 WHERE name = ? RETURNING last_value", name).Scan(&nextVal)
	return nextVal, err
}
