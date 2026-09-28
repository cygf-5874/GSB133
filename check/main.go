// Command check 是 mlfq 的固定验收程序。
//
// ⚠️ 不要修改本文件。它是判定「题目有没有做对」的依据；
// 修改它只会让判定失效，不会让实现变对。
//
// 8 个场景分四组：
//
//	semantics 3：新任务进最高级且同级 FIFO / 用完时间片降级 / 提升回到队首
//	cost      2：计数与任务规模无关（前 T 步）/ 提升下计数仍受控
//	stable    2：固定输入序列逐项相同 / Reset 后与新建一致
//	edge      1：空队列与单任务
//
// 判据全部是硬编码期望值，不依赖墙钟、时基或 map 迭代顺序。
//
// 用法：
//
//	go run ./check                # 跑全部场景
//	go run ./check --only cost    # 只跑一组
//	go run ./check -list          # 列出场景
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"mlfq"
)

type scenario struct {
	group string
	name  string
	run   func() error
}

var scenarios = []scenario{
	{"semantics", "S1_new_task_highest_fifo", s1},
	{"semantics", "S2_demote_after_quantum", s2},
	{"semantics", "S3_boost_front_of_highest", s3},
	{"cost", "C1_ops_independent_of_n", c1},
	{"cost", "C2_ops_bounded_with_boost", c2},
	{"stable", "S1_fixed_sequence_identical", st1},
	{"stable", "S2_reset_fresh_state", st2},
	{"edge", "E1_empty_and_single", e1},
}

var groups = []string{"semantics", "cost", "stable", "edge"}

// counter 是注入的代价计数器。
type counter struct{ n int }

func (c *counter) Add(n int) { c.n += n }

func main() {
	only := ""
	list := false
	for i := 1; i < len(os.Args); i++ {
		a := os.Args[i]
		switch {
		case a == "-list":
			list = true
		case a == "--only":
			if i+1 < len(os.Args) {
				only = os.Args[i+1]
				i++
			}
		case strings.HasPrefix(a, "--only="):
			only = strings.TrimPrefix(a, "--only=")
		}
	}

	if list {
		for _, sc := range scenarios {
			fmt.Printf("%-10s %s\n", sc.group, sc.name)
		}
		return
	}

	if only != "" {
		found := false
		for _, g := range groups {
			if g == only {
				found = true
			}
		}
		if !found {
			fmt.Printf("未知分组 %q（可选：semantics / cost / stable / edge）\n", only)
			os.Exit(2)
		}
	}

	passed, total := 0, 0
	for _, sc := range scenarios {
		if only != "" && sc.group != only {
			continue
		}
		total++
		if err := runScenario(sc); err != nil {
			fmt.Printf("FAIL %s/%s  %s\n", sc.group, sc.name, err.Error())
			continue
		}
		passed++
		fmt.Printf("PASS %s/%s\n", sc.group, sc.name)
	}
	fmt.Printf("结果：通过 %d/%d\n", passed, total)
	if passed != total {
		os.Exit(1)
	}
}

func runScenario(sc scenario) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("场景正常结束（不 panic） 实际=panic: " + fmt.Sprint(r))
		}
	}()
	return sc.run()
}

// ---------------------------------------------------------------- 断言

type detailError struct{ msg string }

func (e *detailError) Error() string { return e.msg }

func expect(what, got string) error {
	return &detailError{msg: "期望=" + what + " 实际=" + got}
}

func expectSeq(what string, want, got []string) error {
	if len(want) == len(got) {
		same := true
		for i := range want {
			if want[i] != got[i] {
				same = false
				break
			}
		}
		if same {
			return nil
		}
	}
	return expect(fmt.Sprintf("%s=%v", what, want), fmt.Sprintf("%v", got))
}

// drain 一直推进直到队列清空，收集调度序列。
func drain(s *mlfq.Scheduler) []string {
	var out []string
	for s.Pending() > 0 {
		id, _ := s.Step()
		out = append(out, id)
	}
	return out
}

// ------------------------------------------------------------- semantics

func s1() error {
	s := mlfq.New(4, 1, 1000, nil)
	s.Submit("a", 2)
	s.Submit("b", 2)

	id, done := s.Step()
	if id != "a" || done {
		return expect("首步调度提交最早的任务 a 且未完成", fmt.Sprintf("(%q,%v)", id, done))
	}
	id, done = s.Step()
	if id != "b" || done {
		return expect("同级 FIFO：次步调度 b", fmt.Sprintf("(%q,%v)", id, done))
	}
	if n := s.Pending(); n != 2 {
		return expect("两个任务都还剩时间片，Pending=2", strconv.Itoa(n))
	}
	return nil
}

func s2() error {
	s := mlfq.New(3, 1, 1000, nil)
	s.Submit("x", 3)
	if id, done := s.Step(); id != "x" || done {
		return expect("首步调度 x 且未完成", fmt.Sprintf("(%q,%v)", id, done))
	}
	// x 已降一级；新提交的 y 在最高级，应先于 x。
	s.Submit("y", 1)
	if id, _ := s.Step(); id != "y" {
		return expect("用完时间片后 x 降级，y 应优先", id)
	}
	return nil
}

func s3() error {
	s := mlfq.New(2, 1, 2, nil)
	s.Submit("L", 4)
	s.Submit("s1", 1)
	s.Submit("s2", 1)
	s.Submit("s3", 1)
	// 每 2 步提升一次，被降级的 L 回到最高级队首，先于尚未降级的短任务。
	want := []string{"L", "s1", "L", "s2", "L", "s3", "L"}
	return expectSeq("提升后调度序列", want, drain(s))
}

// ------------------------------------------------------------------ cost

func c1() error {
	const (
		n     = 100000
		steps = 2000
		bound = 500000
	)
	c := &counter{}
	s := mlfq.New(4, 1, 1<<30, c) // 提升周期极大，测量窗口内不触发提升
	for i := 0; i < n; i++ {
		s.Submit(fmt.Sprintf("t%06d", i), 2)
	}
	for i := 0; i < steps; i++ {
		s.Step()
	}
	if c.n >= bound {
		return expect(fmt.Sprintf("前 %d 步计数 < %d", steps, bound), strconv.Itoa(c.n))
	}
	return nil
}

func c2() error {
	const (
		n     = 6000
		bound = 2000000
	)
	c := &counter{}
	s := mlfq.New(4, 1, 500, c)
	for i := 0; i < n; i++ {
		s.Submit(fmt.Sprintf("t%05d", i), 2)
	}
	steps := 0
	for s.Pending() > 0 && steps < 200000 {
		s.Step()
		steps++
	}
	if s.Pending() != 0 {
		return expect("全部任务完成", fmt.Sprintf("仍有 %d 个未完成", s.Pending()))
	}
	if c.n >= bound {
		return expect(fmt.Sprintf("含提升的整轮计数 < %d", bound), strconv.Itoa(c.n))
	}
	return nil
}

// ---------------------------------------------------------------- stable

func st1() error {
	build := func(c mlfq.Counter) *mlfq.Scheduler {
		s := mlfq.New(3, 1, 4, c)
		s.Submit("a", 5)
		s.Submit("b", 2)
		s.Submit("c", 7)
		s.Submit("d", 1)
		s.Submit("e", 3)
		return s
	}
	first := drain(build(nil))
	if len(first) == 0 {
		return expect("调度序列非空", "空序列")
	}
	second := drain(build(nil))
	if err := expectSeq("同一输入两次调度", first, second); err != nil {
		return err
	}
	third := drain(build(&counter{}))
	return expectSeq("是否注入计数器不影响序列", first, third)
}

func st2() error {
	s := mlfq.New(3, 1, 4, nil)
	s.Submit("x", 4)
	s.Step()
	s.Step()
	s.Reset()
	if n := s.Pending(); n != 0 {
		return expect("Reset 后 Pending=0", strconv.Itoa(n))
	}
	apply := func(sc *mlfq.Scheduler) {
		sc.Submit("p", 3)
		sc.Submit("q", 3)
	}
	fresh := mlfq.New(3, 1, 4, nil)
	apply(s)
	apply(fresh)
	return expectSeq("Reset 后与新建调度一致", drain(fresh), drain(s))
}

// ------------------------------------------------------------------ edge

func e1() error {
	s := mlfq.New(2, 1, 1000, nil)
	if id, done := s.Step(); id != "" || done {
		return expect("空队列 Step = (,false)", fmt.Sprintf("(%q,%v)", id, done))
	}
	if n := s.Pending(); n != 0 {
		return expect("空队列 Pending=0", strconv.Itoa(n))
	}
	s.Submit("only", 1)
	if id, done := s.Step(); id != "only" || !done {
		return expect("单任务首步即完成 = (only,true)", fmt.Sprintf("(%q,%v)", id, done))
	}
	if n := s.Pending(); n != 0 {
		return expect("完成后 Pending=0", strconv.Itoa(n))
	}
	if id, _ := s.Step(); id != "" {
		return expect("再次 Step 返回空", strconv.Quote(id))
	}
	return nil
}
