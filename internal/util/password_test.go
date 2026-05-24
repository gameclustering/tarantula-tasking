package util

import (
	"strconv"
	"testing"
)

func TestUtil(t *testing.T) {
	for i := range 100 {
		part := Partition([]byte("tester_"+strconv.Itoa(i)), 17)
		if part > 16 {
			t.Errorf("P %d\n", part)
		}
	}
}
