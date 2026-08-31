package telegram

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
	"github.com/kazeyukiro/3m-ui/backend/internal/system"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

// Scheduler runs periodic Telegram reports based on Settings.Schedule and
// exposes NotifyEvent for the events agent (tg-5) to fire one-shot alerts
// gated by Settings.EventEnabled.
//
// Schedule specs supported (intentionally minimal — no external cron dep):
//   - @hourly              → every hour at :00
//   - @daily, @midnight    → every day at 00:00
//   - @weekly              → every Sunday at 00:00
//   - @monthly             → 1st of every month at 00:00
//   - @every <duration>    → tick at interval (e.g. @every 6h, @every 30m)
//   - 5-field cron         → minute hour day month weekday
//     (supports *, comma lists, exact values, "A-B" ranges,
//     "*/N" steps, and "A-B/N" stepped ranges)
type Scheduler struct {
	db        *gorm.DB
	mihomo    *mihomo.Service
	userSvc   *user.Service
	systemSvc *system.Service
	dbPath    string
	mihomoCfg string

	mu     sync.Mutex
	stopCh chan struct{}
	wg     sync.WaitGroup

	// startOnce guards Start() so the scheduler goroutine is launched at most
	// once even if Start() is called repeatedly (defensive against caller bugs).
	startOnce sync.Once
}

// NewScheduler constructs a Scheduler. dbPath/mihomoCfg enable optional backup
// attachment; pass empty strings to skip backup.
func NewScheduler(db *gorm.DB, mihomoSvc *mihomo.Service, userSvc *user.Service, systemSvc *system.Service, dbPath, mihomoCfg string) *Scheduler {
	return &Scheduler{
		db:        db,
		mihomo:    mihomoSvc,
		userSvc:   userSvc,
		systemSvc: systemSvc,
		dbPath:    dbPath,
		mihomoCfg: mihomoCfg,
		stopCh:    make(chan struct{}),
	}
}

// Start launches the scheduler goroutine. Idempotent: safe to call multiple
// times — only the first call spawns the loop, subsequent calls are no-ops.
func (s *Scheduler) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.loop()
		}()
		log.Printf("telegram: scheduler started")
	})
}

// Stop signals the scheduler loop to exit and blocks until it has.
func (s *Scheduler) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
	s.mu.Unlock()
	s.wg.Wait()
}

// loop ticks once per minute and matches the current time against the schedule.
// `lastFired` prevents firing twice in the same minute (e.g. if a tick lands
// at 12:00:01 and the next at 12:00:59).
func (s *Scheduler) loop() {
	// Align to the next minute boundary so @daily/@hourly fire close to :00.
	now := time.Now()
	wait := time.Minute - time.Duration(now.Second())*time.Second - time.Duration(now.Nanosecond())
	select {
	case <-s.stopCh:
		return
	case <-time.After(wait):
	}
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	var lastFired time.Time
	for {
		select {
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			settings, err := LoadSettings(s.db)
			if err != nil {
				log.Printf("telegram: scheduler load settings: %v", err)
				continue
			}
			if !settings.Enabled || len(settings.ChatIDs) == 0 || strings.TrimSpace(settings.BotToken) == "" {
				continue
			}
			spec := strings.TrimSpace(settings.Schedule)
			if spec == "" {
				spec = "@daily"
			}
			due, next := scheduleMatch(spec, now, lastFired)
			if !due {
				continue
			}
			lastFired = next
			s.fireReport(settings)
		}
	}
}

// fireReport builds and sends the periodic report message to every admin
// chat in settings.ChatIDs. If settings.AttachBackup is set, also zips the
// SQLite database + Mihomo config in memory and attaches the file.
func (s *Scheduler) fireReport(settings Settings) {
	client := NewClient(settings.BotToken, settings.ChatIDs, settings.ProxyURL, settings.APIServer)
	if !client.Enabled() {
		return
	}
	msg := s.buildReportMessage(settings)
	for _, chatID := range client.ChatIDs {
		if err := client.sendOne(chatID, msg); err != nil {
			log.Printf("telegram: scheduler send to %s: %v", chatID, err)
		}
	}
	if settings.AttachBackup {
		s.sendBackup(client)
	}
}

// buildReportMessage assembles the periodic server summary:
//   - host, version, uptime (from mihomo.GetStatus)
//   - CPU / memory / disk (from system.Service.GetStatus)
//   - user count, online count, blocked count
//   - total traffic sum
//   - depleted / expiring user counts (uses settings.ExpiryWarnDays)
func (s *Scheduler) buildReportMessage(settings Settings) string {
	var b strings.Builder
	host, _ := os.Hostname()
	now := time.Now()
	b.WriteString("📋 <b>3m-ui 定期报告 / Scheduled Report</b>\n")
	b.WriteString(fmt.Sprintf("主机 / Host: <code>%s</code>\n", escapeHTML(host)))
	b.WriteString(fmt.Sprintf("时间 / Time: <code>%s</code>\n", now.Format("2006-01-02 15:04:05")))

	if s.mihomo != nil {
		if st, err := s.mihomo.GetStatus(); err == nil && st != nil {
			core := "stopped"
			if st.Running {
				core = "running"
			}
			b.WriteString(fmt.Sprintf("核心 / Core: <code>%s</code> v%s (uptime %s)\n",
				core, escapeHTML(st.Version), escapeHTML(st.Uptime)))
		}
	}

	if s.systemSvc != nil {
		if stats := s.systemSvc.GetStatus(); stats != nil {
			b.WriteString(fmt.Sprintf("CPU: <code>%.1f%%</code>  Mem: <code>%s / %s</code>  Disk: <code>%.1f%%</code>\n",
				stats.CPU.Percent,
				formatBytes(int64(stats.Memory.Used)), formatBytes(int64(stats.Memory.Total)),
				stats.Disk.Percent))
		}
	}

	var users []models.ProxyUser
	var online, blocked, depleted, expiring int64
	var totalTraffic int64
	warnDays := settings.ExpiryWarnDays
	if warnDays <= 0 {
		warnDays = 3
	}
	warnWindow := time.Duration(warnDays) * 24 * time.Hour
	_ = s.db.Find(&users).Error
	for _, u := range users {
		totalTraffic += u.TrafficUsed
		if u.Online {
			online++
		}
		if !user.IsCredentialActive(u) {
			blocked++
			depleted++
		} else if !u.ExpireTime.IsZero() && u.ExpireTime.After(now) && u.ExpireTime.Sub(now) <= warnWindow {
			expiring++
		}
	}
	b.WriteString(fmt.Sprintf("用户 / Users: %d (online %d, blocked %d)\n", len(users), online, blocked))
	b.WriteString(fmt.Sprintf("累计流量 / Total Traffic: %s\n", formatBytes(totalTraffic)))
	if depleted > 0 || expiring > 0 {
		b.WriteString(fmt.Sprintf("到期预警 / Expiring: %d   超额 / Depleted: %d\n", expiring, depleted))
	}
	return b.String()
}

// sendBackup zips the SQLite DB + Mihomo config into memory and sends the
// resulting archive as a document to every admin chat. Silently no-ops when
// neither path is configured (cannot build a meaningful archive).
func (s *Scheduler) sendBackup(client *Client) {
	if s.dbPath == "" && s.mihomoCfg == "" {
		return
	}
	var buf bytes.Buffer
	if err := system.WriteZip(&buf, system.BackupPaths{DatabasePath: s.dbPath, MihomoConfig: s.mihomoCfg}); err != nil {
		log.Printf("telegram: scheduler backup zip: %v", err)
		return
	}
	name := fmt.Sprintf("3m-ui-backup-%s.zip", time.Now().UTC().Format("20060102-150405"))
	body := bytes.NewReader(buf.Bytes())
	for _, chatID := range client.ChatIDs {
		if _, err := body.Seek(0, 0); err != nil {
			log.Printf("telegram: scheduler backup seek: %v", err)
			return
		}
		if err := client.SendDocument(chatID, name, "application/zip", body); err != nil {
			log.Printf("telegram: scheduler send backup to %s: %v", chatID, err)
		}
	}
}

// NotifyEvent emits a generic event notification. Used by the events agent
// (tg-5) for login/cpu/crash etc. Silently drops when the event is not in the
// EnabledEvents allowlist or Telegram is disabled.
func (s *Scheduler) NotifyEvent(event string, details map[string]string) {
	if s == nil || s.db == nil {
		return
	}
	settings, err := LoadSettings(s.db)
	if err != nil {
		return
	}
	if !settings.Enabled || !settings.EventEnabled(event) {
		return
	}
	if strings.TrimSpace(settings.BotToken) == "" || len(settings.ChatIDs) == 0 {
		return
	}
	client := NewClient(settings.BotToken, settings.ChatIDs, settings.ProxyURL, settings.APIServer)
	if !client.Enabled() {
		return
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("🔔 <b>%s</b>\n", escapeHTML(event)))
	b.WriteString(fmt.Sprintf("时间 / Time: <code>%s</code>\n", time.Now().Format("2006-01-02 15:04:05")))
	for k, v := range details {
		b.WriteString(fmt.Sprintf("%s: <code>%s</code>\n", escapeHTML(k), escapeHTML(v)))
	}
	if err := client.SendText(b.String()); err != nil {
		log.Printf("telegram: NotifyEvent %s failed: %v", event, err)
	}
}

// scheduleMatch reports whether the given spec is "due" at time `now`. `last`
// is the timestamp of the previous successful fire (used to prevent double
// fires within the same minute for cron specs and within the same interval for
// @every). Returns the timestamp to record as lastFired (typically `now`).
func scheduleMatch(spec string, now, last time.Time) (bool, time.Time) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		spec = "@daily"
	}
	// @every <duration>
	if strings.HasPrefix(spec, "@every ") {
		durStr := strings.TrimSpace(strings.TrimPrefix(spec, "@every "))
		d, err := time.ParseDuration(durStr)
		if err != nil || d <= 0 {
			return false, now
		}
		if last.IsZero() || now.Sub(last) >= d {
			return true, now
		}
		return false, now
	}
	switch strings.ToLower(spec) {
	case "@hourly":
		if now.Minute() != 0 {
			return false, now
		}
		if last.IsZero() || last.Hour() != now.Hour() || last.Day() != now.Day() {
			return true, now
		}
		return false, now
	case "@daily", "@midnight":
		if now.Hour() != 0 || now.Minute() != 0 {
			return false, now
		}
		if last.IsZero() || last.Day() != now.Day() {
			return true, now
		}
		return false, now
	case "@weekly":
		if now.Weekday() != time.Sunday || now.Hour() != 0 || now.Minute() != 0 {
			return false, now
		}
		if last.IsZero() || last.Day() != now.Day() {
			return true, now
		}
		return false, now
	case "@monthly":
		if now.Day() != 1 || now.Hour() != 0 || now.Minute() != 0 {
			return false, now
		}
		if last.IsZero() || now.Month() != last.Month() || now.Year() != last.Year() {
			return true, now
		}
		return false, now
	}
	// 5-field cron: minute hour day month weekday
	fields := strings.Fields(spec)
	if len(fields) != 5 {
		return false, now
	}
	match := cronFieldMatches(fields[0], now.Minute()) &&
		cronFieldMatches(fields[1], now.Hour()) &&
		cronFieldMatches(fields[2], now.Day()) &&
		cronFieldMatches(fields[3], int(now.Month())) &&
		cronFieldMatches(fields[4], int(now.Weekday()))
	if !match {
		return false, now
	}
	// Avoid double-firing within the same minute (defensive — the 60s ticker
	// should already align, but cron specs share this guard).
	if !last.IsZero() && now.Sub(last) < time.Minute {
		return false, now
	}
	return true, now
}

// cronFieldMatches reports whether a single cron field matches a value.
// Supports "*" (wildcard), comma "," lists, exact integer values, "*/N"
// (step from base 0), "A-B" (inclusive range), and "A-B/N" (stepped range).
func cronFieldMatches(field string, value int) bool {
	field = strings.TrimSpace(field)
	if field == "*" || field == "" {
		return true
	}
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "*" || part == "" {
			return true
		}
		// Step syntax: "*/N", "A-B/N", "A/N".
		if slashIdx := strings.IndexByte(part, '/'); slashIdx >= 0 {
			rangePart := strings.TrimSpace(part[:slashIdx])
			step, err := strconv.Atoi(strings.TrimSpace(part[slashIdx+1:]))
			if err != nil || step <= 0 {
				continue
			}
			if rangePart == "*" || rangePart == "" {
				// "*/N" — match every Nth value (0, N, 2N, ...).
				if value%step == 0 {
					return true
				}
				continue
			}
			// "A-B/N" — match values in [A,B] that are A + k*step.
			lo, hi, ok := parseCronRange(rangePart)
			if !ok {
				continue
			}
			if value >= lo && value <= hi && (value-lo)%step == 0 {
				return true
			}
			continue
		}
		// Plain inclusive range "A-B".
		if strings.Contains(part, "-") {
			lo, hi, ok := parseCronRange(part)
			if ok && value >= lo && value <= hi {
				return true
			}
			continue
		}
		if n, err := strconv.Atoi(part); err == nil && n == value {
			return true
		}
	}
	return false
}

// parseCronRange parses an "A-B" cron range expression into [lo, hi].
// Returns ok=false when either end is missing, non-numeric, or lo > hi.
func parseCronRange(s string) (int, int, bool) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	lo, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	hi, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || lo > hi {
		return 0, 0, false
	}
	return lo, hi, true
}
