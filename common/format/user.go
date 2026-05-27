package format

func UserTag(tag string, uuid string) string {
	// The Go compiler optimises simple + for a small number of string
	// operands into a single allocation, which is faster than
	// strings.Builder for two-part concatenation.
	return tag + "|" + uuid
}
