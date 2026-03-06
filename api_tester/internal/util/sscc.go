package util

import (
	"api_tester/internal/db"
	"fmt"
	"strconv"
)

// GetNextSSCC теперь работает с SQLite
func GetNextSSCC(tin string) (string, error) {
	var countStr string
	// 1. Получаем текущее значение из базы
	err := db.DB.QueryRow("SELECT value FROM settings WHERE key = 'sscc_counter'").Scan(&countStr)
	if err != nil {
		return "", err
	}

	count, _ := strconv.Atoi(countStr)

	// 2. Формируем базу (Extension + TIN + Serial)
	base := fmt.Sprintf("0%s%07d", tin, count)
	checkDigit := calculateCheckDigit(base)

	// 3. Инкрементируем и сохраняем обратно в базу
	newCount := count + 1
	_, err = db.DB.Exec("UPDATE settings SET value = ? WHERE key = 'sscc_counter'", strconv.Itoa(newCount))
	if err != nil {
		return "", err
	}

	return "00" + base + strconv.Itoa(checkDigit), nil
}

// calculateCheckDigit остается без изменений (алгоритм Luhn)
func calculateCheckDigit(s string) int {
	sum := 0
	for i := len(s) - 1; i >= 0; i-- {
		digit, _ := strconv.Atoi(string(s[i]))
		posFromRight := len(s) - i
		if posFromRight%2 != 0 {
			sum += digit * 3
		} else {
			sum += digit * 1
		}
	}
	return (10 - (sum % 10)) % 10
}
