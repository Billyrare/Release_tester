package util

import "strings"

// kiLengths — длина КИ (Кода Идентификации) в символах по товарным группам.
// КИ = полный КМ без GS-разделителя и криптохвоста (AI 93).
// Источник: таблица форматов ASL Belgisi.
var kiLengths = map[string]int{
	"tobacco":      21,
	"alcohol":      25,
	"beer":         25,
	"beer_group":   31,  // групповая упаковка пива (КИГУ)
	"appliances":   38,
	"pharma":       31,  // лекарственные препараты
	"medicals":     31,  // медицинские изделия
	"water":        31,
	"vegetableoil": 31,
	"bio":          31,
	"fertilizers":  31,
	// antiseptic — нет данных о длине КИ
}

// TruncateToKI обрезает полный КМ (Код Маркировки) до формата КИ (Код Идентификации).
// 1. Режет по точной КИ-длине для товарной группы.
// 2. Убирает GS-символ (\x1d / ) — он никогда не должен быть внутри КИ.
func TruncateToKI(code, productGroup string) string {
	if kiLen, ok := kiLengths[productGroup]; ok {
		if len(code) > kiLen {
			code = code[:kiLen]
		}
	} else {
		// Fallback: ищем GS + "93" (начало криптохвоста)
		code = ExtractKI(code)
	}
	// Убираем GS-разделитель (0x1D) на случай если он попал в результат
	return strings.ReplaceAll(code, "\x1d", "")
}

// TruncateToKIList конвертирует массив полных КМ в массив КИ для заданной товарной группы.
func TruncateToKIList(codes []string, productGroup string) []string {
	result := make([]string, len(codes))
	for i, c := range codes {
		result[i] = TruncateToKI(c, productGroup)
	}
	return result
}

// ExtractKI извлекает КИ из полного КМ через поиск GS-разделителя перед криптохвостом.
// Используется как fallback когда productGroup неизвестен.
func ExtractKI(fullCode string) string {
	// GS (0x1D) + "93" — стандартное начало криптохвоста
	if idx := strings.Index(fullCode, "\x1d93"); idx != -1 {
		return fullCode[:idx]
	}
	// Иногда GS приходит как экранированный unicode 
	if idx := strings.Index(fullCode, "93"); idx != -1 {
		return fullCode[:idx]
	}
	// Если код длиннее максимально известного КИ — обрезаем до 38
	if len(fullCode) > 38 {
		return fullCode[:38]
	}
	return fullCode
}

// ConvertToKIList — устаревший вариант, оставлен для совместимости.
// Используйте TruncateToKIList с явным productGroup.
func ConvertToKIList(codes []string) []string {
	result := make([]string, len(codes))
	for i, code := range codes {
		result[i] = ExtractKI(code)
	}
	return result
}
