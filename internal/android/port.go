package android

import (
	"fmt"
	"net"
)

// freeEmulatorPort picks an even console port that the Android emulator can
// use.  The emulator needs the console port and the next (odd) port for adb,
// so both have to be free.  Ports 5554-5682 are the range the emulator and
// adb use for local devices.
func freeEmulatorPort() (int, error) {
	for port := 5554; port <= 5682; port += 2 {
		if portFree(port) && portFree(port+1) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free emulator port in 5554-5682; stop another emulator and retry")
}

func portFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}
