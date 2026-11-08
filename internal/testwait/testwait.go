// Package testwait waits in tests for a condition that nothing signals, such
// as a file a detached process writes.
package testwait

import "time"

// Until polls cond until it holds. It sets no time limit, so a slow runner
// cannot fail it: a condition that never holds fails at go test's timeout,
// with every goroutine's stack.
func Until(cond func() bool) {
	for !cond() {
		time.Sleep(5 * time.Millisecond)
	}
}
