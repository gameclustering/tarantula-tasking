package util

import (
	"sync"
	"testing"
)

func TestSmoke(t *testing.T) {
	sfk := NewSnowflake(1,EpochMillisecondsFromMidnight(2020, 1, 1))
	var wait sync.WaitGroup
	const loop = 100
	const round = 10000
	wait.Add(loop)
	for i := range loop {
		go func() {
			defer wait.Done()
			m := make(map[int64]int,round)
			for j := range round {
				id, err := sfk.Id()
				if err != nil {
					t.Errorf("No ID %d >> %d >>%d\n", i, id, j)
					return
				}
				_, existed := m[id]
				if existed {
					tm, nd, se := sfk.Parse(id)
					t.Errorf("Duplcated Key %d >> %d >> %d >>%d\n", tm, nd,se,id)
					return
				}
				m[id] = i
			}
			ct := 0
			for k, v := range m {
				ct++
				_, nd, _ := sfk.Parse(k)
				if nd != sfk.NodeId {
					t.Error("Node Id not matched")
				}
				if v != i {
					t.Error("Value not matched")
				}
				delete(m,k)
			}
			
			if ct != round {
				t.Errorf("Total Not Match: %d\n", ct)
			}
		}()
	}
	wait.Wait()
}
