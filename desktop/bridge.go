package main

// bridge.go 是 Python 桥客户端（进程管理器）。
//
// 协议见 desktop/docs/INTERFACE.md §1–§4：
//   - 启动： [python] -u -m api.desktop_bridge，cwd=引擎仓库，继承环境变量并强制
//     PYTHONUNBUFFERED=1 / PYTHONIOENCODING=utf-8；
//   - stdin/stdout 走 NDJSON（每行一个 JSON 对象，单行上限 1 MiB）；
//   - stdout 只解析协议行，stderr 收进 200 行环形缓冲区（排障用，不刷 UI）；
//   - 请求 id 自增，pending map + 每命令超时；响应按 id 匹配；
//   - 桥进程崩溃/EOF 时按指数退避自动重启，最多 3 次，未完成请求用中文 error 拒绝；
//   - 生命周期通过 bridge:state 事件通知前端（starting/ready/restarting/failed）。
//
// 本类型线程安全，可直接被 Wails 绑定的多个方法并发调用。

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	bridgeMaxLineBytes    = 1024 * 1024 // 契约 §1：单行上限 1 MiB
	bridgeStderrRingLines = 200
	bridgeMaxRestarts     = 3
	// stderr 单行截断长度：tqdm 进度行不需要整行，但要继续排空管道。
	bridgeStderrLineMax = 8 * 1024
	// 连续命令超时达到该次数后主动重启桥（防止桥假死）。
	bridgeMaxConsecutiveTimeouts = 2
	// 关闭时的优雅收尾预算（P2 要求：run.stop ≤1.5s，等进程退出 ≤2.5s）。
	bridgeGracefulStopTimeout = 1500 * time.Millisecond
	bridgeCloseDrainTimeout   = 2500 * time.Millisecond
	bridgeKillWaitTimeout     = 2 * time.Second
	bridgeDefaultTimeout      = 30 * time.Second
	bridgePingTimeout         = 10 * time.Second
	bridgeCoursesTimeout      = 120 * time.Second
	bridgeRunStartTimeout     = 30 * time.Second
)

// 桥生命周期状态（bridge:state 事件与 GetStatus.bridge_state 共用）。
const (
	bridgeStateStarting   = "starting"
	bridgeStateReady      = "ready"
	bridgeStateRestarting = "restarting"
	bridgeStateFailed     = "failed"
	bridgeStateStopped    = "stopped"
)

// commandTimeouts 返回命令对应的超时（契约 §1）。
func commandTimeout(cmd string) time.Duration {
	switch cmd {
	case "ping":
		return bridgePingTimeout
	case "courses.list":
		return bridgeCoursesTimeout
	case "run.start":
		return bridgeRunStartTimeout
	default:
		return bridgeDefaultTimeout
	}
}

// bridgeError 是桥返回的错误帧（契约 §1）。
type bridgeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
}

// Error 给前端的是中文可读文案：优先用桥返回的 message，
// 缺失时按错误码映射；detail 只在 message 为空时作为补充，避免泄露内部细节。
func (e *bridgeError) Error() string {
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = defaultBridgeErrorMessage(e.Code)
	}
	return msg
}

func defaultBridgeErrorMessage(code string) string {
	switch code {
	case "bad_request":
		return "请求参数有误，请检查输入后重试。"
	case "not_found":
		return "未找到对应的账号或课程。"
	case "busy":
		return "已有刷课任务在运行，请稍后再试。"
	case "engine_error":
		return "引擎内部异常，请查看运行日志后重试。"
	case "login_failed":
		return "登录失败，请检查账号密码或稍后重试。"
	case "network_error":
		return "网络请求失败，请检查网络后重试。"
	case "config_error":
		return "配置读写失败，请检查配置文件和目录权限。"
	case "not_ready":
		return "Python 桥尚未就绪，请稍后重试。"
	case "timeout":
		return "Python 桥响应超时，请稍后重试。"
	default:
		return "Python 桥内部错误，请稍后重试。"
	}
}

// bridgeFrame 同时覆盖响应帧（有 id）与事件帧（有 event）。
type bridgeFrame struct {
	ID     string          `json:"id"`
	OK     *bool           `json:"ok"`
	Result json.RawMessage `json:"result"`
	Error  *bridgeError    `json:"error"`
	Event  string          `json:"event"`
	Data   json.RawMessage `json:"data"`
}

type bridgeReply struct {
	result json.RawMessage
	err    error
}

// ringBuffer 是固定行数的环形缓冲区，给 stderr 用（最近 N 行）。
type ringBuffer struct {
	mu    sync.Mutex
	lines []string
	max   int
}

func newRingBuffer(max int) *ringBuffer {
	if max <= 0 {
		max = bridgeStderrRingLines
	}
	return &ringBuffer{max: max}
}

func (r *ringBuffer) Add(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.lines) >= r.max {
		copy(r.lines, r.lines[1:])
		r.lines[len(r.lines)-1] = line
		return
	}
	r.lines = append(r.lines, line)
}

// Tail 返回最近 n 行副本（n<=0 表示全部）。
func (r *ringBuffer) Tail(n int) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	lines := r.lines
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	out := make([]string, len(lines))
	copy(out, lines)
	return out
}

// Bridge 管理一个 Python 桥子进程。
type Bridge struct {
	repo   string
	python string

	// 测试注入：argv 非空时替代默认 [python -u -m api.desktop_bridge]；
	// extraEnv 追加到子进程环境；restartDelay 控制首次重启退避。
	argv         []string
	extraEnv     []string
	restartDelay time.Duration

	mu           sync.Mutex
	writeMu      sync.Mutex
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	pending      map[string]chan bridgeReply
	nextID       uint64
	state        string
	stateMessage string
	stopped      bool
	starting     bool
	ready        bool
	closing      bool // Close 已开始，禁止重启/新请求
	runActive    bool // 通过 run.started/run.finished 维护，Close 时决定是否先 run.stop
	timeouts     int  // 连续命令超时次数
	restarts     int
	stderr       *ringBuffer
	procDone     chan struct{}
	onEvent      func(event string, data json.RawMessage)
	onState      func(state, message string)
}

// NewBridge 创建一个未启动的桥客户端。
func NewBridge(repo, python string) *Bridge {
	return &Bridge{
		repo:         repo,
		python:       python,
		restartDelay: 300 * time.Millisecond,
		pending:      make(map[string]chan bridgeReply),
		stderr:       newRingBuffer(bridgeStderrRingLines),
		state:        bridgeStateStopped,
	}
}

// SetHandlers 注册事件与状态回调（应在 Start 之前调用）。传 nil 可取消。
func (b *Bridge) SetHandlers(onEvent func(event string, data json.RawMessage), onState func(state, message string)) {
	b.mu.Lock()
	b.onEvent = onEvent
	b.onState = onState
	b.mu.Unlock()
}

// State 返回当前桥状态与说明文案。
func (b *Bridge) State() (state, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state, b.stateMessage
}

// StderrTail 返回最近 n 行 stderr（排障用）。
func (b *Bridge) StderrTail(n int) []string {
	return b.stderr.Tail(n)
}

// setState 更新状态；状态变化时调用 onState（锁外调用，避免回调重入死锁）。
func (b *Bridge) setState(state, message string) {
	b.mu.Lock()
	changed := b.state != state || b.stateMessage != message
	b.state = state
	b.stateMessage = message
	handler := b.onState
	b.mu.Unlock()
	if changed && handler != nil {
		handler(state, message)
	}
}

// buildEnv 组装子进程环境：继承 os.Environ()，追加测试额外变量，
// 再覆盖 PYTHONUNBUFFERED / PYTHONIOENCODING（契约 §0）。
func (b *Bridge) buildEnv() []string {
	env := os.Environ()
	env = append(env, b.extraEnv...)
	env = append(env, "PYTHONUNBUFFERED=1", "PYTHONIOENCODING=utf-8")
	return env
}

// Start 启动桥进程（幂等）。进程已在运行或正在启动时直接返回 nil。
func (b *Bridge) Start() error {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return errors.New("Python 桥已关闭")
	}
	if b.cmd != nil || b.starting {
		b.mu.Unlock()
		return nil
	}
	b.starting = true
	b.mu.Unlock()

	b.setState(bridgeStateStarting, "正在启动 Python 桥…")
	err := b.startProcess()

	b.mu.Lock()
	b.starting = false
	b.mu.Unlock()
	if err != nil {
		b.setState(bridgeStateFailed, err.Error())
		return err
	}
	return nil
}

// startProcess 拉起子进程并启动读循环。调用方负责状态文案。
func (b *Bridge) startProcess() error {
	argv := b.argv
	if len(argv) == 0 {
		if b.python == "" {
			return errors.New("未指定 Python 路径，无法启动引擎")
		}
		argv = []string{b.python, "-u", "-m", "api.desktop_bridge"}
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = b.repo
	cmd.Env = b.buildEnv()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("创建 Python 桥输入通道失败：%v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建 Python 桥输出通道失败：%v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建 Python 桥错误通道失败：%v", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 Python 桥失败：请检查 Python 路径与引擎目录是否有效（%v）", err)
	}

	done := make(chan struct{})
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		_ = cmd.Process.Kill()
		return errors.New("Python 桥已关闭")
	}
	b.cmd = cmd
	b.stdin = stdin
	b.procDone = done
	b.ready = false
	b.mu.Unlock()

	go b.readStderr(stderr)
	go b.readLoop(cmd, stdout, done)
	return nil
}

// readStderr 持续读取子进程 stderr 并写入环形缓冲。
//
// 不能用 bufio.Scanner：引擎的 tqdm 进度条用 \r 刷新且 leave=False 时不写 \n，
// 长时间运行累计超过 Scanner 单行上限后会返回 ErrTooLong 并停止读取；
// 一旦停止读，stderr 管道被写满，引擎线程就会阻塞 -> 刷课卡死。
//
// 因此这里按 \r / \n 双分隔手动切分：单行截断到 bridgeStderrLineMax 后仍继续排空，
// 读取出错（非 EOF）也继续 io.Copy(io.Discard) 排空；始终只保留最近 200 行。
func (b *Bridge) readStderr(r io.Reader) {
	reader := bufio.NewReaderSize(r, 64*1024)
	var line []byte
	truncated := false
	flush := func() {
		if len(line) > 0 {
			b.stderr.Add(string(line))
		}
		line = line[:0]
		truncated = false
	}
	appendPart := func(part []byte) {
		if truncated {
			return
		}
		remain := bridgeStderrLineMax - len(line)
		if remain <= 0 {
			truncated = true
			return
		}
		if len(part) > remain {
			part = part[:remain]
		}
		line = append(line, part...)
		if len(line) >= bridgeStderrLineMax {
			truncated = true
		}
	}

	buf := make([]byte, 32*1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			for len(chunk) > 0 {
				idx := bytes.IndexAny(chunk, "\r\n")
				if idx < 0 {
					appendPart(chunk)
					break
				}
				appendPart(chunk[:idx])
				flush()
				chunk = chunk[idx+1:]
			}
		}
		if err != nil {
			break
		}
	}
	flush()
	// 即使读取出错也继续排空，保证子进程不会因为 stderr 管道写满而无法退出。
	_, _ = io.Copy(io.Discard, r)
}

func (b *Bridge) readLoop(cmd *exec.Cmd, stdout io.Reader, done chan struct{}) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), bridgeMaxLineBytes)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		b.handleLine(line)
	}
	if err := scanner.Err(); err != nil {
		// 超长行/读错误会让 Scanner 停下，先杀掉子进程避免 Wait 永久阻塞。
		b.stderr.Add("[读取桥输出失败] " + err.Error())
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	b.handleExit(cmd, done, waitErr)
}

// handleLine 解析一行协议帧：事件帧转给回调，响应帧按 id 唤醒等待者。
func (b *Bridge) handleLine(line []byte) {
	var frame bridgeFrame
	if err := json.Unmarshal(line, &frame); err != nil {
		b.stderr.Add("[协议行不是合法 JSON] " + truncateLine(string(line), 500))
		return
	}
	// 收到任何合法协议帧即说明桥已可用（契约 §1：事件可随时到达）。
	b.markReady()

	if frame.Event != "" {
		b.mu.Lock()
		switch frame.Event {
		case "run.started":
			b.runActive = true
		case "run.finished":
			b.runActive = false
		}
		handler := b.onEvent
		b.mu.Unlock()
		if handler != nil {
			handler(frame.Event, frame.Data)
		}
		return
	}
	if frame.ID == "" {
		b.stderr.Add("[缺少 id 的帧] " + truncateLine(string(line), 500))
		return
	}

	b.mu.Lock()
	ch, ok := b.pending[frame.ID]
	if ok {
		delete(b.pending, frame.ID)
	}
	b.mu.Unlock()
	if !ok {
		b.stderr.Add("[无匹配请求的响应 id=" + frame.ID + "]")
		return
	}

	reply := bridgeReply{result: frame.Result}
	if frame.OK == nil || !*frame.OK {
		if frame.Error != nil {
			reply.err = frame.Error
		} else {
			reply.err = &bridgeError{Code: "internal", Message: "Python 桥返回了失败的响应"}
		}
	}
	if reply.err == nil {
		b.mu.Lock()
		b.timeouts = 0
		b.mu.Unlock()
	}
	ch <- reply // 带缓冲 1，不会阻塞
}

// markReady 在收到第一帧后把状态切到 ready，并重置退避计数。
func (b *Bridge) markReady() {
	b.mu.Lock()
	if b.ready {
		b.mu.Unlock()
		return
	}
	b.ready = true
	b.restarts = 0
	changed := b.state != bridgeStateReady
	b.state = bridgeStateReady
	b.stateMessage = ""
	handler := b.onState
	b.mu.Unlock()
	if changed && handler != nil {
		handler(bridgeStateReady, "")
	}
}

// handleExit 处理子进程退出：拒绝所有 pending 请求，并决定是否重启。
func (b *Bridge) handleExit(cmd *exec.Cmd, done chan struct{}, waitErr error) {
	close(done)

	b.mu.Lock()
	if b.cmd != cmd {
		// 旧进程的退出回调（例如 Close 后又重启过），忽略。
		b.mu.Unlock()
		return
	}
	stdin := b.stdin
	b.cmd = nil
	b.stdin = nil
	b.ready = false
	b.runActive = false // 进程已死，运行中的任务由 App 侧补发终态事件
	stopped := b.stopped || b.closing
	pending := b.pending
	b.pending = make(map[string]chan bridgeReply)
	b.mu.Unlock()

	if stdin != nil {
		_ = stdin.Close()
	}
	reason := "进程已退出"
	if waitErr != nil {
		reason = waitErr.Error()
	}
	for _, ch := range pending {
		ch <- bridgeReply{err: &bridgeError{
			Code:    "not_ready",
			Message: "Python 桥进程已退出（" + reason + "），请求未完成，正在尝试自动恢复。",
		}}
	}

	if stopped {
		b.setState(bridgeStateStopped, "")
		return
	}
	b.scheduleRestart()
}

// scheduleRestart 指数退避后重启（最多 3 次）；超过则置 failed。
func (b *Bridge) scheduleRestart() {
	b.mu.Lock()
	if b.stopped || b.closing {
		b.mu.Unlock()
		return
	}
	attempts := b.restarts
	if attempts >= bridgeMaxRestarts {
		b.mu.Unlock()
		msg := "Python 桥多次异常退出，自动重启失败。请检查引擎目录与 Python 环境，修复后在设置中重试。"
		tail := b.stderr.Tail(5)
		if len(tail) > 0 {
			msg += "（最近错误：" + truncateLine(tail[len(tail)-1], 160) + "）"
		}
		b.setState(bridgeStateFailed, msg)
		return
	}
	delay := b.restartDelay * time.Duration(1<<attempts)
	if delay <= 0 {
		delay = 100 * time.Millisecond
	}
	b.restarts = attempts + 1
	b.mu.Unlock()

	b.setState(bridgeStateRestarting, fmt.Sprintf(
		"Python 桥异常退出，%d 毫秒后自动重启（第 %d/%d 次）…",
		delay.Milliseconds(), attempts+1, bridgeMaxRestarts,
	))

	go func() {
		time.Sleep(delay)
		b.mu.Lock()
		stopped := b.stopped
		b.mu.Unlock()
		if stopped {
			return
		}
		if err := b.startProcess(); err != nil {
			b.setState(bridgeStateRestarting, "重启 Python 桥失败，继续尝试…")
			b.scheduleRestart()
		}
	}()
}

// Close 停止桥进程并拒绝所有未完成请求（应用退出时调用）。
// Close 优雅停止桥进程：运行中先 best-effort 发 run.stop（≤1.5s），
// 再关 stdin 让桥自行收尾（≤2.5s），最后才 Kill，避免硬杀丢失 interrupt 收尾。
func (b *Bridge) Close() {
	b.mu.Lock()
	if b.stopped || b.closing {
		b.mu.Unlock()
		return
	}
	b.closing = true
	runActive := b.runActive
	running := b.cmd != nil
	b.mu.Unlock()

	// 1) 运行中先请求停止，给引擎 interrupt 一个收尾窗口。
	if runActive && running {
		_, _ = b.callInternal("run.stop", nil, bridgeGracefulStopTimeout, true)
	}

	// 2) 标记关闭、拒绝未完成请求并关闭 stdin（桥收 stdin EOF 后会自行 shutdown）。
	b.mu.Lock()
	b.stopped = true
	b.runActive = false
	stdin := b.stdin
	done := b.procDone
	pending := b.pending
	b.pending = make(map[string]chan bridgeReply)
	b.mu.Unlock()

	for _, ch := range pending {
		ch <- bridgeReply{err: &bridgeError{Code: "not_ready", Message: "应用正在退出，Python 桥已关闭。"}}
	}
	if stdin != nil {
		_ = stdin.Close()
	}

	// 3) 等待进程自行退出，超时才强制 Kill。
	if done != nil {
		select {
		case <-done:
		case <-time.After(bridgeCloseDrainTimeout):
			b.mu.Lock()
			cmd := b.cmd
			b.mu.Unlock()
			if cmd != nil && cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			select {
			case <-done:
			case <-time.After(bridgeKillWaitTimeout):
			}
		}
	}
	b.setState(bridgeStateStopped, "")
}

// Call 发送命令并等待响应，返回 result 原始 JSON。
// timeout<=0 时用该命令的默认超时（契约 §1）。
func (b *Bridge) Call(cmd string, params any, timeout time.Duration) (json.RawMessage, error) {
	return b.callInternal(cmd, params, timeout, false)
}

// callInternal 是 Call 的实现；allowClosing 仅供 Close 的 run.stop 在关闭阶段使用。
func (b *Bridge) callInternal(cmd string, params any, timeout time.Duration, allowClosing bool) (json.RawMessage, error) {
	if timeout <= 0 {
		timeout = commandTimeout(cmd)
	}
	if err := b.ensureRunning(allowClosing); err != nil {
		return nil, err
	}
	// 并发调用 Start 时可能进程还没就绪，等一小段时间拿到 stdin。
	stdin, _, err := b.waitStdin(10*time.Second, allowClosing)
	if err != nil {
		return nil, err
	}

	b.mu.Lock()
	b.nextID++
	id := fmt.Sprintf("%d", b.nextID)
	ch := make(chan bridgeReply, 1)
	b.pending[id] = ch
	b.mu.Unlock()

	paramsRaw := json.RawMessage("{}")
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			b.removePending(id)
			return nil, fmt.Errorf("序列化请求参数失败：%v", err)
		}
		paramsRaw = raw
	}
	payload, err := json.Marshal(struct {
		ID     string          `json:"id"`
		Cmd    string          `json:"cmd"`
		Params json.RawMessage `json:"params"`
	}{ID: id, Cmd: cmd, Params: paramsRaw})
	if err != nil {
		b.removePending(id)
		return nil, fmt.Errorf("序列化请求失败：%v", err)
	}
	payload = append(payload, '\n')

	b.writeMu.Lock()
	_, werr := stdin.Write(payload)
	b.writeMu.Unlock()
	if werr != nil {
		b.removePending(id)
		return nil, fmt.Errorf("向 Python 桥发送请求失败：%v", werr)
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case reply := <-ch:
		if reply.err != nil {
			return nil, reply.err
		}
		return reply.result, nil
	case <-timer.C:
		b.removePending(id)
		b.noteTimeout(cmd)
		return nil, fmt.Errorf("请求超时（%s，%s）：Python 桥没有在预期时间内响应，请稍后重试。", cmd, timeout)
	}
}

// noteTimeout 记录连续超时；连续 2 次说明桥可能假死，杀掉进程触发自动重启。
func (b *Bridge) noteTimeout(cmd string) {
	b.mu.Lock()
	b.timeouts++
	count := b.timeouts
	proc := b.cmd
	b.mu.Unlock()
	if count < bridgeMaxConsecutiveTimeouts || proc == nil || proc.Process == nil {
		return
	}
	b.stderr.Add(fmt.Sprintf("[watchdog] 连续 %d 次命令超时（最后一次：%s），主动重启 Python 桥", count, cmd))
	b.mu.Lock()
	b.timeouts = 0
	proc = b.cmd
	b.mu.Unlock()
	if proc != nil && proc.Process != nil {
		_ = proc.Process.Kill() // 读循环 EOF 后会走 handleExit -> scheduleRestart
	}
}

func (b *Bridge) removePending(id string) {
	b.mu.Lock()
	delete(b.pending, id)
	b.mu.Unlock()
}

// ensureRunning 保证桥进程已启动；启动失败返回中文错误。
// allowClosing=true 时允许在应用退出阶段只使用现有进程（不重启）。
func (b *Bridge) ensureRunning(allowClosing bool) error {
	b.mu.Lock()
	stopped := b.stopped
	closing := b.closing && !allowClosing
	running := b.cmd != nil
	starting := b.starting
	state := b.state
	b.mu.Unlock()

	if stopped || closing {
		return errors.New("Python 桥已关闭，请重新启动应用。")
	}
	// 运行中/启动中/等待自动重启时都不重复拉起进程，由 waitStdin 等待就绪。
	if running || starting || state == bridgeStateRestarting {
		return nil
	}
	return b.Start()
}

// waitStdin 等待桥进程就绪（stdin 可用）或失败/超时。
func (b *Bridge) waitStdin(timeout time.Duration, allowClosing bool) (io.WriteCloser, *exec.Cmd, error) {
	deadline := time.Now().Add(timeout)
	for {
		b.mu.Lock()
		stdin, cmd := b.stdin, b.cmd
		stopped, closing, state := b.stopped, b.closing, b.state
		b.mu.Unlock()
		if stopped || (closing && !allowClosing) {
			return nil, nil, errors.New("Python 桥已关闭，请重新启动应用。")
		}
		if stdin != nil && cmd != nil {
			return stdin, cmd, nil
		}
		if closing {
			return nil, nil, errors.New("应用正在退出，Python 桥已关闭。")
		}
		if time.Now().After(deadline) {
			if state == bridgeStateFailed {
				return nil, nil, errors.New("Python 桥启动失败，请检查引擎目录与 Python 环境后重试。")
			}
			return nil, nil, errors.New("Python 桥启动超时，请稍后重试。")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// request 是带结果解码的通用调用。
func (b *Bridge) request(cmd string, params any, out any) error {
	raw, err := b.Call(cmd, params, 0)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if len(raw) == 0 {
		raw = json.RawMessage("null")
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("解析 Python 桥返回失败（%s）：%v", cmd, err)
	}
	return nil
}

// ---- 契约 §3 的强类型封装 ----

// Ping 调用 ping（10s 超时）。
func (b *Bridge) Ping() (PingInfo, error) {
	var out PingInfo
	err := b.request("ping", nil, &out)
	return out, err
}

// Status 调用 status。
func (b *Bridge) Status() (Status, error) {
	var out Status
	err := b.request("status", nil, &out)
	if out.Accounts == nil {
		out.Accounts = []Account{}
	}
	return out, err
}

// ListAccounts 调用 accounts.list。
func (b *Bridge) ListAccounts() ([]Account, error) {
	var wrapper struct {
		Accounts []Account `json:"accounts"`
	}
	if err := b.request("accounts.list", nil, &wrapper); err != nil {
		return nil, err
	}
	if wrapper.Accounts == nil {
		wrapper.Accounts = []Account{}
	}
	return wrapper.Accounts, nil
}

// AddAccount 调用 accounts.add。
func (b *Bridge) AddAccount(username, password, name string) (Account, error) {
	var wrapper struct {
		Account Account `json:"account"`
	}
	err := b.request("accounts.add", map[string]string{
		"username": username,
		"password": password,
		"name":     name,
	}, &wrapper)
	return wrapper.Account, err
}

// RemoveAccount 调用 accounts.remove。
func (b *Bridge) RemoveAccount(id string) error {
	return b.request("accounts.remove", map[string]string{"id": id}, nil)
}

// GetSettings 调用 settings.get。
func (b *Bridge) GetSettings() (Settings, error) {
	var wrapper struct {
		Settings Settings `json:"settings"`
	}
	err := b.request("settings.get", nil, &wrapper)
	return wrapper.Settings, err
}

// SaveSettings 调用 settings.set（只接受白名单字段的部分设置）。
func (b *Bridge) SaveSettings(settings map[string]any) (Settings, error) {
	var wrapper struct {
		Settings Settings `json:"settings"`
	}
	err := b.request("settings.set", map[string]any{"settings": settings}, &wrapper)
	return wrapper.Settings, err
}

// ListCourses 调用 courses.list（网络请求，120s 超时）。
func (b *Bridge) ListCourses(accountID string) (CourseListResult, error) {
	var out CourseListResult
	err := b.request("courses.list", map[string]string{"account_id": accountID}, &out)
	if out.Courses == nil {
		out.Courses = []Course{}
	}
	return out, err
}

// StartRun 调用 run.start。
func (b *Bridge) StartRun(opts RunOptions) (RunStartResult, error) {
	var out RunStartResult
	err := b.request("run.start", opts, &out)
	return out, err
}

// StopRun 调用 run.stop。
func (b *Bridge) StopRun() error {
	return b.request("run.stop", nil, nil)
}

// GetRunState 调用 run.status。
func (b *Bridge) GetRunState() (RunState, error) {
	var out RunState
	err := b.request("run.status", nil, &out)
	if out.CourseIDs == nil {
		out.CourseIDs = []string{}
	}
	return out, err
}

// ListReviews 调用 reviews.list；days=0 表示全部（limit 放大到 500，其余 50）。
func (b *Bridge) ListReviews(days int) (ReviewListResult, error) {
	var out ReviewListResult
	limit := 50
	if days == 0 {
		limit = 500
	}
	params := map[string]int{"days": days, "limit": limit}
	err := b.request("reviews.list", params, &out)
	if out.Items == nil {
		out.Items = []ReviewSummary{}
	}
	return out, err
}

// GetReview 调用 reviews.get。
func (b *Bridge) GetReview(submissionID string) (ReviewItem, error) {
	var wrapper struct {
		Item ReviewItem `json:"item"`
	}
	err := b.request("reviews.get", map[string]string{"submission_id": submissionID}, &wrapper)
	return wrapper.Item, err
}

// truncateLine 截断诊断输出，避免把超长内容塞进缓冲区。
func truncateLine(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
