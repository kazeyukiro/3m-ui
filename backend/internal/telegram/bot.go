package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kazeyukiro/3m-ui/backend/internal/mihomo"
	"github.com/kazeyukiro/3m-ui/backend/internal/user"
	"gorm.io/gorm"
)

type Bot struct { db *gorm.DB; mihomo *mihomo.Service; users *user.Service; mu sync.Mutex; stopCh chan struct{}; wg sync.WaitGroup; wizardMu sync.Mutex; wizards map[string]*addClientWizard }
type addClientWizard struct { ListenerID uint }

func NewBot(db *gorm.DB,mihomoSvc *mihomo.Service,userSvc *user.Service)*Bot{return &Bot{db:db,mihomo:mihomoSvc,users:userSvc,stopCh:make(chan struct{}),wizards:make(map[string]*addClientWizard)}}
func(b *Bot)Start(){if b==nil{return};b.wg.Add(1);go func(){defer b.wg.Done();b.loop()}();log.Printf("telegram: bot command loop started")}
func(b *Bot)Stop(){if b==nil{return};b.mu.Lock();select{case <-b.stopCh:default:close(b.stopCh)};b.mu.Unlock();b.wg.Wait()}

func(b *Bot)restoreWizard(chatID string){b.wizardMu.Lock();if _,ok:=b.wizards[chatID];ok{b.wizardMu.Unlock();return};b.wizardMu.Unlock();state,err:=loadWizardState(b.db,chatID);if err!=nil{return};if state!=nil{b.wizardMu.Lock();if _,ok:=b.wizards[chatID];!ok{b.wizards[chatID]=state};b.wizardMu.Unlock()}}
func(b *Bot)persistWizard(chatID string){b.wizardMu.Lock();state,ok:=b.wizards[chatID];b.wizardMu.Unlock();if ok{if err:=saveWizardState(b.db,chatID,*state);err!=nil{log.Printf("telegram: persist wizard %s: %v",chatID,err)}}}
func(b *Bot)clearWizard(chatID string){b.wizardMu.Lock();delete(b.wizards,chatID);b.wizardMu.Unlock();if err:=clearWizardState(b.db,chatID);err!=nil{log.Printf("telegram: clear wizard %s: %v",chatID,err)}}

func(b *Bot)loop(){
	offset:=LoadUpdateOffset(b.db);commandsToken:="";httpClient:=&http.Client{Timeout:45*time.Second}
	for{
		select{case <-b.stopCh:return;default:}
		tgClient,settings,err:=NewClientFromDB(b.db)
		if err!=nil||tgClient==nil||!settings.Enabled{select{case <-b.stopCh:return;case <-time.After(15*time.Second)};continue}
		if commandsToken!=settings.BotToken{if err:=setCommandsWithRetry(tgClient,5);err!=nil{log.Printf("telegram: set commands failed after retries: %v",err)}else{commandsToken=settings.BotToken}}
		updates,_,err:=getUpdates(httpClient,settings.BotToken,offset,25)
		if err!=nil{log.Printf("telegram: getUpdates: %v",err);select{case <-b.stopCh:return;case <-time.After(5*time.Second)};continue}
		for _,u:=range updates{
			if u.UpdateID<offset{continue}
			if err:=b.processUpdate(tgClient,settings,u);err!=nil{log.Printf("telegram: update %d failed: %v",u.UpdateID,err);break}
			next:=u.UpdateID+1
			if next>offset{if err:=SaveUpdateOffset(b.db,next);err!=nil{log.Printf("telegram: persist update offset %d: %v",next,err);break};offset=next}
		}
	}
}

func(b *Bot)processUpdate(c *Client,settings Settings,u tgUpdate)error{
	if u.CallbackQuery!=nil{
		chatID:=strconv.FormatInt(u.CallbackQuery.Message.Chat.ID,10)
		current,err:=LoadSettings(b.db);if err!=nil{return err}
		if !current.Enabled||current.BotToken!=settings.BotToken{return c.AnswerCallback(u.CallbackQuery.ID,"Configuration changed / 配置已变更")}
		return b.handleCallbackGuarded(c,current,chatID,u.CallbackQuery.ID,u.CallbackQuery.Data)
	}
	if u.Message==nil||strings.TrimSpace(u.Message.Text)==""{return nil}
	chatID:=strconv.FormatInt(u.Message.Chat.ID,10)
	current,err:=LoadSettings(b.db);if err!=nil{return err}
	if !current.Enabled||current.BotToken!=settings.BotToken{return nil}
	text:=strings.TrimSpace(u.Message.Text)
	if strings.EqualFold(strings.Fields(text)[0],"/cancel"){
		b.clearWizard(chatID)
		return c.SendTo(chatID,"Client creation wizard cancelled / 客户端创建向导已取消。",nil)
	}
	if strings.HasPrefix(strings.ToLower(text),"/bind ")||strings.HasPrefix(strings.ToLower(text),"/bind\t"){
		parts:=strings.Fields(text)
		if len(parts)>=3&&!b.isAdmin(chatID,current){return c.SendTo(chatID,"Permission denied / 无权限。",nil)}
		if len(parts)>=3{return c.SendTo(chatID,bindUserTransactional(b.db,parts[1],parts[2]),nil)}
	}
	b.restoreWizard(chatID)
	if b.handleWizardMessage(chatID,text){b.clearWizard(chatID);return nil}
	reply,markup:=b.handleCommand(chatID,text)
	return c.SendTo(chatID,reply,markup)
}

func(b *Bot)handleCallbackGuarded(c *Client,s Settings,chatID,callbackID,data string)error{current,err:=LoadSettings(b.db);if err!=nil{return err};if !current.Enabled||current.BotToken!=s.BotToken{return c.AnswerCallback(callbackID,"Configuration changed / 配置已变更")};if strings.HasPrefix(data,"admin:")||strings.HasPrefix(data,"add:"){if !b.isAdmin(chatID,current){return c.AnswerCallback(callbackID,"Permission denied / 无权限")}};if data=="admin:add"||strings.HasPrefix(data,"add:"){b.wizardMu.Lock();_,active:=b.wizards[chatID];b.wizardMu.Unlock();if active{return c.AnswerCallback(callbackID,"Wizard already active / 向导已在进行中")}};err=b.handleCallback(c,current,chatID,callbackID,data);b.persistWizard(chatID);return err}

func setCommandsWithRetry(c *Client,attempts int)error{return setCommandsWithRetryFn(c.SetCommands,attempts)}
func setCommandsWithRetryFn(fn func()error,attempts int)error{if attempts<=0{return fmt.Errorf("telegram: invalid retry count %d",attempts)};var err error;for i:=0;i<attempts;i++{if err=fn();err==nil{return nil};if i+1<attempts{delay:=time.Duration(1<<i)*time.Second;log.Printf("telegram: set commands attempt %d/%d failed: %v; retrying in %s",i+1,attempts,err,delay);time.Sleep(delay)}};return err}

type tgUpdate struct{UpdateID int64 `json:"update_id"`;Message *struct{Text string `json:"text"`;Chat struct{ID int64 `json:"id"`} `json:"chat"`} `json:"message"`;CallbackQuery *struct{ID string `json:"id"`;Data string `json:"data"`;Message struct{Chat struct{ID int64 `json:"id"`} `json:"chat"`} `json:"message"`} `json:"callback_query"`}
type tgUpdatesResponse struct{OK bool `json:"ok"`;Result []tgUpdate `json:"result"`;Description string `json:"description"`;Parameters *struct{RetryAfter int `json:"retry_after"`} `json:"parameters,omitempty"`}
func parseUpdates(raw []byte,offset int64)([]tgUpdate,int64,error){var parsed tgUpdatesResponse;if err:=json.Unmarshal(raw,&parsed);err!=nil{return nil,offset,err};if !parsed.OK{return nil,offset,fmt.Errorf("telegram API: %s",parsed.Description)};next:=offset;for _,u:=range parsed.Result{if u.UpdateID+1>next{next=u.UpdateID+1}};return parsed.Result,next,nil}
func getUpdates(httpClient *http.Client,token string,offset int64,timeoutSec int)([]tgUpdate,int64,error){q:=url.Values{};q.Set("timeout",strconv.Itoa(timeoutSec));q.Set("allowed_updates",`["message","callback_query"]`);if offset>0{q.Set("offset",strconv.FormatInt(offset,10))};for attempt:=0;attempt<4;attempt++{resp,err:=httpClient.Get(fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?%s",token,q.Encode()));if err!=nil{if attempt<3{time.Sleep(time.Duration(1<<attempt)*time.Second);continue};return nil,offset,err};raw,readErr:=io.ReadAll(io.LimitReader(resp.Body,1<<20));resp.Body.Close();if readErr!=nil{return nil,offset,readErr};if resp.StatusCode>=500||resp.StatusCode==429{var p tgUpdatesResponse;_ = json.Unmarshal(raw,&p);delay:=time.Duration(1<<attempt)*time.Second;if p.Parameters!=nil&&p.Parameters.RetryAfter>0{delay=time.Duration(p.Parameters.RetryAfter)*time.Second};if attempt<3{time.Sleep(delay);continue};return nil,offset,fmt.Errorf("telegram API HTTP %d: %s",resp.StatusCode,strings.TrimSpace(string(raw)))};if resp.StatusCode>=300{return nil,offset,fmt.Errorf("telegram API HTTP %d: %s",resp.StatusCode,strings.TrimSpace(string(raw)))};return parseUpdates(raw,offset)};return nil,offset,fmt.Errorf("telegram getUpdates retries exhausted")}
