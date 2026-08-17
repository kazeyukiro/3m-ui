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

func NewBot(db *gorm.DB, mihomoSvc *mihomo.Service, userSvc *user.Service) *Bot { return &Bot{db:db,mihomo:mihomoSvc,users:userSvc,stopCh:make(chan struct{}),wizards:make(map[string]*addClientWizard)} }
func (b *Bot) Start(){if b==nil{return};b.wg.Add(1);go func(){defer b.wg.Done();b.loop()}();log.Printf("telegram: bot command loop started")}
func (b *Bot) Stop(){if b==nil{return};b.mu.Lock();select{case <-b.stopCh:default:close(b.stopCh)};b.mu.Unlock();b.wg.Wait()}

func (b *Bot) loop(){var offset int64;commandsSet:=false;client:=&http.Client{Timeout:45*time.Second};for{select{case <-b.stopCh:return;default:};tgClient,settings,err:=NewClientFromDB(b.db);if err!=nil||tgClient==nil||!settings.Enabled{select{case <-b.stopCh:return;case <-time.After(15*time.Second)};continue};if !commandsSet{if err:=tgClient.SetCommands();err!=nil{log.Printf("telegram: set commands: %v",err)}else{commandsSet=true}};updates,next,err:=getUpdates(client,settings.BotToken,offset,25);if err!=nil{log.Printf("telegram: getUpdates: %v",err);select{case <-b.stopCh:return;case <-time.After(5*time.Second)};continue};if next>offset{offset=next};for _,u:=range updates{if u.CallbackQuery!=nil{chatID:=strconv.FormatInt(u.CallbackQuery.Message.Chat.ID,10);if err:=b.handleCallback(tgClient,settings,chatID,u.CallbackQuery.ID,u.CallbackQuery.Data);err!=nil{log.Printf("telegram: callback: %v",err)};continue};if u.Message==nil||strings.TrimSpace(u.Message.Text)==""{continue};chatID:=strconv.FormatInt(u.Message.Chat.ID,10);if b.handleWizardMessage(chatID,u.Message.Text){continue};reply,markup:=b.handleCommand(chatID,u.Message.Text);if err:=tgClient.SendTo(chatID,reply,markup);err!=nil{log.Printf("telegram: reply: %v",err)}}}}

type tgUpdate struct{UpdateID int64 `json:"update_id"`;Message *struct{Text string `json:"text"`;Chat struct{ID int64 `json:"id"`} `json:"chat"`} `json:"message"`;CallbackQuery *struct{ID string `json:"id"`;Data string `json:"data"`;Message struct{Chat struct{ID int64 `json:"id"`} `json:"chat"`} `json:"message"`} `json:"callback_query"`}
func getUpdates(httpClient *http.Client,token string,offset int64,timeoutSec int)([]tgUpdate,int64,error){q:=url.Values{};q.Set("timeout",strconv.Itoa(timeoutSec));q.Set("allowed_updates",`["message","callback_query"]`);if offset>0{q.Set("offset",strconv.FormatInt(offset,10))};resp,err:=httpClient.Get(fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?%s",token,q.Encode()));if err!=nil{return nil,offset,err};defer resp.Body.Close();raw,err:=io.ReadAll(io.LimitReader(resp.Body,1<<20));if err!=nil{return nil,offset,err};if resp.StatusCode>=300{return nil,offset,fmt.Errorf("HTTP %d: %s",resp.StatusCode,strings.TrimSpace(string(raw)))};var parsed struct{OK bool `json:"ok"`;Result []tgUpdate `json:"result"`;Description string `json:"description"`};if err:=json.Unmarshal(raw,&parsed);err!=nil{return nil,offset,err};if !parsed.OK{return nil,offset,fmt.Errorf("telegram getUpdates: %s",parsed.Description)};next:=offset;for _,u:=range parsed.Result{if u.UpdateID+1>next{next=u.UpdateID+1}};return parsed.Result,next,nil}
