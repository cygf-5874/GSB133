// Package mlfq 实现多级反馈队列（MLFQ）调度：
// 新任务进最高优先级，用完一个时间片降一级，低优先级每 boostS 个调度步
// 被提升回最高级（防止饥饿），同级按 FIFO。
//
// 完整语义见 README 的「对外契约」一节。
package mlfq

// Counter 是调度代价的注入点。实现每次「检查 / 比较」一个任务或一个级别时
// 都应调用 Add，供调用方度量调度代价；允许为 nil。
type Counter interface {
	Add(n int)
}

type item struct {
	id     string
	remain int
	level  int
	seq    int
}

// Scheduler 是多级反馈队列调度器。
type Scheduler struct {
	levels  int
	quantum int
	boostS  int
	counter Counter

	items   []*item
	steps   int
	nextSeq int
}

// New 创建调度器。
//
//	levels  优先级级数（>= 1），级别 0 最高；
//	quantum 每个时间片的代价单位（>= 1）；
//	boostS  每多少个调度步把低优先级任务提升回最高级（>= 1）；
//	counter 代价计数器，可为 nil。
func New(levels, quantum, boostS int, counter Counter) *Scheduler {
	if levels < 1 {
		levels = 1
	}
	if quantum < 1 {
		quantum = 1
	}
	if boostS < 1 {
		boostS = 1
	}
	return &Scheduler{levels: levels, quantum: quantum, boostS: boostS, counter: counter}
}

// Reset 清空队列与计数，状态与同参数的 New 一致。
func (s *Scheduler) Reset() {
	s.items = nil
	s.steps = 0
	s.nextSeq = 0
}

// Submit 把任务放入最高优先级（级别 0）。
func (s *Scheduler) Submit(id string, cost int) {
	s.items = append(s.items, &item{id: id, remain: cost, level: 0, seq: s.nextSeq})
	s.nextSeq++
}

// Pending 返回仍在队列里的任务数。
func (s *Scheduler) Pending() int {
	return len(s.items)
}

// Step 推进一个时间片：返回本步调度的任务 ID，以及它是否在此步完成。
// 队列为空时返回 ("", false)。
func (s *Scheduler) Step() (string, bool) {
	if len(s.items) == 0 {
		return "", false
	}

	// 选当前最高优先级（level 最小）的任务；同级取提交最早者（FIFO）。
	best := -1
	for i, it := range s.items {
		if s.counter != nil {
			s.counter.Add(1)
		}
		if best == -1 || it.level < s.items[best].level {
			best = i
		}
	}

	it := s.items[best]
	it.remain -= s.quantum
	id := it.id
	done := it.remain <= 0

	if done {
		s.items = append(s.items[:best], s.items[best+1:]...)
	} else if it.level < s.levels-1 {
		it.level++
	}

	s.steps++
	if s.steps%s.boostS == 0 {
		s.boost()
	}
	return id, done
}

// boost 把所有任务提升回最高级：按「当前级别从低到高、同级 FIFO」收集，
// 整体插到最高级队列的头部（先于尚未被降级的新任务）。
func (s *Scheduler) boost() {
	collected := make([]*item, 0, len(s.items))
	for lvl := s.levels - 1; lvl >= 0; lvl-- {
		for _, it := range s.items {
			if s.counter != nil {
				s.counter.Add(1)
			}
			if it.level == lvl {
				collected = append(collected, it)
			}
		}
	}
	for _, it := range collected {
		it.level = 0
	}
	s.items = collected
}
