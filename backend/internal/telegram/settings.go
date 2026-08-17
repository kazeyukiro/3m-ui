package telegram

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/kazeyukiro/3m-ui/backend/internal/database/models"
	"gorm.io/gorm"
)

const settingKey = "telegram"
const updateOffsetKey = "telegram_update_offset"

type Settings struct { Enabled bool `json:"enabled"`; BotToken string `json:"bot_token"`; ChatIDs []string `json:"chat_ids"`; NotifyOnBlock bool `json:"notify_on_block"`; NotifyOnUnblock bool `json:"notify_on_unblock"`; NotifyOnExpiry bool `json:"notify_on_expiry"`; NotifyDailyDigest bool `json:"notify_daily_digest"` }
func DefaultSettings() Settings { return Settings{NotifyOnBlock:true,NotifyOnUnblock:true,NotifyOnExpiry:true} }
func LoadSettings(db *gorm.DB) (Settings,error) { s:=DefaultSettings();if db==nil{return s,nil};var row models.PanelSetting;if err:=db.Where("key = ?",settingKey).First(&row).Error;err!=nil{if err==gorm.ErrRecordNotFound{return s,nil};return s,err};if strings.TrimSpace(row.Value)==""{return s,nil};if err:=json.Unmarshal([]byte(row.Value),&s);err!=nil{return DefaultSettings(),err};return s,nil }
func SaveSettings(db *gorm.DB,s Settings) error {clean:=make([]string,0,len(s.ChatIDs));for _,id:=range s.ChatIDs{if id=strings.TrimSpace(id);id!=""{clean=append(clean,id)}};s.ChatIDs=clean;s.BotToken=strings.TrimSpace(s.BotToken);raw,err:=json.Marshal(s);if err!=nil{return err};var row models.PanelSetting;err=db.Where("key = ?",settingKey).First(&row).Error;if err==gorm.ErrRecordNotFound{return db.Create(&models.PanelSetting{Key:settingKey,Value:string(raw)}).Error};if err!=nil{return err};row.Value=string(raw);return db.Save(&row).Error }
func LoadUpdateOffset(db *gorm.DB) int64 { if db==nil{return 0};var row models.PanelSetting;if err:=db.Where("key = ?",updateOffsetKey).First(&row).Error;err!=nil{return 0};var offset int64;if _,err:=fmt.Sscanf(strings.TrimSpace(row.Value),"%d",&offset);err!=nil||offset<0{return 0};return offset }
func SaveUpdateOffset(db *gorm.DB,offset int64) error { if db==nil{return nil};var row models.PanelSetting;err:=db.Where("key = ?",updateOffsetKey).First(&row).Error;if err==gorm.ErrRecordNotFound{return db.Create(&models.PanelSetting{Key:updateOffsetKey,Value:strconv.FormatInt(offset,10)}).Error};if err!=nil{return err};row.Value=strconv.FormatInt(offset,10);return db.Save(&row).Error }
func NewClientFromDB(db *gorm.DB) (*Client,Settings,error) {s,err:=LoadSettings(db);if err!=nil{return nil,s,err};if !s.Enabled||s.BotToken==""||len(s.ChatIDs)==0{return nil,s,nil};return NewClient(s.BotToken,s.ChatIDs),s,nil}
