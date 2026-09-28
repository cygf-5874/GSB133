队列的推进顺序一上量就乱了。mlfq 是 Go 1.24 的多级反馈队列调度库，仅标准库，自检走 scripts/check.sh。
模块名即库名 `mlfq`，`go.mod` 不写 `require`；构建走 `bash scripts/build.sh`，既有用例走 `go test ./...`。

这份调度器已经能跑，既有用例全绿。请按 README「对外契约」的 9 条把它做到位，
让固定件 `check/` 的 8 个场景全过。语义细节以 README 为准。

验收：
- go test ./... 全绿；
- bash scripts/check.sh 退出码 0，8 个场景全过（semantics 3 + cost 2 + stable 2 + edge 1）。

约束：
1. 不改 `check/`；`Submit` / `Step` / `Pending` / `Reset` 与 `Counter` 接口的名字、签名不许改。
2. 既有用例一条都不许删改。
3. 仅标准库，不引入任何第三方依赖。
4. 结果与容器迭代顺序无关，判据全部走注入的计数器，不依赖墙钟。