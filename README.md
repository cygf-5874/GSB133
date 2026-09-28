# mlfq

多级反馈队列（MLFQ）调度库（Go 1.24，仅标准库，`go.mod` 无 `require`）。

- 语言/依赖：Go 1.24，仅标准库，无第三方依赖。
- 模块名即库名：`mlfq`；库代码在仓库根（`mlfq.go`）。
- 自检：`bash scripts/check.sh`（`check/` 是固定验收程序，**勿改**）；构建：`bash scripts/build.sh`；
  既有用例：`go test ./...`。

## 怎么跑

```bash
go build ./...            # 或 bash scripts/build.sh
go test ./...             # 既有用例
go run ./check            # 固定验收程序（8 个场景）
go run ./check -list      # 列出全部场景
go run ./check --only cost
bash scripts/check.sh     # 薄封装：go run ./check "$@"
```

## 概念

调度器把任务放进若干优先级队列（级别 `0` 最高），每个任务有一个待消耗的代价
`cost`。调度器每次只推进一个任务一个时间片。

```go
s := mlfq.New(4, 1, 1000, nil) // 4 级，每片代价 1，每 1000 步提升一次，不注入计数器
s.Submit("a", 3)               // 放入最高级
id, done := s.Step()           // 推进一个时间片
```

## 对外契约

下面 9 条是 `mlfq` 的行为契约（`New` / `Submit` / `Step` / `Pending` / `Reset` 与
`Counter` 接口）。

1. **调度语义**：`Submit` 把任务放入最高优先级（级别 `0`）。`Step` 选择**当前非空的
   最高优先级**队列的队首任务，推进**一个时间片**，把它的剩余代价减去 `quantum`；
   若剩余 `> 0` 则降一级（已在最低级则留在原级），若剩余 `<= 0` 则移出队列并在返回值里
   标记完成。**同级按 FIFO**（提交早者先）。**提升**：每完成 `boostS` 个调度步，把所有
   任务提升回最高级 —— 按「当前级别从低到高、同级 FIFO」收集后，**整体插到最高级队列的
   头部**（先于尚未被降级的新任务），全部级别置为 `0`。
2. **代价计数**：判据通过注入的 `Counter` 度量调度代价 —— 实现每**检查 / 比较**一个任务
   或一个级别时调用 `counter.Add(1)`。**不得**依赖墙钟、`time` 或任何时基。
3. **规模**：`n = 100000` 个任务、固定提交序列下，**总会计数必须显著小于上界**
   （朴素的「每步全表扫描选最高级」约 `O(n²)`，会远超上界）。计数只允许随调度步数近似
   线性增长，不得随活动任务数线性增长。
4. **确定性**：同一操作序列的调度结果**逐项相同**；不得依赖 `map` / 容器迭代顺序。
5. **非抢占**：一次 `Step` 只推进一个任务的一个时间片，不一次跑完整个任务。
6. **边界**：空队列 `Step` 返回 `("", false)`；全部任务同级与「全部用完配额」都按第 1 条
   处理，不得 panic。
7. **`Reset`**：清空队列与内部计数，之后的状态与同参数的 `New` 完全一致。
8. **可复现**：同一输入序列在任意多次运行中产出**逐项相同**的调度序列。
9. **可配置提升周期**：`boostS` 可任意配置，只改变提升频率，不改变其余语义；`boostS <= 0`
   时按 `1` 处理。

## 固定件场景（8 个）

| 分组 | 场景 | 覆盖 |
| --- | --- | --- |
| `semantics` | `S1_new_task_highest_fifo` | 新任务进最高级、同级 FIFO |
| `semantics` | `S2_demote_after_quantum` | 用完时间片降级，新任务优先 |
| `semantics` | `S3_boost_front_of_highest` | 提升回到最高级队首 |
| `cost` | `C1_ops_independent_of_n` | 前 2000 步计数与任务规模无关 |
| `cost` | `C2_ops_bounded_with_boost` | 含提升的整轮计数受控 |
| `stable` | `S1_fixed_sequence_identical` | 固定输入两次序列相同、与是否注入计数器无关 |
| `stable` | `S2_reset_fresh_state` | `Reset` 后与新建一致 |
| `edge` | `E1_empty_and_single` | 空队列与单任务边界 |

`--only <组名>`（`semantics` / `cost` / `stable` / `edge`）可只跑一组。失败不早退。

## 目录

```
.
├── .gitattributes
├── .gitignore
├── go.mod               module mlfq
├── mlfq.go              已有实现（本次重构对象）
├── mlfq_test.go         既有用例
├── PROMPT.md            本题的 User Prompt（逐字）
├── README.md
├── check/main.go        固定验收程序（勿改）
└── scripts/
    ├── build.sh         go build ./...
    └── check.sh         薄封装：go run ./check "$@"
```