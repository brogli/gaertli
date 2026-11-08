package testwait

import "testing"

func TestUntilReturnsOnceTheConditionHolds(t *testing.T) {
	calls := 0
	Until(func() bool {
		calls++
		return calls == 3
	})
	if calls != 3 {
		t.Errorf("cond called %d times, want 3", calls)
	}
}
