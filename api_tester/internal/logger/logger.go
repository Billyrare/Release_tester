package logger

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// LogEntry представляет одну запись логов
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

// LogManager управляет логами для SSE
type LogManager struct {
	mu       sync.Mutex
	messages []LogEntry
	maxSize  int
	subs     []chan LogEntry
}

var globalLogger *LogManager

func init() {
	globalLogger = &LogManager{
		messages: make([]LogEntry, 0, 100),
		maxSize:  200, // Максимум 200 логов в памяти
		subs:     make([]chan LogEntry, 0),
	}
}

// GetLogger возвращает глобальный логгер
func GetLogger() *LogManager {
	return globalLogger
}

// AddLog добавляет новый лог
func (lm *LogManager) AddLog(level, message string) {
	entry := LogEntry{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Level:     level,
		Message:   message,
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Добавить в очередь
	lm.messages = append(lm.messages, entry)

	// Удалить старые, если превышен лимит
	if len(lm.messages) > lm.maxSize {
		lm.messages = lm.messages[len(lm.messages)-lm.maxSize:]
	}

	// Отправить всем подписчикам
	for _, ch := range lm.subs {
		select {
		case ch <- entry:
		default:
			// Если канал переполнен, пропускаем
		}
	}

	// Также вывести в стандартный лог
	log.Printf("[%s] %s: %s", entry.Timestamp, level, message)
}

// Info логирует информационное сообщение
func (lm *LogManager) Info(msg string) {
	lm.AddLog("INFO", msg)
}

// Infof логирует с форматированием
func (lm *LogManager) Infof(format string, args ...interface{}) {
	lm.AddLog("INFO", fmt.Sprintf(format, args...))
}

// Debug логирует debug сообщение
func (lm *LogManager) Debug(msg string) {
	lm.AddLog("DEBUG", msg)
}

// Debugf логирует debug с форматированием
func (lm *LogManager) Debugf(format string, args ...interface{}) {
	lm.AddLog("DEBUG", fmt.Sprintf(format, args...))
}

// Error логирует ошибку
func (lm *LogManager) Error(msg string) {
	lm.AddLog("ERROR", msg)
}

// Errorf логирует ошибку с форматированием
func (lm *LogManager) Errorf(format string, args ...interface{}) {
	lm.AddLog("ERROR", fmt.Sprintf(format, args...))
}

// Subscribe подписывается на логи
func (lm *LogManager) Subscribe() chan LogEntry {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	ch := make(chan LogEntry, 10)
	lm.subs = append(lm.subs, ch)

	// Отправить последние 20 логов новому подписчику
	go func() {
		lm.mu.Lock()
		start := len(lm.messages) - 20
		if start < 0 {
			start = 0
		}
		msgs := make([]LogEntry, len(lm.messages[start:]))
		copy(msgs, lm.messages[start:])
		lm.mu.Unlock()

		for _, msg := range msgs {
			select {
			case ch <- msg:
			case <-time.After(1 * time.Second):
				// Таймаут, закрыть канал
				close(ch)
				return
			}
		}
	}()

	return ch
}

// GetLastLogs возвращает последние N логов
func (lm *LogManager) GetLastLogs(n int) []LogEntry {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if n > len(lm.messages) {
		n = len(lm.messages)
	}

	start := len(lm.messages) - n
	if start < 0 {
		start = 0
	}

	result := make([]LogEntry, n)
	copy(result, lm.messages[start:])
	return result
}

// Clear очищает логи
func (lm *LogManager) Clear() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.messages = make([]LogEntry, 0, 100)
}
