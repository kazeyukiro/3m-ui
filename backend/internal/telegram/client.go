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
	"unicode/utf8"
)

type Client struct { Token string; ChatIDs []string; HTTPClient *http.Client }
func NewClient(token string, chatIDs []string) *Client { clean:=make([]string,0,len(chatIDs)); for _,id:=range chatIDs { if id=strings.TrimSpace(id); id!="" { clean=append(clean,id) } }; return &Client{Token:strings.TrimSpace(token),ChatIDs:clean,HTTPClient:&http.Client{Timeout:30*time.Second}} }
func (c *Client) Enabled() bool { return c!=nil && c.Token!="" && len(c.ChatIDs)>0 }
type sendMessageRequest struct { ChatID string `json:"chat_id"`; Text string `json:"text"`; ParseMode string `json:"parse_mode,omitempty"`; ReplyMarkup interface{} `json:"reply_markup,omitempty"` }
type telegramResponse struct { OK bool `json:"ok"`; Description string `json:"description"` }
type InlineKeyboardMarkup struct { InlineKeyboard [][]InlineButton `json:"inline_keyboard"` }
type InlineButton struct { Text string `json:"text"`; CallbackData string `json:"callback_data,omitempty"`; URL string `json:"url,omitempty"` }

func (c *Client) SendText(text string) error { if !c.Enabled(){return fmt.Errorf("telegram is not configured")}; var last error; delivered:=0; for _,chatID:=range c.ChatIDs { if err:=c.SendTo(chatID,text,nil); err!=nil {last=err} else {delivered++} }; if delivered==0 {if last!=nil{return last};return fmt.Errorf("no telegram chats delivered")};return nil }
func (c *Client) SendTo(chatID,text string,markup *InlineKeyboardMarkup) error { if c==nil||c.Token=="" {return fmt.Errorf("telegram is not configured")}; chunks:=splitTelegramHTML(text,4096); for i,chunk:=range chunks { var m *InlineKeyboardMarkup; if i==len(chunks)-1 {m=markup}; if err:=c.sendOne(strings.TrimSpace(chatID),chunk,m);err!=nil{return err} };return nil }
func (c *Client) SendDocument(chatID,filename string,data []byte) error { if c==nil||c.Token=="" {return fmt.Errorf("telegram is not configured")};var buf bytes.Buffer;mw:=multipart.NewWriter(&buf);if err:=mw.WriteField("chat_id",chatID);err!=nil{return err};part,err:=mw.CreateFormFile("document",filename);if err!=nil{return err};if _,err:=part.Write(data);err!=nil{return err};if err:=mw.Close();err!=nil{return err};req,err:=http.NewRequest(http.MethodPost,fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument",c.Token),&buf);if err!=nil{return err};req.Header.Set("Content-Type",mw.FormDataContentType());resp,err:=c.HTTPClient.Do(req);if err!=nil{return err};defer resp.Body.Close();raw,_:=io.ReadAll(io.LimitReader(resp.Body,4096));var parsed telegramResponse;if err:=json.Unmarshal(raw,&parsed);err!=nil{return err};if resp.StatusCode>=300||!parsed.OK{return fmt.Errorf("telegram API %d: %s",resp.StatusCode,parsed.Description)};return nil }
func (c *Client) AnswerCallback(callbackID,text string) error { endpoint:=fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery",c.Token);body,_:=json.Marshal(map[string]string{"callback_query_id":callbackID,"text":text});return c.post(endpoint,body) }
func (c *Client) SetCommands() error { commands:=[]map[string]string{{"command":"start","description":"打开菜单 / Open menu"},{"command":"help","description":"帮助 / Help"},{"command":"status","description":"服务状态 / Service status"},{"command":"id","description":"Telegram ID / Telegram ID"},{"command":"usage","description":"用量 / Usage"},{"command":"inbound","description":"入站 / Inbound"},{"command":"restart","description":"重启 Mihomo / Restart Mihomo"}};body,_:=json.Marshal(map[string]interface{}{"commands":commands});return c.post(fmt.Sprintf("https://api.telegram.org/bot%s/setMyCommands",c.Token),body) }
func (c *Client) Validate() error { if c==nil||c.Token=="" {return fmt.Errorf("telegram bot token is empty")};resp,err:=c.HTTPClient.Get(fmt.Sprintf("https://api.telegram.org/bot%s/getMe",c.Token));if err!=nil{return err};defer resp.Body.Close();raw,_:=io.ReadAll(io.LimitReader(resp.Body,4096));var parsed telegramResponse;if err:=json.Unmarshal(raw,&parsed);err!=nil{return err};if resp.StatusCode>=300||!parsed.OK{return fmt.Errorf("telegram API %d: %s",resp.StatusCode,parsed.Description)};return nil }
func (c *Client) sendOne(chatID,text string,markup *InlineKeyboardMarkup) error { body,err:=json.Marshal(sendMessageRequest{ChatID:chatID,Text:text,ParseMode:"HTML",ReplyMarkup:markup});if err!=nil{return err};return c.post(fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage",c.Token),body) }
func (c *Client) post(endpoint string,body []byte) error { req,err:=http.NewRequest(http.MethodPost,endpoint,bytes.NewReader(body));if err!=nil{return err};req.Header.Set("Content-Type","application/json");resp,err:=c.HTTPClient.Do(req);if err!=nil{return err};defer resp.Body.Close();raw,_:=io.ReadAll(io.LimitReader(resp.Body,4096));var parsed telegramResponse;if err:=json.Unmarshal(raw,&parsed);err!=nil{return fmt.Errorf("telegram API returned invalid response: %w",err)};if resp.StatusCode>=300||!parsed.OK{return fmt.Errorf("telegram API %d: %s",resp.StatusCode,parsed.Description)};return nil }

// splitTelegramHTML keeps simple Telegram HTML tags intact and prefers newline
// boundaries. It also reopens active tags in the next chunk.
func splitTelegramHTML(text string, max int) []string { if utf8.RuneCountInString(text)<=max{return []string{text}};var chunks []string;var buf strings.Builder;var stack []string;visible:=0;flush:=func(){if buf.Len()==0{return};s:=buf.String();for i:=len(stack)-1;i>=0;i--{s+="</"+stack[i]+">"};chunks=append(chunks,s);buf.Reset();visible=0;for _,tag:=range stack{buf.WriteString("<"+tag+">")}};for i:=0;i<len(text);{if text[i]=='<' {j:=strings.IndexByte(text[i:],'>');if j<0 {break};j+=i+1;tagRaw:=text[i+1:j-1];lower:=strings.ToLower(strings.TrimSpace(tagRaw));buf.WriteString(text[i:j]);if strings.HasPrefix(lower,"/"){name:=strings.TrimSpace(strings.TrimPrefix(lower,"/"));for k:=len(stack)-1;k>=0;k--{if stack[k]==name{stack=append(stack[:k],stack[k+1:]...);break}}} else if lower=="b"||lower=="strong"||lower=="i"||lower=="em"||lower=="u"||lower=="s"||lower=="code"||lower=="pre" {stack=append(stack,lower)};i=j;continue};r,n:=utf8.DecodeRuneInString(text[i:]);if visible+1>max-64 {flush();continue};buf.WriteRune(r);visible++;i+=n;if r=='\n'&&visible>max/2 { } };if buf.Len()>0 {s:=buf.String();for i:=len(stack)-1;i>=0;i--{s+="</"+stack[i]+">"};chunks=append(chunks,s)};return chunks }
