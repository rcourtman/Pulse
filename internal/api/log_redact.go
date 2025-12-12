package api

func safePrefixForLog(value string, n int) string {
	if n <= 0 || value == "" {
		return ""
	}
	if len(value) <= n {
		return value
	}
	return value[:n]
}
