package util

import "strings"

// ExtractKI извлекает Код Идентификации (31-38 символов) из полного Кода Маркировки
// Для бытовой техники (appliances) мы отсекаем криптохвост (все что после \x1d93)
func ExtractKI(fullCode string) string {
	// Ищем разделитель группы криптохвоста \x1d93
	// \x1d - это тот самый символ GS (Group Separator)
	index := strings.Index(fullCode, "\x1d93")
	if index != -1 {
		return fullCode[:index]
	}

	// Если спецсимвола нет, пробуем найти просто "93" как текст (иногда так бывает в JSON)
	index = strings.Index(fullCode, "93")
	if index != -1 && len(fullCode) > 31 {
		return fullCode[:index]
	}

	// Если ничего не нашли, а код длинный (92 симв),
	// для бытовой техники КИ обычно занимает первые 31 символ (01 + 14 + 21 + 13)
	// Но лучше обрезать по стандарту до 31-38 символов.
	if len(fullCode) > 38 {
		return fullCode[:31]
	}

	return fullCode
}

// ConvertToKIList преобразует массив полных КМ в массив КИ
func ConvertToKIList(codes []string) []string {
	kiList := make([]string, len(codes))
	for i, code := range codes {
		if len(code) > 38 {
			kiList[i] = code[:38]
		} else {
			kiList[i] = code
		}
	}
	return kiList
}
