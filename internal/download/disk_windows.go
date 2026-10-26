//go:build windows

package download

// FreeSpace is not implemented on Windows; callers treat 0 as unknown.
func FreeSpace(path string) (uint64, error) { return 0, nil }
