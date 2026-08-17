package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

type Client struct { Token string; ChatIDs []string; HTTPClient *http.Client }
func NewClient(token string, chatIDs []string) *Client { clean:=make([]string,0,len(chatIDs));for _,id:=range chatIDs{if id=strings.TrimSpace(id);id!=""{clean=append(clean,id)}};return &Client{Token:strings.TrimSpace(token),ChatIDs:clean,HTTPClient:&http.Client{Timeout:30*time.Second}} }
func (c *Client) Enabled() bool{return c!=nil&&c.Token!=""&&len(c.ChatIDs)>0}
type sendMessageRequest struct{ChatID string `json:"chat_id"`;Text string `json:"text"`;ParseMode string `json:"parse_mode,omitempty"`;ReplyMarkup interface{} `json:"reply_markup,omitempty"`}
type telegramResponse struct{OK bool `json:"ok"`;Description string `json:"description"`}
type InlineKeyboardMarkup struct{InlineKeyboard [][]InlineButton `json:"inline_keyboard"`}
type InlineButton struct{Text string `json:"text"`;CallbackData string `json:"callback_data,omitempty"`;URL string `json:"url,omitempty"`}

// bilingualize keeps the existing Chinese bot implementation intact while
// making every outbound bot message bilingual. Dynamic values are preserved.
// Messages that already contain an English translation are returned unchanged.
func bilingualize(text string) string {
	if strings.Contains(text, " / ") || strings.Contains(text, "English") { return text }
	r := strings.NewReplacer(
		"你的 Telegram ID：", "Your Telegram ID / 你的 Telegram ID: ",
		"未知指令。发送 /help 查看菜单。", "Unknown command. Send /help to view the menu. / 未知指令。发送 /help 查看菜单。",
		"这个命令只对管理员开放。", "This command is for administrators only. / 这个命令只对管理员开放。",
		"无权限。", "Permission denied. / 无权限。",
		"还没有绑定 3m-ui 客户端。请把你的 Telegram ID 发给管理员绑定。", "No 3m-ui client is linked yet. Send your Telegram ID to an administrator to link it. / 还没有绑定 3m-ui 客户端。请把你的 Telegram ID 发给管理员绑定。",
		"没有找到客户端。", "Client not found. / 没有找到客户端。",
		"查询失败：", "Query failed / 查询失败: ",
		"没有找到这个入站。", "Inbound not found. / 没有找到这个入站。",
		"Telegram ID 无效。", "Invalid Telegram ID. / Telegram ID 无效。",
		"绑定失败：", "Binding failed / 绑定失败: ",
		"暂无客户端。", "No clients. / 暂无客户端。",
		"读取客户端失败。", "Failed to read clients. / 读取客户端失败。",
		"当前没有在线客户端。", "No clients are currently online. / 当前没有在线客户端。",
		"暂无入站。", "No inbounds. / 暂无入站。",
		"暂无数据。", "No data. / 暂无数据。",
		"重启失败：", "Restart failed / 重启失败: ",
		"Mihomo 服务未初始化。", "Mihomo service is not initialized. / Mihomo 服务未初始化。",
		"Mihomo 已重启。", "Mihomo restarted successfully. / Mihomo 已重启。",
		"用法：", "Usage / 用法: ",
		"选择要绑定新客户端的入站：", "Select an inbound for the new client / 选择要绑定新客户端的入站：",
		"请输入客户端用户名：", "Enter the client username / 请输入客户端用户名：",
		"客户端已创建", "Client created / 客户端已创建",
		"用户名：", "Username / 用户名: ",
		"密码：", "Password / 密码: ",
		"流量排行", "Traffic Ranking / 流量排行",
		"在线客户端", "Online Clients / 在线客户端",
		"客户端", "Clients / 客户端",
		"入站", "Inbounds / 入站",
		"服务状态", "Service Status / 服务状态",
		"版本：", "Version / 版本: ",
		"PID：", "PID: ",
		"客户端：", "Clients / 客户端: ",
		"入站：", "Inbounds / 入站: ",
		"运行中", "Running / 运行中",
		"已停止", "Stopped / 已停止",
		"已用：", "Used / 已用: ",
		"上传：", "Upload / 上传: ",
		"下载：", "Download / 下载: ",
		"到期：", "Expires / 到期: ",
		"状态：", "Status / 状态: ",
		"正常", "Active / 正常",
		"已停用/过期", "Disabled/Expired / 已停用/过期",
		"永不过期", "Never expires / 永不过期",
		"不限", "Unlimited / 不限",
		"协议：", "Protocol / 协议: ",
		"地址：", "Address / 地址: ",
		"端口：", "Port / 端口: ",
		"状态：", "Status / 状态: ",
		"已将 ", "Bound ",
		" 绑定到 Telegram ID ", " to Telegram ID ",
	)
	return r.Replace(text)
}

func(c *Client)SendText(text string)error{if !c.Enabled(){return fmt.Errorf("telegram is not configured")};var last error;ok:=0;for _,chatID:=range c.ChatIDs{if err:=c.sendOne(chatID,text,nil);err!=nil{last=err;continue};ok++};if ok==0{if last!=nil{return last};return fmt.Errorf("no telegram chats delivered")};return nil}
func(c *Client)SendTo(chatID,text string,markup *InlineKeyboardMarkup)error{if c==nil||c.Token==""{return fmt.Errorf("telegram is not configured")};return c.sendOne(strings.TrimSpace(chatID),text,markup)}
func(c *Client)SendDocument(chatID,filename string,data []byte)error{if c==nil||c.Token==""{return fmt.Errorf("telegram is not configured")};var buf bytes.Buffer;mw:=multipart.NewWriter(&buf);if err:=mw.WriteField("chat_id",chatID);err!=nil{return err};part,err:=mw.CreateFormFile("document",filename);if err!=nil{return err};if _,err:=part.Write(data);err!=nil{return err};if err:=mw.Close();err!=nil{return err};req,err:=http.NewRequest(http.MethodPost,fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument",c.Token),&buf);if err!=nil{return err};req.Header.Set("Content-Type",mw.FormDataContentType());resp,err:=c.HTTPClient.Do(req);if err!=nil{return err};defer resp.Body.Close();raw,_:=io.ReadAll(io.LimitReader(resp.Body,4096));var parsed telegramResponse;if err:=json.Unmarshal(raw,&parsed);err!=nil{return err};if resp.StatusCode>=300||!parsed.OK{return fmt.Errorf("telegram API %d: %s",resp.StatusCode,parsed.Description)};return nil}
func(c *Client)AnswerCallback(callbackID,text string)error{endpoint:=fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery",c.Token);body,_:=json.Marshal(map[string]string{"callback_query_id":callbackID,"text":text});return c.post(endpoint,body)}
func(c *Client)SetCommands()error{commands:=[]map[string]string{{"command":"start","description":"打开菜单 / Open menu"},{"command":"help","description":"帮助 / Help"},{"command":"status","description":"服务状态 / Service status"},{"command":"id","description":"Telegram ID / Telegram ID"},{"command":"usage","description":"用量 / Usage"},{"command":"inbound","description":"入站 / Inbound"},{"command":"restart","description":"重启 Mihomo / Restart Mihomo"}};body,_:=json.Marshal(map[string]interface{}{"commands":commands});return c.post(fmt.Sprintf("https://api.telegram.org/bot%s/setMyCommands",c.Token),body)}
func(c *Client)Validate()error{if c==nil||c.Token==""{return fmt.Errorf("telegram bot token is empty")};resp,err:=c.HTTPClient.Get(fmt.Sprintf("https://api.telegram.org/bot%s/getMe",c.Token));if err!=nil{return err};defer resp.Body.Close();var parsed telegramResponse;if err:=json.NewDecoder(io.LimitReader(resp.Body,4096)).Decode(&parsed);err!=nil{return err};if resp.StatusCode>=300||!parsed.OK{return fmt.Errorf("telegram API: %s",parsed.Description)};return nil}
func(c *Client)sendOne(chatID,text string,markup *InlineKeyboardMarkup)error{body,err:=json.Marshal(sendMessageRequest{ChatID:chatID,Text:bilingualize(text),ParseMode:"HTML",ReplyMarkup:markup});if err!=nil{return err};return c.post(fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage",c.Token),body)}
func(c *Client)post(endpoint string,body []byte)error{req,err:=http.NewRequest(http.MethodPost,endpoint,bytes.NewReader(body));if err!=nil{return err};req.Header.Set("Content-Type","application/json");resp,err:=c.HTTPClient.Do(req);if err!=nil{return err};defer resp.Body.Close();raw,_:=io.ReadAll(io.LimitReader(resp.Body,4096));var parsed telegramResponse;if err:=json.Unmarshal(raw,&parsed);err!=nil{return fmt.Errorf("telegram API returned invalid response: %w",err)};if resp.StatusCode>=300||!parsed.OK{return fmt.Errorf("telegram API %d: %s",resp.StatusCode,parsed.Description)};return nil}
