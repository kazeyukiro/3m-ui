package telegram

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

type Notifier struct { db *gorm.DB; mu sync.Mutex; initialized bool; lastBlocked map[uint]bool }
func NewNotifier(db *gorm.DB)*Notifier{return &Notifier{db:db,lastBlocked:map[uint]bool{}}}

func(n *Notifier)CheckAndNotify(){
	if n==nil||n.db==nil{return}
	client,settings,err:=NewClientFromDB(n.db);if err!=nil||client==nil{return}
	var users []models.ProxyUser;if err:=n.db.Find(&users).Error;err!=nil{return}
	now:=time.Now();current:=make(map[uint]bool,len(users));blockedNow:=make([]models.ProxyUser,0)
	for _,u:=range users{if !user.IsCredentialActive(u){current[u.ID]=true;blockedNow=append(blockedNow,u)}}
	n.mu.Lock();prev:=n.lastBlocked;firstRun:=!n.initialized;n.initialized=true;n.lastBlocked=current;n.mu.Unlock()
	if !firstRun{
		for _,u:=range blockedNow{if prev[u.ID]{continue};reason:=blockReason(u);if reason=="expired"&&!settings.NotifyOnExpiry{continue};if reason!="expired"&&!settings.NotifyOnBlock{continue};msg:=fmt.Sprintf("⛔ <b>用户已被阻止 / User blocked</b>\n用户 / User：<code>%s</code>\n原因 / Reason：%s\n时间 / Time：%s",escapeHTML(u.Username),reasonText(reason),now.Format("2006-01-02 15:04:05"));if err:=client.SendText(msg);err!=nil{log.Printf("telegram: block notification failed: %v",err)}}
		if settings.NotifyOnUnblock{for id:=range prev{if current[id]{continue};var u models.ProxyUser;if err:=n.db.First(&u,id).Error;err!=nil{continue};msg:=fmt.Sprintf("✅ <b>用户已恢复 / User restored</b>\n用户 / User：<code>%s</code>\n时间 / Time：%s",escapeHTML(u.Username),now.Format("2006-01-02 15:04:05"));if err:=client.SendText(msg);err!=nil{log.Printf("telegram: unblock notification failed: %v",err)}}}
	}
	if settings.NotifyDailyDigest{
		today:=now.Format("2006-01-02")
		if !dailyDigestSent(n.db,today){
			if err:=client.SendText(dailyDigest(users,now));err!=nil{log.Printf("telegram: daily digest failed: %v",err)}else if err:=markDailyDigestSent(n.db,today);err!=nil{log.Printf("telegram: persist daily digest state failed: %v",err)}
		}
	}
}

func dailyDigest(users []models.ProxyUser,now time.Time)string{var total,used,blocked int64;for _,u:=range users{total++;used+=u.TrafficUsed;if !user.IsCredentialActive(u){blocked++}};return fmt.Sprintf("📊 <b>3m-ui 每日摘要 / Daily Summary</b>\n用户数 / Users：%d\n已阻止 / Blocked：%d\n累计流量 / Traffic：%s\n时间 / Time：%s",total,blocked,formatBytes(used),now.Format("2006-01-02 15:04:05"))}
func reasonText(reason string)string{switch reason{case "disabled":return "用户已禁用 / Disabled";case "expired":return "已过期 / Expired";case "traffic_limit":return "流量已用尽 / Traffic limit reached";default:return "凭据不可用 / Credentials unavailable"}}
func blockReason(u models.ProxyUser)string{now:=time.Now();if !u.Enabled{return "disabled"};if !u.ExpireTime.IsZero()&&!u.ExpireTime.After(now){return "expired"};if u.TrafficLimit>0&&u.TrafficUsed>=u.TrafficLimit{return "traffic_limit"};return "blocked"}
func escapeHTML(s string)string{r:=strings.NewReplacer("&","&amp;","<","&lt;",">","&gt;","\"","&quot;");return r.Replace(s)}
