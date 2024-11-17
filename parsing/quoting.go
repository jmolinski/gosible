package parsing

func isQuoted(s string) bool {
	if len(s) < 2 {
		return false
	}
	sameFirstLast := s[0] == s[len(s)-1]
	firstIsQuote := s[0] == '"' || s[0] == '\''
	lastNotEscaped := s[len(s)-2] != '\\'
	return sameFirstLast && firstIsQuote && lastNotEscaped
}

// Unquote removes first and last quotes from a string, if the string starts and ends with the same quotes
func Unquote(s string) string {
	if isQuoted(s) {
		return s[1 : len(s)-1]
	}
	return s
}
