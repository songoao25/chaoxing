package main

// bridge_test.go 覆盖 Python 桥客户端的 framing、id 匹配、超时、事件转发、
// stderr 环形缓冲、崩溃自动重启与关闭清理。
//
// 测试用“Go 子进程替身”充当 Python 桥：把当前测试二进制以
// -test.run=TestBridgeHelperProcess 启动，环境变量 CX_TEST_MODE 控制行为。
// 不依赖真实 Python，也不联网。

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---- 假桥子进程 ----

// TestBridgeHelperProcess 平时被跳过；作为假桥子进程运行时执行 NDJSON 服务。
func TestBridgeHelperProcess(t *testing.T) {
	if os.Getenv("CX_TEST_HELPER") != "1" {
		t.Skip("仅作为假桥子进程入口")
	}
	runBridgeHelper()
	os.Exit(0)
}

func incrementCounter(dir string) int {
	path := filepath.Join(dir, "starts.txt")
	n := 0
	if data, err := os.ReadFile(path); err == nil {
		n, _ = strconv.Atoi(strings.TrimSpace(string(data)))
	}
	n++
	_ = os.WriteFile(path, []byte(strconv.Itoa(n)), 0o644)
	return n
}

func writeHelperEvents(write func(any)) {
	write(map[string]any{
		"event": "run.log",
		"data": map[string]any{
			"run_id": "r-test",
			"level":  "info",
			"text":   "hello from fake bridge",
			"ts":     "2026-11-04 21:00:00",
		},
	})
	write(map[string]any{
		"event": "run.started",
		"data": map[string]any{
			"run_id":          "r-test",
			"started_at":      "2026-11-04 21:00:00",
			"account_id":      "13800000001",
			"username_masked": "138****0001",
			"courses":         []any{},
			"scope":           map[string]any{},
		},
	})
}

func runBridgeHelper() {
	mode := os.Getenv("CX_TEST_MODE")
	startCount := 0
	if dir := os.Getenv("CX_TEST_COUNTER_DIR"); dir != "" {
		startCount = incrementCounter(dir)
	}
	out := bufio.NewWriter(os.Stdout)
	enc := json.NewEncoder(out)
	write := func(v any) {
		_ = enc.Encode(v)
		_ = out.Flush()
	}
	writeFile := func(name, content string) {
		dir := os.Getenv("CX_TEST_MARKER_DIR")
		if dir == "" {
			return
		}
		_ = os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	}
	mockSettings := map[string]any{
		"speed":                   1.25,
		"jobs":                    4,
		"notopen_action":          "retry",
		"retry_interval":          2.0,
		"serial_video":            true,
		"add_learning_count":      true,
		"target_count":            42,
		"work_redo_enabled":       true,
		"work_max_retries":        5,
		"task_center_submit_mode": "auto",
	}

	switch mode {
	case "crash-immediately":
		fmt.Fprintln(os.Stderr, "假桥：启动即退出（测试）")
		os.Exit(3)
	case "crash-once":
		if startCount == 1 {
			fmt.Fprintln(os.Stderr, "假桥：第一次启动故意退出（测试）")
			os.Exit(3)
		}
		writeHelperEvents(write)
	case "echo":
		writeHelperEvents(write)
	case "garbage-first":
		writeHelperEvents(write)
		fmt.Fprintln(out, "this is not a json frame")
		_ = out.Flush()
	case "stderr":
		for i := 0; i < 250; i++ {
			fmt.Fprintf(os.Stderr, "diag-%03d\n", i)
		}
	case "stderr-cr-flood":
		// 模拟 tqdm： 刷新 3 万条进度（无换行），随后一条超长行 + 正常行。
		bw := bufio.NewWriterSize(os.Stderr, 64*1024)
		for i := 0; i < 30000; i++ {
			fmt.Fprintf(bw, "\rtqdm %06d %s", i, strings.Repeat("x", 80))
		}
		bw.WriteString("\n")
		bw.WriteString("\r")
		bw.WriteString(strings.Repeat("y", 20000))
		bw.WriteString("\r")
		bw.WriteString("tail-marker\n")
		_ = bw.Flush()
	}

	if mode == "silent" {
		// 读取但永远不响应，直到 stdin 关闭。
		buf := make([]byte, 4096)
		for {
			if _, err := os.Stdin.Read(buf); err != nil {
				return
			}
		}
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), bridgeMaxLineBytes)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var req struct {
			ID     string          `json:"id"`
			Cmd    string          `json:"cmd"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		writeEcho := func() {
			write(map[string]any{
				"id": req.ID,
				"ok": true,
				"result": map[string]any{
					"cmd":    req.Cmd,
					"params": req.Params,
				},
			})
		}
		writeSettings := func() {
			write(map[string]any{
				"id":     req.ID,
				"ok":     true,
				"result": map[string]any{"settings": mockSettings},
			})
		}
		writeRunStarted := func(runID string) {
			write(map[string]any{
				"event": "run.started",
				"data": map[string]any{
					"run_id":          runID,
					"started_at":      "2026-11-04 21:00:00",
					"account_id":      "13800000001",
					"username_masked": "138****0001",
					"courses":         []any{},
					"scope":           map[string]any{},
				},
			})
		}

		switch mode {
		case "error":
			write(map[string]any{
				"id": req.ID,
				"ok": false,
				"error": map[string]any{
					"code":    "busy",
					"message": "已有刷课任务在运行，请稍后再试。",
				},
			})
		case "large":
			write(map[string]any{
				"id": req.ID,
				"ok": true,
				"result": map[string]any{
					"big": strings.Repeat("x", 700*1024),
				},
			})
		case "crash-after-run-start":
			switch req.Cmd {
			case "settings.get":
				writeSettings()
			case "run.start":
				write(map[string]any{
					"id": req.ID,
					"ok": true,
					"result": map[string]any{
						"run_id":     "r-crash",
						"started_at": "2026-11-04 21:00:00",
					},
				})
				writeRunStarted("r-crash")
				_ = out.Flush()
				fmt.Fprintln(os.Stderr, "假桥：运行开始后故意崩溃（测试）")
				os.Exit(4)
			default:
				writeEcho()
			}
		case "record-params":
			safe := strings.NewReplacer(".", "_", "/", "_").Replace(req.Cmd)
			writeFile("params-"+safe+".json", string(req.Params))
			writeEcho()
		case "record-run-params":
			switch req.Cmd {
			case "settings.get":
				writeSettings()
			case "run.start":
				writeFile("run-params.json", string(req.Params))
				write(map[string]any{
					"id": req.ID,
					"ok": true,
					"result": map[string]any{
						"run_id":     "r-recorded",
						"started_at": "2026-11-04 21:00:00",
					},
				})
				writeRunStarted("r-recorded")
			default:
				writeEcho()
			}
		case "run-stop-graceful":
			switch req.Cmd {
			case "run.start":
				write(map[string]any{
					"id": req.ID,
					"ok": true,
					"result": map[string]any{
						"run_id":     "r-graceful",
						"started_at": "2026-11-04 21:00:00",
					},
				})
				writeRunStarted("r-graceful")
			case "run.stop":
				writeFile("stop-requested.txt", "1")
				write(map[string]any{
					"id":     req.ID,
					"ok":     true,
					"result": map[string]any{"stopping": true},
				})
			default:
				writeEcho()
			}
		default:
			writeEcho()
		}
	}
}

// ---- 测试辅助 ----

// bridgeProbe 记录桥回调，供断言使用。
type bridgeProbe struct {
	mu       sync.Mutex
	events   []string
	eventRaw []json.RawMessage
	states   []string
}

func (p *bridgeProbe) attach(b *Bridge) {
	b.SetHandlers(func(event string, data json.RawMessage) {
		p.mu.Lock()
		p.events = append(p.events, event)
		raw := make(json.RawMessage, len(data))
		copy(raw, data)
		p.eventRaw = append(p.eventRaw, raw)
		p.mu.Unlock()
	}, func(state, message string) {
		p.mu.Lock()
		p.states = append(p.states, state)
		p.mu.Unlock()
	})
}

func (p *bridgeProbe) eventNames() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.events...)
}

func (p *bridgeProbe) eventPayloads() []json.RawMessage {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]json.RawMessage(nil), p.eventRaw...)
}

func (p *bridgeProbe) hasState(state string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.states {
		if s == state {
			return true
		}
	}
	return false
}

// newTestBridge 创建一个以当前测试二进制为假桥的 Bridge。
func newTestBridge(t *testing.T, mode string, extraEnv ...string) *Bridge {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("获取测试可执行文件失败：%v", err)
	}
	repo := t.TempDir()
	b := NewBridge(repo, "")
	b.argv = []string{exe, "-test.run=TestBridgeHelperProcess"}
	b.restartDelay = 20 * time.Millisecond
	b.extraEnv = append([]string{"CX_TEST_HELPER=1", "CX_TEST_MODE=" + mode}, extraEnv...)
	t.Cleanup(b.Close)
	return b
}

// waitFor 轮询等待条件成立。
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

// ---- 测试用例 ----

func TestRingBufferCapAndOrder(t *testing.T) {
	rb := newRingBuffer(3)
	for i := 0; i < 5; i++ {
		rb.Add(fmt.Sprintf("line-%d", i))
	}
	all := rb.Tail(0)
	if len(all) != 3 {
		t.Fatalf("环形缓冲长度 = %d，期望 3", len(all))
	}
	want := []string{"line-2", "line-3", "line-4"}
	for i := range want {
		if all[i] != want[i] {
			t.Errorf("Tail[%d] = %q，期望 %q", i, all[i], want[i])
		}
	}
	two := rb.Tail(2)
	if len(two) != 2 || two[0] != "line-3" || two[1] != "line-4" {
		t.Errorf("Tail(2) = %v，期望 [line-3 line-4]", two)
	}
}

func TestBridgeCallAndEventForwarding(t *testing.T) {
	b := newTestBridge(t, "echo")
	probe := &bridgeProbe{}
	probe.attach(b)

	raw, err := b.Call("ping", map[string]any{"n": 1}, 3*time.Second)
	if err != nil {
		t.Fatalf("Call 失败：%v", err)
	}
	var result struct {
		Cmd    string         `json:"cmd"`
		Params map[string]any `json:"params"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("解析响应失败：%v（原始：%s）", err, raw)
	}
	if result.Cmd != "ping" {
		t.Errorf("响应 cmd = %q，期望 ping", result.Cmd)
	}
	if n, ok := result.Params["n"].(float64); !ok || n != 1 {
		t.Errorf("响应 params 未原样回传：%v", result.Params)
	}

	if !waitFor(t, 2*time.Second, func() bool { return len(probe.eventNames()) >= 2 }) {
		t.Fatalf("未收到事件转发，实际：%v", probe.eventNames())
	}
	names := probe.eventNames()
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "run.log") || !strings.Contains(joined, "run.started") {
		t.Errorf("事件名不正确：%v", names)
	}
	payloads := probe.eventPayloads()
	if len(payloads) == 0 || !bytes.Contains(payloads[0], []byte("hello from fake bridge")) {
		t.Errorf("事件 payload 未完整转发：%s", payloads)
	}
	if !waitFor(t, 2*time.Second, func() bool { return probe.hasState(bridgeStateReady) }) {
		t.Errorf("未观察到 ready 状态，实际状态：%v", probe.states)
	}
}

func TestBridgeConcurrentIDMatching(t *testing.T) {
	b := newTestBridge(t, "echo")
	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			raw, err := b.Call("ping", map[string]int{"n": i}, 3*time.Second)
			if err != nil {
				errs <- fmt.Errorf("第 %d 个请求失败：%w", i, err)
				return
			}
			var result struct {
				Params map[string]int `json:"params"`
			}
			if err := json.Unmarshal(raw, &result); err != nil {
				errs <- fmt.Errorf("第 %d 个响应解析失败：%w", i, err)
				return
			}
			if result.Params["n"] != i {
				errs <- fmt.Errorf("第 %d 个响应串了：%v", i, result.Params)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestBridgeTimeout(t *testing.T) {
	b := newTestBridge(t, "silent")
	start := time.Now()
	_, err := b.Call("ping", nil, 200*time.Millisecond)
	if err == nil {
		t.Fatal("假桥不响应时应超时")
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Errorf("超时错误应为中文，实际：%v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("超时耗时过长：%v", elapsed)
	}
}

func TestBridgeErrorFrame(t *testing.T) {
	b := newTestBridge(t, "error")
	_, err := b.Call("run.start", map[string]any{}, 3*time.Second)
	if err == nil {
		t.Fatal("错误帧应返回 error")
	}
	if !strings.Contains(err.Error(), "已有刷课任务在运行") {
		t.Errorf("错误文案应来自桥的 message，实际：%v", err)
	}
}

func TestBridgeCrashRestart(t *testing.T) {
	counterDir := t.TempDir()
	b := newTestBridge(t, "crash-once", "CX_TEST_COUNTER_DIR="+counterDir)
	probe := &bridgeProbe{}
	probe.attach(b)

	// 第一次请求会撞上假桥故意崩溃。
	if _, err := b.Call("ping", nil, 3*time.Second); err == nil {
		t.Fatal("第一次请求应因假桥崩溃而失败")
	}
	if !waitFor(t, 2*time.Second, func() bool { return probe.hasState(bridgeStateRestarting) }) {
		t.Errorf("未观察到 restarting 状态，实际：%v", probe.states)
	}

	// 自动重启后应恢复可用。
	var lastErr error
	ok := waitFor(t, 5*time.Second, func() bool {
		_, lastErr = b.Call("ping", nil, 2*time.Second)
		return lastErr == nil
	})
	if !ok {
		t.Fatalf("崩溃后未自动恢复：%v（状态：%v）", lastErr, probe.states)
	}
	if !waitFor(t, 2*time.Second, func() bool { return probe.hasState(bridgeStateReady) }) {
		t.Errorf("恢复后未观察到 ready 状态：%v", probe.states)
	}
	data, err := os.ReadFile(filepath.Join(counterDir, "starts.txt"))
	if err != nil {
		t.Fatalf("读取启动计数失败：%v", err)
	}
	starts, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if starts < 2 {
		t.Errorf("假桥启动次数 = %d，期望至少 2（崩溃后重启）", starts)
	}
}

func TestBridgeCrashExhaustsRestarts(t *testing.T) {
	counterDir := t.TempDir()
	b := newTestBridge(t, "crash-immediately", "CX_TEST_COUNTER_DIR="+counterDir)
	probe := &bridgeProbe{}
	probe.attach(b)

	if _, err := b.Call("ping", nil, 5*time.Second); err == nil {
		t.Fatal("桥持续崩溃时请求应失败")
	}
	if !waitFor(t, 5*time.Second, func() bool {
		state, _ := b.State()
		return state == bridgeStateFailed
	}) {
		state, message := b.State()
		t.Fatalf("重试 3 次后应进入 failed 状态，实际状态=%q 消息=%q", state, message)
	}
	if !probe.hasState(bridgeStateFailed) {
		t.Errorf("未通过状态回调收到 failed，实际：%v", probe.states)
	}
	data, _ := os.ReadFile(filepath.Join(counterDir, "starts.txt"))
	starts, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if starts < 4 {
		t.Errorf("假桥启动次数 = %d，期望初始 1 次 + 重启 3 次", starts)
	}
}

func TestBridgeStderrRingBuffer(t *testing.T) {
	b := newTestBridge(t, "stderr")
	if _, err := b.Call("ping", nil, 3*time.Second); err != nil {
		t.Fatalf("Call 失败：%v", err)
	}
	if !waitFor(t, 2*time.Second, func() bool {
		lines := b.StderrTail(0)
		return len(lines) == bridgeStderrRingLines && lines[len(lines)-1] == "diag-249"
	}) {
		lines := b.StderrTail(0)
		t.Fatalf("stderr 环形缓冲应保留最近 %d 行且末行为 diag-249，实际 %d 行：%v",
			bridgeStderrRingLines, len(lines), lastLines(lines, 3))
	}
	tail := b.StderrTail(2)
	if len(tail) != 2 || tail[0] != "diag-248" || tail[1] != "diag-249" {
		t.Errorf("StderrTail(2) = %v", tail)
	}
}

func TestBridgeLargeFrame(t *testing.T) {
	b := newTestBridge(t, "large")
	raw, err := b.Call("ping", nil, 5*time.Second)
	if err != nil {
		t.Fatalf("大帧请求失败：%v", err)
	}
	var result struct {
		Big string `json:"big"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("大帧解析失败：%v", err)
	}
	if len(result.Big) != 700*1024 {
		t.Errorf("大帧长度 = %d，期望 %d", len(result.Big), 700*1024)
	}
}

func TestBridgeGarbageStdoutLine(t *testing.T) {
	b := newTestBridge(t, "garbage-first")
	probe := &bridgeProbe{}
	probe.attach(b)
	if _, err := b.Call("ping", nil, 3*time.Second); err != nil {
		t.Fatalf("非协议行不应影响后续请求：%v", err)
	}
	if !waitFor(t, 2*time.Second, func() bool {
		for _, line := range b.StderrTail(0) {
			if strings.Contains(line, "不是合法 JSON") {
				return true
			}
		}
		return false
	}) {
		t.Errorf("非协议行应记入 stderr 诊断缓冲，实际：%v", b.StderrTail(3))
	}
}

func TestBridgeCloseRejectsPending(t *testing.T) {
	b := newTestBridge(t, "silent")
	errCh := make(chan error, 1)
	go func() {
		_, err := b.Call("ping", nil, 10*time.Second)
		errCh <- err
	}()
	time.Sleep(200 * time.Millisecond) // 等请求写入/进程启动
	b.Close()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("关闭桥后未完成请求应返回错误")
		}
		if !strings.Contains(err.Error(), "关闭") && !strings.Contains(err.Error(), "退出") {
			t.Errorf("关闭错误应为中文提示，实际：%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("关闭桥后未完成请求没有被及时拒绝")
	}
}

func lastLines(lines []string, n int) []string {
	if n > len(lines) {
		n = len(lines)
	}
	return lines[len(lines)-n:]
}

func TestBridgeStderrLongCarriageReturnStream(t *testing.T) {
	b := newTestBridge(t, "stderr-cr-flood")
	start := time.Now()
	if _, err := b.Call("ping", nil, 10*time.Second); err != nil {
		t.Fatalf("超长 \\r stderr 流不应阻塞请求：%v", err)
	}
	// 再发一次，证明进程没有被 stderr 管道反压卡死。
	if _, err := b.Call("ping", nil, 5*time.Second); err != nil {
		t.Fatalf("第二次请求失败（stderr 可能未持续排空）：%v", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("超长 stderr 流处理耗时过长：%v", elapsed)
	}
	if !waitFor(t, 3*time.Second, func() bool {
		lines := b.StderrTail(0)
		if len(lines) == 0 {
			return false
		}
		return lines[len(lines)-1] == "tail-marker"
	}) {
		t.Fatalf("stderr 环形缓冲未收到末尾行，最后 3 行：%v", lastLines(b.StderrTail(0), 3))
	}
	lines := b.StderrTail(0)
	if len(lines) > bridgeStderrRingLines {
		t.Errorf("环形缓冲行数 = %d，超过上限 %d", len(lines), bridgeStderrRingLines)
	}
	maxLen := 0
	hasTqdm := false
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
		if strings.Contains(line, "tqdm 029999") {
			hasTqdm = true
		}
	}
	if maxLen > bridgeStderrLineMax {
		t.Errorf("单行未截断：最长 %d 字节，上限 %d", maxLen, bridgeStderrLineMax)
	}
	if !hasTqdm {
		t.Errorf("环形缓冲应保留最近的 tqdm 进度行，实际末几行：%v", lastLines(lines, 3))
	}

	// 关闭应能在优雅收尾预算内完成（进程需自行退出，而不是被卡住）。
	b.Close()
	if state, _ := b.State(); state != bridgeStateStopped {
		t.Errorf("Close 后状态 = %q，期望 stopped", state)
	}
}

func TestBridgeConsecutiveTimeoutsRestartBridge(t *testing.T) {
	counterDir := t.TempDir()
	b := newTestBridge(t, "silent", "CX_TEST_COUNTER_DIR="+counterDir)

	for i := 0; i < 2; i++ {
		if _, err := b.Call("ping", nil, 200*time.Millisecond); err == nil {
			t.Fatalf("第 %d 次请求应超时", i+1)
		}
	}
	if !waitFor(t, 3*time.Second, func() bool {
		data, err := os.ReadFile(filepath.Join(counterDir, "starts.txt"))
		if err != nil {
			return false
		}
		n, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		return n >= 2
	}) {
		t.Fatal("连续 2 次超时后应主动重启桥进程")
	}
	state, _ := b.State()
	if state != bridgeStateRestarting && state != bridgeStateStarting && state != bridgeStateReady {
		t.Errorf("看门狗重启后状态不合理：%q", state)
	}
}

func TestBridgeCloseSendsRunStopBeforeExit(t *testing.T) {
	markerDir := t.TempDir()
	b := newTestBridge(t, "run-stop-graceful", "CX_TEST_MARKER_DIR="+markerDir)

	res, err := b.StartRun(RunOptions{
		AccountID: "13800000001",
		CourseIDs: []string{"2151141"},
		Scope:     RunScope{Chapters: true, TaskCenter: true},
	})
	if err != nil {
		t.Fatalf("run.start 失败：%v", err)
	}
	if res.RunID != "r-graceful" {
		t.Errorf("run_id = %q，期望 r-graceful", res.RunID)
	}
	// 等 Bridge 通过 run.started 事件感知到运行中。
	if !waitFor(t, 3*time.Second, func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.runActive
	}) {
		t.Fatal("Bridge 未通过 run.started 事件标记运行中")
	}

	b.Close()

	data, err := os.ReadFile(filepath.Join(markerDir, "stop-requested.txt"))
	if err != nil {
		t.Fatalf("Close 应在 Kill 前 best-effort 发送 run.stop：%v", err)
	}
	if strings.TrimSpace(string(data)) != "1" {
		t.Errorf("run.stop 标记内容异常：%q", string(data))
	}
}

func TestBridgeListReviewsLimit(t *testing.T) {
	markerDir := t.TempDir()
	b := newTestBridge(t, "record-params", "CX_TEST_MARKER_DIR="+markerDir)

	readLimit := func(days int) map[string]any {
		path := filepath.Join(markerDir, "params-reviews_list.json")
		_ = os.Remove(path)
		if _, err := b.ListReviews(days); err != nil {
			t.Fatalf("ListReviews(%d) 失败：%v", days, err)
		}
		var params map[string]any
		if !waitFor(t, 3*time.Second, func() bool {
			data, err := os.ReadFile(path)
			if err != nil {
				return false
			}
			return json.Unmarshal(data, &params) == nil
		}) {
			t.Fatalf("假桥未记录 reviews.list 参数（days=%d）", days)
		}
		return params
	}

	all := readLimit(0)
	if all["days"] != 0.0 || all["limit"] != 500.0 {
		t.Errorf("days=0（全部）参数 = %v，期望 days=0 limit=500", all)
	}
	recent := readLimit(3)
	if recent["days"] != 3.0 || recent["limit"] != 50.0 {
		t.Errorf("days=3 参数 = %v，期望 days=3 limit=50", recent)
	}
}
