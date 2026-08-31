package mihomo

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type ProcessManager struct {
	mu          sync.Mutex
	lifecycleMu sync.Mutex
	cmd         *exec.Cmd
	pid         int
	startTime   time.Time
	binaryPath  string
	configPath  string
	done        chan struct{}
	logs        []string
	desired     bool
	external    bool
	// crashHandler is invoked from waitProcess when the mihomo process exits
	// with a non-zero status AND desired == true (i.e. the exit was not
	// initiated by an admin-issued Stop). Wired in app/container.go to forward
	// to telegram.NotifyCrash. May be nil — callers must guard.
	crashHandler func(exitErr error)
}

var globalPM *ProcessManager
var pmOnce sync.Once

func GetProcessManager(binary, config string) *ProcessManager {
	pmOnce.Do(func() {
		globalPM = &ProcessManager{
			binaryPath: binary,
			configPath: config,
			logs:       make([]string, 0, 200),
		}
	})

	return globalPM
}

// NewProcessManager creates an independent process manager instance.
func NewProcessManager(binary, config string) *ProcessManager {
	return &ProcessManager{
		binaryPath: binary,
		configPath: config,
		logs:       make([]string, 0, 200),
	}
}

// SetCrashHandler registers a callback invoked when the managed mihomo process
// exits unexpectedly (non-zero exit code while desired == true). The handler
// is called from the waitProcess goroutine, so callers must not block on
// resources held by that goroutine (e.g. pm.mu). Pass nil to disable.
func (pm *ProcessManager) SetCrashHandler(fn func(exitErr error)) {
	pm.mu.Lock()
	pm.crashHandler = fn
	pm.mu.Unlock()
}

// productionAllowedBinaryPrefixes is the hard-coded allowlist used in
// production. It prevents an attacker from tricking 3m-ui into executing an
// arbitrary file by manipulating the configured binary path.
var productionAllowedBinaryPrefixes = []string{
	"/usr/local/bin/",
	"/usr/bin/",
	"/opt/",
}

// extraAllowedBinaryPrefixes is consulted by isAllowedBinaryPath in addition
// to the production list. Production code never sets this; unit tests may
// temporarily allow paths under t.TempDir() via AllowBinaryPathPrefixForTesting.
var extraAllowedBinaryPrefixes []string

// AllowBinaryPathPrefixForTesting appends a directory prefix that
// isAllowedBinaryPath will accept. Intended only for unit tests that need a
// fake mihomo binary under t.TempDir(). Callers should restore the previous
// state with the returned function (typically via t.Cleanup).
func AllowBinaryPathPrefixForTesting(prefix string) (restore func()) {
	prev := append([]string(nil), extraAllowedBinaryPrefixes...)
	clean := filepath.Clean(prefix)
	if clean != "" && !strings.HasSuffix(clean, string(filepath.Separator)) {
		clean += string(filepath.Separator)
	}
	extraAllowedBinaryPrefixes = append(extraAllowedBinaryPrefixes, clean)
	return func() {
		extraAllowedBinaryPrefixes = prev
	}
}

// isAllowedBinaryPath restricts where the Mihomo binary can reside.
func isAllowedBinaryPath(path string) bool {
	clean := filepath.Clean(path)
	for _, prefix := range productionAllowedBinaryPrefixes {
		if strings.HasPrefix(clean, prefix) {
			return true
		}
	}
	for _, prefix := range extraAllowedBinaryPrefixes {
		if prefix != "" && strings.HasPrefix(clean, prefix) {
			return true
		}
	}
	return false
}

func (pm *ProcessManager) GetVersion() (*VersionInfo, error) {
	info, err := os.Stat(pm.binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("mihomo binary not found: %s", pm.binaryPath)
		}

		return nil, fmt.Errorf("failed to stat mihomo binary: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("mihomo binary path is a directory: %s", pm.binaryPath)
	}
	if !isAllowedBinaryPath(pm.binaryPath) {
		return nil, fmt.Errorf("mihomo binary path is not in allowed list: %s", pm.binaryPath)
	}

	cmd := exec.Command(pm.binaryPath, "-v")

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run mihomo -v: %w", err)
	}

	output := strings.TrimSpace(out.String())
	parts := strings.Fields(output)

	version := "unknown"
	// Mihomo output has changed over time (for example, it may contain
	// both the product name and the version). Prefer the first token that
	// looks like a semantic version instead of assuming it is fields[1].
	for _, part := range parts {
		if strings.HasPrefix(part, "v") && len(part) > 1 && strings.ContainsAny(part[1:], "0123456789") {
			version = part
			break
		}
	}

	return &VersionInfo{
		Version: version,
		Commit:  "official-build",
		Build:   output,
	}, nil
}

func (pm *ProcessManager) ValidateConfig() error {
	pm.mu.Lock()
	binaryPath := pm.binaryPath
	configPath := pm.configPath
	pm.mu.Unlock()

	if binaryPath == "" || configPath == "" {
		return fmt.Errorf("mihomo binary or config path is empty")
	}

	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("mihomo binary unavailable: %w", err)
	}

	if info.IsDir() || info.Mode()&0111 == 0 {
		return fmt.Errorf("mihomo binary is not executable: %s", binaryPath)
	}

	if !isAllowedBinaryPath(binaryPath) {
		return fmt.Errorf("mihomo binary path is not in allowed list: %s", binaryPath)
	}

	cfgInfo, err := os.Stat(configPath)
	if err != nil {
		return fmt.Errorf("mihomo config not found: %s", configPath)
	}

	if cfgInfo.IsDir() || cfgInfo.Size() == 0 {
		return fmt.Errorf("mihomo config is empty: %s", configPath)
	}

	cmd := exec.Command(
		binaryPath,
		"-t",
		"-d",
		filepath.Dir(configPath),
		"-f",
		configPath,
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(out.String())

		if detail != "" {
			return fmt.Errorf(
				"mihomo configuration validation failed: %s",
				detail,
			)
		}

		return fmt.Errorf(
			"mihomo configuration validation failed: %w",
			err,
		)
	}

	return nil
}

// findExistingProcesses 查找已经运行的、使用相同 Mihomo binary + config 的进程。
func (pm *ProcessManager) findExistingProcesses() []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	binary, err := filepath.EvalSymlinks(pm.binaryPath)
	if err != nil {
		return nil
	}

	config := filepath.Clean(pm.configPath)

	configDir := filepath.Dir(config)

	var pids []int

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 || pid == os.Getpid() {
			continue
		}

		base := filepath.Join("/proc", entry.Name())

		cmdline, err := os.ReadFile(
			filepath.Join(base, "cmdline"),
		)
		if err != nil {
			continue
		}

		args := strings.Split(
			string(cmdline),
			"\x00",
		)

		if len(args) < 2 {
			continue
		}

		exe, err := filepath.EvalSymlinks(
			filepath.Join(base, "exe"),
		)
		if err != nil {
			continue
		}

		if exe != binary {
			continue
		}

		matchedDir := false
		matchedConfig := false

		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "-d":
				if i+1 < len(args) {
					matchedDir =
						filepath.Clean(args[i+1]) == configDir
				}

			case "-f":
				if i+1 < len(args) {
					matchedConfig =
						filepath.Clean(args[i+1]) == config
				}
			}
		}

		if matchedDir && matchedConfig {
			pids = append(pids, pid)
		}
	}

	return pids
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	return process.Signal(syscall.Signal(0)) == nil
}

// adoptExistingLocked 接管已经存在的 Mihomo。
func (pm *ProcessManager) adoptExistingLocked(
	pids []int,
) int {
	if len(pids) == 0 {
		return 0
	}

	keep := pids[0]

	for _, pid := range pids[1:] {
		if pid < keep {
			keep = pid
		}
	}

	for _, pid := range pids {
		if pid == keep {
			continue
		}

		process, err := os.FindProcess(pid)
		if err == nil {
			_ = process.Signal(syscall.SIGTERM)
		}

		pm.appendLogLocked(
			fmt.Sprintf(
				"stopped duplicate Mihomo process PID %d; keeping PID %d",
				pid,
				keep,
			),
		)
	}

	pm.pid = keep
	pm.startTime = time.Now()
	pm.desired = true
	pm.external = true

	return keep
}

func (pm *ProcessManager) Start() error {
	pm.lifecycleMu.Lock()
	defer pm.lifecycleMu.Unlock()
	return pm.start()
}

func (pm *ProcessManager) start() error {
	pm.mu.Lock()

	if pm.isRunning() {
		pid := pm.pid

		pm.mu.Unlock()

		return fmt.Errorf(
			"mihomo is already running (PID: %d)",
			pid,
		)
	}

	binaryPath := pm.binaryPath
	configPath := pm.configPath

	pm.mu.Unlock()

	info, err := os.Stat(binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"mihomo binary not found: %s",
				binaryPath,
			)
		}

		return fmt.Errorf(
			"failed to stat mihomo binary: %w",
			err,
		)
	}

	if info.IsDir() || info.Mode()&0111 == 0 {
		return fmt.Errorf(
			"mihomo binary is not executable: %s",
			binaryPath,
		)
	}

	if !isAllowedBinaryPath(binaryPath) {
		return fmt.Errorf(
			"mihomo binary path is not in allowed list: %s",
			binaryPath,
		)
	}

	cfgInfo, err := os.Stat(configPath)
	if err != nil {
		return fmt.Errorf(
			"mihomo config not found: %s",
			configPath,
		)
	}

	if cfgInfo.IsDir() || cfgInfo.Size() == 0 {
		return fmt.Errorf(
			"mihomo config is empty: %s",
			configPath,
		)
	}

	// findExistingProcesses scans /proc and is slow I/O. Do NOT hold pm.mu
	// during the scan — it blocks Logs()/Status()/Stop() and any other caller
	// that needs the lock. The result depends only on binaryPath/configPath,
	// which are immutable after construction, so it can be computed before the
	// lock is acquired. Only the subsequent state mutation needs the lock.
	existing := pm.findExistingProcesses()

	pm.mu.Lock()

	if pm.isRunning() {
		pid := pm.pid

		pm.mu.Unlock()

		return fmt.Errorf(
			"mihomo is already running (PID: %d)",
			pid,
		)
	}

	if len(existing) > 0 {
		pid := pm.adoptExistingLocked(existing)

		pm.appendLogLocked(
			fmt.Sprintf(
				"adopted existing Mihomo process PID %d",
				pid,
			),
		)

		pm.mu.Unlock()

		return nil
	}

	pm.mu.Unlock()

	cmd := exec.Command(
		binaryPath,
		"-d",
		filepath.Dir(configPath),
		"-f",
		configPath,
	)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	writer := &processLogWriter{
		pm: pm,
	}

	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		return fmt.Errorf(
			"failed to start mihomo: %w",
			err,
		)
	}

	pm.mu.Lock()

	pm.cmd = cmd
	pm.pid = cmd.Process.Pid
	pm.startTime = time.Now()
	pm.done = make(chan struct{})
	pm.desired = true
	pm.external = false

	done := pm.done

	pm.mu.Unlock()

	go pm.waitProcess(cmd, done)

	select {
	case <-done:
		pm.mu.Lock()

		if pm.cmd == cmd {
			pm.cmd = nil
			pm.pid = 0
			pm.startTime = time.Time{}
			pm.done = nil
			pm.desired = false
		}

		pm.mu.Unlock()

		return fmt.Errorf(
			"mihomo exited immediately; check logs",
		)

	case <-time.After(500 * time.Millisecond):
		return nil
	}
}

func (pm *ProcessManager) waitProcess(
	cmd *exec.Cmd,
	done chan struct{},
) {
	err := cmd.Wait()

	close(done)

	pm.mu.Lock()

	if err != nil {
		pm.appendLogLocked(
			fmt.Sprintf(
				"process exited: %v",
				err,
			),
		)
	} else {
		pm.appendLogLocked(
			"process exited",
		)
	}

	restart :=
		pm.desired &&
			pm.cmd == cmd

	// Crash notification: only fire on a non-zero exit while desired == true
	// (intentional Stop sets desired=false first). Captured inside the lock
	// so the handler invocation is consistent with the exit verdict and does
	// not race a concurrent Stop. Invoked outside the lock to avoid
	// deadlocks if the handler calls back into pm.
	crashed := err != nil && pm.desired
	crashHandler := pm.crashHandler

	pm.mu.Unlock()

	if crashed && crashHandler != nil {
		crashHandler(err)
	}

	if !restart {
		return
	}

	time.Sleep(2 * time.Second)

	if pm.IsRunning() {
		return
	}

	if err := pm.ValidateConfig(); err != nil {
		pm.mu.Lock()

		pm.appendLogLocked(
			fmt.Sprintf(
				"automatic restart blocked by config validation: %v",
				err,
			),
		)

		pm.mu.Unlock()

		return
	}

	if err := pm.Start(); err != nil {
		pm.mu.Lock()

		pm.appendLogLocked(
			fmt.Sprintf(
				"automatic restart failed: %v",
				err,
			),
		)

		pm.mu.Unlock()
	}
}

func (pm *ProcessManager) Stop() error {
	pm.lifecycleMu.Lock()
	defer pm.lifecycleMu.Unlock()
	return pm.stop()
}

func (pm *ProcessManager) stop() error {
	pm.mu.Lock()

	if !pm.isRunning() {
		pm.desired = false

		pm.mu.Unlock()

		return fmt.Errorf(
			"mihomo is not running",
		)
	}

	pid := pm.pid
	cmd := pm.cmd
	done := pm.done

	pm.desired = false

	pm.mu.Unlock()

	if pgid, err := syscall.Getpgid(pid); err == nil &&
		pgid > 0 {

		_ = syscall.Kill(
			-pgid,
			syscall.SIGTERM,
		)

	} else if process, err := os.FindProcess(pid); err == nil {
		_ = process.Signal(syscall.SIGTERM)
	}

	if done != nil {
		select {
		case <-done:

		case <-time.After(5 * time.Second):

			if pgid, err := syscall.Getpgid(pid); err == nil &&
				pgid > 0 {

				_ = syscall.Kill(
					-pgid,
					syscall.SIGKILL,
				)
			}

			if cmd != nil &&
				cmd.Process != nil {

				_ = cmd.Process.Kill()
			}

			<-done
		}

	} else {
		deadline :=
			time.Now().Add(5 * time.Second)

		for processAlive(pid) &&
			time.Now().Before(deadline) {

			time.Sleep(
				100 * time.Millisecond,
			)
		}

		if processAlive(pid) {
			if process, err := os.FindProcess(pid); err == nil {
				_ = process.Kill()
			}
		}
	}

	pm.mu.Lock()

	if pm.pid == pid {
		pm.cmd = nil
		pm.pid = 0
		pm.startTime = time.Time{}
		pm.done = nil
		pm.external = false
	}

	pm.mu.Unlock()

	return nil
}

func (pm *ProcessManager) Restart() error {
	pm.lifecycleMu.Lock()
	defer pm.lifecycleMu.Unlock()

	if pm.IsRunning() {
		if err := pm.stop(); err != nil {
			return fmt.Errorf(
				"failed to stop before restart: %w",
				err,
			)
		}
	}

	if err := pm.ValidateConfig(); err != nil {
		return err
	}

	return pm.start()
}

func (pm *ProcessManager) IsRunning() bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return pm.isRunning()
}

func (pm *ProcessManager) isRunning() bool {
	if pm.pid == 0 {
		return false
	}

	if pm.done != nil {
		select {
		case <-pm.done:
			return false

		default:
		}
	}

	return processAlive(pm.pid)
}

func (pm *ProcessManager) Status() (*StatusResponse, error) {
	pm.mu.Lock()

	running := pm.isRunning()
	pid := pm.pid
	startTime := pm.startTime

	pm.mu.Unlock()

	versionStr := "unknown"

	if vInfo, err := pm.GetVersion(); err == nil &&
		vInfo != nil {

		versionStr = vInfo.Version
	}

	uptime := "0s"

	if running && !startTime.IsZero() {
		uptime =
			formatDuration(
				time.Since(startTime),
			)
	}

	return &StatusResponse{
		Running: running,
		Version: versionStr,
		PID:     pid,
		Uptime:  uptime,
	}, nil
}

func (pm *ProcessManager) Logs() []string {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	out := make(
		[]string,
		len(pm.logs),
	)

	copy(
		out,
		pm.logs,
	)

	return out
}

func (pm *ProcessManager) appendLogLocked(line string) {
	line = strings.TrimSpace(line)

	if line == "" {
		return
	}

	pm.logs = append(
		pm.logs,
		time.Now().Format(time.RFC3339)+" "+line,
	)

	if len(pm.logs) > 200 {
		pm.logs =
			pm.logs[len(pm.logs)-200:]
	}
}

type processLogWriter struct {
	pm *ProcessManager
}

func (w *processLogWriter) Write(
	p []byte,
) (int, error) {

	lines :=
		strings.Split(
			string(p),
			"\n",
		)

	w.pm.mu.Lock()
	defer w.pm.mu.Unlock()

	for _, line := range lines {
		w.pm.appendLogLocked(line)
	}

	return len(p), nil
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	h := d / time.Hour
	d -= h * time.Hour

	m := d / time.Minute
	d -= m * time.Minute

	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}

	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}

	return fmt.Sprintf("%ds", s)
}
