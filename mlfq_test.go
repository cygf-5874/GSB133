package mlfq

import (
	"fmt"
	"testing"
)

func run(s *Scheduler) []string {
	var out []string
	for s.Pending() > 0 {
		id, _ := s.Step()
		out = append(out, id)
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFIFOWithinHighest(t *testing.T) {
	s := New(4, 1, 1000, nil)
	s.Submit("a", 1)
	s.Submit("b", 1)
	if id, done := s.Step(); id != "a" || !done {
		t.Fatalf("首步 = (%q,%v)，期望 (a,true)", id, done)
	}
	if id, done := s.Step(); id != "b" || !done {
		t.Fatalf("次步 = (%q,%v)，期望 (b,true)", id, done)
	}
}

func TestDemoteAfterQuantum(t *testing.T) {
	s := New(3, 1, 1000, nil)
	s.Submit("x", 3)
	if id, done := s.Step(); id != "x" || done {
		t.Fatalf("首步 = (%q,%v)，期望 (x,false)", id, done)
	}
	s.Submit("y", 1)
	if id, _ := s.Step(); id != "y" {
		t.Fatalf("次步 = %q，期望 y（新任务在最高级）", id)
	}
}

func TestPendingAndEmpty(t *testing.T) {
	s := New(2, 1, 1000, nil)
	if id, done := s.Step(); id != "" || done {
		t.Fatalf("空队列 Step = (%q,%v)，期望 (,false)", id, done)
	}
	if n := s.Pending(); n != 0 {
		t.Fatalf("空队列 Pending = %d，期望 0", n)
	}
	s.Submit("only", 1)
	if n := s.Pending(); n != 1 {
		t.Fatalf("Pending = %d，期望 1", n)
	}
}

func TestReset(t *testing.T) {
	s := New(3, 1, 1000, nil)
	s.Submit("x", 4)
	s.Step()
	s.Step()
	s.Reset()
	if n := s.Pending(); n != 0 {
		t.Fatalf("Reset 后 Pending = %d，期望 0", n)
	}
	if id, _ := s.Step(); id != "" {
		t.Fatalf("Reset 后 Step = %q，期望空", id)
	}
	s.Submit("p", 3)
	s.Submit("q", 3)
	fresh := New(3, 1, 1000, nil)
	fresh.Submit("p", 3)
	fresh.Submit("q", 3)
	if got, want := run(s), run(fresh); !eq(got, want) {
		t.Fatalf("Reset 后序列 = %v，期望与新建一致 %v", got, want)
	}
}

func TestBoostOrder(t *testing.T) {
	s := New(2, 1, 2, nil)
	s.Submit("L", 4)
	s.Submit("s1", 1)
	s.Submit("s2", 1)
	s.Submit("s3", 1)
	want := []string{"L", "s1", "L", "s2", "L", "s3", "L"}
	if got := run(s); !eq(got, want) {
		t.Fatalf("序列 = %v，期望 %v", got, want)
	}
}

func TestSubmitOrderStable(t *testing.T) {
	build := func() *Scheduler {
		s := New(3, 1, 4, nil)
		for i := 0; i < 6; i++ {
			s.Submit(fmt.Sprintf("j%d", i), (i%3)+1)
		}
		return s
	}
	if got, want := run(build()), run(build()); !eq(got, want) {
		t.Fatalf("两次调度 = %v vs %v，期望一致", got, want)
	}
}
