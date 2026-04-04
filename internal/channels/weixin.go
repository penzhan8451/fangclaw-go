package channels

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/penzhan8451/fangclaw-go/internal/config"
)

func init() {
	RegisterAutoRegister(autoRegisterWeixin)
}

func autoRegisterWeixin(registry *Registry, getSecret SecretGetter) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	if cfg.Channels.Weixin != nil {
		token := cfg.Channels.Weixin.Token
		if token == "" && cfg.Channels.Weixin.TokenEnv != "" {
			token = getSecret(cfg.Channels.Weixin.TokenEnv)
		}
		if token != "" {
			channel := &Channel{
				Name:  "weixin",
				Type:  ChannelTypeWeixin,
				State: ChannelStateIdle,
				Config: ChannelAdapterConfig{
					Weixin: &WeixinChannelConfig{
						Token:              token,
						BaseURL:            cfg.Channels.Weixin.BaseURL,
						CDNBaseURL:         cfg.Channels.Weixin.CDNBaseURL,
						Proxy:              cfg.Channels.Weixin.Proxy,
						ReasoningChannelID: cfg.Channels.Weixin.ReasoningChannelID,
					},
				},
			}
			if err := registry.RegisterChannel(channel); err != nil {
				return err
			}
		}
	}
	return nil
}

const (
	weixinChannelVersion       = "2.1.1"
	weixinIlinkAppID           = "bot"
	weixinClientVersion        = 131329
	weixinDefaultCDNBaseURL    = "https://novac2c.cdn.weixin.qq.com/c2c"
	weixinConfigCacheTTL       = 24 * time.Hour
	weixinConfigRetryInitial   = 2 * time.Second
	weixinConfigRetryMax       = time.Hour
	weixinSessionPauseDuration = time.Hour
	weixinSessionExpiredCode   = -14
)

type BaseInfo struct {
	ChannelVersion string `json:"channel_version,omitempty"`
}

type APIStatus struct {
	Ret     int    `json:"ret,omitempty"`
	Errcode int    `json:"errcode,omitempty"`
	Errmsg  string `json:"errmsg,omitempty"`
}

const (
	UploadMediaTypeImage = 1
	UploadMediaTypeVideo = 2
	UploadMediaTypeFile  = 3
	UploadMediaTypeVoice = 4
)

type GetUploadUrlReq struct {
	Filekey         string   `json:"filekey,omitempty"`
	MediaType       int      `json:"media_type,omitempty"`
	ToUserID        string   `json:"to_user_id,omitempty"`
	Rawsize         int64    `json:"rawsize,omitempty"`
	RawfileMD5      string   `json:"rawfilemd5,omitempty"`
	Filesize        int64    `json:"filesize,omitempty"`
	ThumbRawsize    int64    `json:"thumb_rawsize,omitempty"`
	ThumbRawfileMD5 string   `json:"thumb_rawfilemd5,omitempty"`
	ThumbFilesize   int64    `json:"thumb_filesize,omitempty"`
	NoNeedThumb     bool     `json:"no_need_thumb,omitempty"`
	Aeskey          string   `json:"aeskey,omitempty"`
	BaseInfo        BaseInfo `json:"base_info,omitempty"`
}

type GetUploadUrlResp struct {
	APIStatus
	UploadParam      string `json:"upload_param,omitempty"`
	ThumbUploadParam string `json:"thumb_upload_param,omitempty"`
	UploadFullURL    string `json:"upload_full_url,omitempty"`
}

const (
	MessageTypeNone = 0
	MessageTypeUser = 1
	MessageTypeBot  = 2
)

const (
	MessageItemTypeNone  = 0
	MessageItemTypeText  = 1
	MessageItemTypeImage = 2
	MessageItemTypeVoice = 3
	MessageItemTypeFile  = 4
	MessageItemTypeVideo = 5
)

const (
	MessageStateNew        = 0
	MessageStateGenerating = 1
	MessageStateFinish     = 2
)

type TextItem struct {
	Text string `json:"text,omitempty"`
}

type CDNMedia struct {
	EncryptQueryParam string `json:"encrypt_query_param,omitempty"`
	AesKey            string `json:"aes_key,omitempty"`
	EncryptType       int    `json:"encrypt_type,omitempty"`
	FullURL           string `json:"full_url,omitempty"`
}

type ImageItem struct {
	Media       *CDNMedia `json:"media,omitempty"`
	ThumbMedia  *CDNMedia `json:"thumb_media,omitempty"`
	Aeskey      string    `json:"aeskey,omitempty"`
	Url         string    `json:"url,omitempty"`
	MidSize     int64     `json:"mid_size,omitempty"`
	ThumbSize   int64     `json:"thumb_size,omitempty"`
	ThumbHeight int       `json:"thumb_height,omitempty"`
	ThumbWidth  int       `json:"thumb_width,omitempty"`
	HDSize      int64     `json:"hd_size,omitempty"`
}

type VoiceItem struct {
	Media         *CDNMedia `json:"media,omitempty"`
	EncodeType    int       `json:"encode_type,omitempty"`
	BitsPerSample int       `json:"bits_per_sample,omitempty"`
	SampleRate    int       `json:"sample_rate,omitempty"`
	Playtime      int       `json:"playtime,omitempty"`
	Text          string    `json:"text,omitempty"`
}

type FileItem struct {
	Media    *CDNMedia `json:"media,omitempty"`
	FileName string    `json:"file_name,omitempty"`
	MD5      string    `json:"md5,omitempty"`
	Len      string    `json:"len,omitempty"`
}

type VideoItem struct {
	Media       *CDNMedia `json:"media,omitempty"`
	VideoSize   int64     `json:"video_size,omitempty"`
	PlayLength  int       `json:"play_length,omitempty"`
	VideoMD5    string    `json:"video_md5,omitempty"`
	ThumbMedia  *CDNMedia `json:"thumb_media,omitempty"`
	ThumbSize   int64     `json:"thumb_size,omitempty"`
	ThumbHeight int       `json:"thumb_height,omitempty"`
	ThumbWidth  int       `json:"thumb_width,omitempty"`
}

type RefMessage struct {
	MessageItem *MessageItem `json:"message_item,omitempty"`
	Title       string       `json:"title,omitempty"`
}

type MessageItem struct {
	Type         int         `json:"type,omitempty"`
	CreateTimeMs int64       `json:"create_time_ms,omitempty"`
	UpdateTimeMs int64       `json:"update_time_ms,omitempty"`
	IsCompleted  bool        `json:"is_completed,omitempty"`
	MsgID        string      `json:"msg_id,omitempty"`
	RefMsg       *RefMessage `json:"ref_msg,omitempty"`
	TextItem     *TextItem   `json:"text_item,omitempty"`
	ImageItem    *ImageItem  `json:"image_item,omitempty"`
	VoiceItem    *VoiceItem  `json:"voice_item,omitempty"`
	FileItem     *FileItem   `json:"file_item,omitempty"`
	VideoItem    *VideoItem  `json:"video_item,omitempty"`
}

type WeixinMessage struct {
	Seq          int           `json:"seq,omitempty"`
	MessageID    int64         `json:"message_id,omitempty"`
	FromUserID   string        `json:"from_user_id,omitempty"`
	ToUserID     string        `json:"to_user_id,omitempty"`
	ClientID     string        `json:"client_id,omitempty"`
	CreateTimeMs int64         `json:"create_time_ms,omitempty"`
	UpdateTimeMs int64         `json:"update_time_ms,omitempty"`
	DeleteTimeMs int64         `json:"delete_time_ms,omitempty"`
	SessionID    string        `json:"session_id,omitempty"`
	GroupID      string        `json:"group_id,omitempty"`
	MessageType  int           `json:"message_type,omitempty"`
	MessageState int           `json:"message_state,omitempty"`
	ItemList     []MessageItem `json:"item_list,omitempty"`
	ContextToken string        `json:"context_token,omitempty"`
}

type GetUpdatesReq struct {
	SyncBuf       string   `json:"sync_buf,omitempty"`
	GetUpdatesBuf string   `json:"get_updates_buf,omitempty"`
	BaseInfo      BaseInfo `json:"base_info,omitempty"`
}

type GetUpdatesResp struct {
	APIStatus
	Msgs                 []WeixinMessage `json:"msgs,omitempty"`
	SyncBuf              string          `json:"sync_buf,omitempty"`
	GetUpdatesBuf        string          `json:"get_updates_buf,omitempty"`
	LongpollingTimeoutMs int             `json:"longpolling_timeout_ms,omitempty"`
}

type SendMessageReq struct {
	Msg      WeixinMessage `json:"msg,omitempty"`
	BaseInfo BaseInfo      `json:"base_info,omitempty"`
}

type SendMessageResp struct {
	APIStatus
}

type GetConfigReq struct {
	IlinkUserID  string   `json:"ilink_user_id,omitempty"`
	ContextToken string   `json:"context_token,omitempty"`
	BaseInfo     BaseInfo `json:"base_info,omitempty"`
}

type GetConfigResp struct {
	APIStatus
	TypingTicket string `json:"typing_ticket,omitempty"`
}

type GetQRCodeReq struct {
	BotType  string   `json:"bot_type,omitempty"`
	BaseInfo BaseInfo `json:"base_info,omitempty"`
}

type GetQRCodeResp struct {
	APIStatus
	Qrcode           string `json:"qrcode,omitempty"`
	QrcodeImgContent string `json:"qrcode_img_content,omitempty"`
	Baseurl          string `json:"baseurl,omitempty"`
}

type GetQRCodeStatusReq struct {
	Qrcode   string   `json:"qrcode,omitempty"`
	BaseInfo BaseInfo `json:"base_info,omitempty"`
}

type GetQRCodeStatusResp struct {
	APIStatus
	Status       string `json:"status,omitempty"`
	BotToken     string `json:"bot_token,omitempty"`
	IlinkUserID  string `json:"ilink_user_id,omitempty"`
	IlinkBotID   string `json:"ilink_bot_id,omitempty"`
	Baseurl      string `json:"baseurl,omitempty"`
	RedirectHost string `json:"redirect_host,omitempty"`
}

const (
	TypingStatusTyping = 1
	TypingStatusCancel = 2
)

type SendTypingReq struct {
	IlinkUserID  string   `json:"ilink_user_id,omitempty"`
	TypingTicket string   `json:"typing_ticket,omitempty"`
	Status       int      `json:"status,omitempty"`
	BaseInfo     BaseInfo `json:"base_info,omitempty"`
}

type SendTypingResp struct {
	APIStatus
}

type ApiClient struct {
	BaseURL    string
	Token      string
	HttpClient *http.Client
}

func NewApiClient(baseURL, token string, proxy string) (*ApiClient, error) {
	if baseURL == "" {
		baseURL = "https://ilinkai.weixin.qq.com/"
	}

	client := &http.Client{}

	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL %q: %w", proxy, err)
		}

		if defaultTransport, ok := http.DefaultTransport.(*http.Transport); ok {
			transport := defaultTransport.Clone()
			transport.Proxy = http.ProxyURL(proxyURL)
			client.Transport = transport
		} else {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
		}
	}

	return &ApiClient{
		BaseURL:    baseURL,
		Token:      token,
		HttpClient: client,
	}, nil
}

func randomWechatUIN() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	uint32Val := binary.BigEndian.Uint32(b[:])
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", uint32Val)))
}

func (c *ApiClient) post(ctx context.Context, endpoint string, body any, responseObj any) error {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return err
	}
	u.Path = path.Join(u.Path, endpoint)

	jsonData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header["iLink-App-Id"] = []string{weixinIlinkAppID}
	req.Header["iLink-App-ClientVersion"] = []string{strconv.Itoa(weixinClientVersion)}
	if endpoint != "ilink/bot/get_bot_qrcode" && endpoint != "ilink/bot/get_qrcode_status" {
		req.Header["AuthorizationType"] = []string{"ilink_bot_token"}
		req.Header["X-WECHAT-UIN"] = []string{randomWechatUIN()}
		if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http POST %s failed: %w", endpoint, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http %d %s: %s", resp.StatusCode, resp.Status, string(respBody))
	}

	if responseObj != nil {
		if err := json.Unmarshal(respBody, responseObj); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w, body: %s", err, string(respBody))
		}
	}

	return nil
}

func (c *ApiClient) getQR(ctx context.Context, endpoint string, query map[string]string, respObj any) error {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return err
	}
	u.Path = path.Join(u.Path, endpoint)
	q := u.Query()
	for key, value := range query {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return err
	}
	req.Header["iLink-App-Id"] = []string{weixinIlinkAppID}
	req.Header["iLink-App-ClientVersion"] = []string{strconv.Itoa(weixinClientVersion)}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s failed: %d %s", endpoint, resp.StatusCode, string(respBody))
	}
	if err := json.Unmarshal(respBody, respObj); err != nil {
		return err
	}

	return nil
}

func (c *ApiClient) GetUpdates(ctx context.Context, req GetUpdatesReq) (*GetUpdatesResp, error) {
	req.BaseInfo = BaseInfo{ChannelVersion: weixinChannelVersion}
	var resp GetUpdatesResp
	err := c.post(ctx, "ilink/bot/getupdates", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *ApiClient) SendMessage(ctx context.Context, req SendMessageReq) (*SendMessageResp, error) {
	req.BaseInfo = BaseInfo{ChannelVersion: weixinChannelVersion}
	var resp SendMessageResp
	if err := c.post(ctx, "ilink/bot/sendmessage", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *ApiClient) GetUploadUrl(ctx context.Context, req GetUploadUrlReq) (*GetUploadUrlResp, error) {
	req.BaseInfo = BaseInfo{ChannelVersion: weixinChannelVersion}
	var resp GetUploadUrlResp
	err := c.post(ctx, "ilink/bot/getuploadurl", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *ApiClient) GetConfig(ctx context.Context, req GetConfigReq) (*GetConfigResp, error) {
	req.BaseInfo = BaseInfo{ChannelVersion: weixinChannelVersion}
	var resp GetConfigResp
	if err := c.post(ctx, "ilink/bot/getconfig", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *ApiClient) SendTyping(ctx context.Context, req SendTypingReq) (*SendTypingResp, error) {
	req.BaseInfo = BaseInfo{ChannelVersion: weixinChannelVersion}
	var resp SendTypingResp
	if err := c.post(ctx, "ilink/bot/sendtyping", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *ApiClient) GetQRCode(ctx context.Context, botType string) (*GetQRCodeResp, error) {
	if botType == "" {
		botType = "3"
	}
	var resp GetQRCodeResp
	if err := c.getQR(ctx, "ilink/bot/get_bot_qrcode", map[string]string{
		"bot_type": botType,
	}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *ApiClient) GetQRCodeStatus(ctx context.Context, qrcode string) (*GetQRCodeStatusResp, error) {
	var resp GetQRCodeStatusResp
	if err := c.getQR(ctx, "ilink/bot/get_qrcode_status", map[string]string{
		"qrcode": qrcode,
	}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type WeixinAdapter struct {
	*BaseAdapter
	api               *ApiClient
	ctx               context.Context
	cancel            context.CancelFunc
	contextTokens     sync.Map
	typingMu          sync.Mutex
	typingCache       map[string]typingTicketCacheEntry
	pauseMu           sync.Mutex
	pauseUntil        time.Time
	syncBufPath       string
	contextTokensPath string
	msgChan           chan *Message
	stopOnce          sync.Once
}

type typingTicketCacheEntry struct {
	ticket      string
	nextFetchAt time.Time
	retryDelay  time.Duration
}

type syncCursorFile struct {
	GetUpdatesBuf string `json:"get_updates_buf"`
}

type contextTokensFile struct {
	Tokens map[string]string `json:"tokens"`
}

func NewWeixinAdapter(channel *Channel) (Adapter, error) {
	if channel.Config.Weixin == nil {
		return nil, fmt.Errorf("weixin config is nil")
	}

	cfg := channel.Config.Weixin

	api, err := NewApiClient(cfg.BaseURL, cfg.Token, cfg.Proxy)
	if err != nil {
		return nil, fmt.Errorf("weixin: failed to create API client: %w", err)
	}

	base := NewBaseAdapter(channel)

	syncBufPath, contextTokensPath := buildWeixinPaths(channel)

	return &WeixinAdapter{
		BaseAdapter:       base,
		api:               api,
		typingCache:       make(map[string]typingTicketCacheEntry),
		syncBufPath:       syncBufPath,
		contextTokensPath: contextTokensPath,
		msgChan:           make(chan *Message, 100),
	}, nil
}

func buildWeixinPaths(channel *Channel) (string, string) {
	owner := channel.Owner
	if owner == "" {
		owner = "global"
	}

	baseDir, err := os.UserHomeDir()
	if err != nil {
		baseDir = "."
	}

	var dataDir string
	if owner == "global" {
		dataDir = filepath.Join(baseDir, ".fangclaw-go", "channels", "weixin")
	} else {
		dataDir = filepath.Join(baseDir, ".fangclaw-go", "users", owner, "channels", "weixin")
	}

	syncPath := filepath.Join(dataDir, "sync_buf.json")
	tokensPath := filepath.Join(dataDir, "context_tokens.json")

	return syncPath, tokensPath
}

func (a *WeixinAdapter) Connect() error {
	return nil
}

func (a *WeixinAdapter) Disconnect() error {
	return a.Stop()
}

func (a *WeixinAdapter) Start() error {
	a.healthMonitor.Start()
	a.State = ChannelStateConnected
	a.Channel.State = ChannelStateConnected
	a.Channel.UpdatedAt = time.Now()
	a.ctx, a.cancel = context.WithCancel(context.Background())

	a.restoreContextTokens()
	go a.pollLoop()

	return nil
}

func (a *WeixinAdapter) Stop() error {
	a.stopOnce.Do(func() {
		a.healthMonitor.Stop()
		a.State = ChannelStateDisconnected
		a.Channel.State = ChannelStateDisconnected
		a.Channel.UpdatedAt = time.Now()
		if a.cancel != nil {
			a.cancel()
		}
		close(a.msgChan)
	})
	return nil
}

func (a *WeixinAdapter) IsRunning() bool {
	return a.State == ChannelStateConnected
}

func (a *WeixinAdapter) Send(msg *Message) error {
	if !a.IsRunning() {
		return fmt.Errorf("channel not running")
	}
	if err := a.ensureSessionActive(); err != nil {
		return err
	}

	if msg.Content == "" {
		return nil
	}

	toUserID := msg.Recipient

	contextToken := ""
	if ct, ok := a.contextTokens.Load(toUserID); ok {
		contextToken, _ = ct.(string)
	}

	if contextToken == "" {
		return fmt.Errorf("weixin send: missing context token for chat %s", toUserID)
	}

	if err := a.sendTextMessage(a.ctx, toUserID, contextToken, msg.Content); err != nil {
		if a.remainingPause() > 0 {
			return fmt.Errorf("weixin send failed")
		}
		return err
	}

	a.healthMonitor.RecordMessageSent()
	return nil
}

func (a *WeixinAdapter) Receive(ctx context.Context) (<-chan *Message, error) {
	return a.msgChan, nil
}

func (a *WeixinAdapter) pollLoop() {
	const (
		defaultPollTimeoutMs = 35_000
		retryDelay           = 2 * time.Second
		backoffDelay         = 30 * time.Second
		maxConsecutiveFails  = 3
	)

	consecutiveFails := 0
	getUpdatesBuf, err := loadGetUpdatesBuf(a.syncBufPath)
	if err != nil {
		getUpdatesBuf = ""
	}

	nextTimeoutMs := defaultPollTimeoutMs

	for {
		select {
		case <-a.ctx.Done():
			return
		default:
		}

		if err := a.waitWhileSessionPaused(a.ctx); err != nil {
			if a.ctx.Err() != nil {
				return
			}
			continue
		}

		pollCtx, pollCancel := context.WithTimeout(a.ctx, time.Duration(nextTimeoutMs+5000)*time.Millisecond)

		resp, err := a.api.GetUpdates(pollCtx, GetUpdatesReq{
			GetUpdatesBuf: getUpdatesBuf,
		})
		pollCancel()

		if err != nil {
			if a.ctx.Err() != nil {
				return
			}

			consecutiveFails++
			if consecutiveFails >= maxConsecutiveFails {
				consecutiveFails = 0
				select {
				case <-a.ctx.Done():
					return
				case <-time.After(backoffDelay):
				}
			} else {
				select {
				case <-a.ctx.Done():
					return
				case <-time.After(retryDelay):
				}
			}
			continue
		}

		if isSessionExpiredStatus(resp.Ret, resp.Errcode) {
			remaining := a.pauseSession("getupdates", resp.Ret, resp.Errcode, resp.Errmsg)
			select {
			case <-a.ctx.Done():
				return
			case <-time.After(remaining):
			}
			continue
		}

		if resp.Errcode != 0 || resp.Ret != 0 {
			consecutiveFails++
			select {
			case <-a.ctx.Done():
				return
			case <-time.After(retryDelay):
			}
			continue
		}

		consecutiveFails = 0

		if resp.LongpollingTimeoutMs > 0 {
			nextTimeoutMs = resp.LongpollingTimeoutMs
		}

		if resp.GetUpdatesBuf != "" {
			getUpdatesBuf = resp.GetUpdatesBuf
			_ = saveGetUpdatesBuf(a.syncBufPath, getUpdatesBuf)
		}

		for _, msg := range resp.Msgs {
			a.handleInboundMessage(msg)
		}
	}
}

func (a *WeixinAdapter) handleInboundMessage(msg WeixinMessage) {
	fromUserID := msg.FromUserID
	if fromUserID == "" {
		return
	}

	messageID := msg.ClientID
	if messageID == "" {
		messageID = uuid.New().String()
	}

	var parts []string
	for _, item := range msg.ItemList {
		switch item.Type {
		case MessageItemTypeText:
			if item.TextItem != nil && item.TextItem.Text != "" {
				parts = append(parts, item.TextItem.Text)
			}
		case MessageItemTypeVoice:
			if item.VoiceItem != nil && item.VoiceItem.Text != "" {
				parts = append(parts, item.VoiceItem.Text)
			} else {
				parts = append(parts, "[audio]")
			}
		case MessageItemTypeImage:
			parts = append(parts, "[image]")
		case MessageItemTypeFile:
			if item.FileItem != nil && item.FileItem.FileName != "" {
				parts = append(parts, fmt.Sprintf("[file: %s]", item.FileItem.FileName))
			} else {
				parts = append(parts, "[file]")
			}
		case MessageItemTypeVideo:
			parts = append(parts, "[video]")
		}
	}

	content := strings.Join(parts, "\n")
	if content == "" {
		return
	}

	metadata := map[string]interface{}{
		"from_user_id":  fromUserID,
		"context_token": msg.ContextToken,
		"session_id":    msg.SessionID,
	}

	if msg.ContextToken != "" {
		a.contextTokens.Store(fromUserID, msg.ContextToken)
		a.persistContextTokens()
	}

	channelMsg := &Message{
		ID:        messageID,
		ChannelID: a.Channel.ID,
		Content:   content,
		Sender:    fromUserID,
		Recipient: fromUserID,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}

	a.msgChan <- channelMsg
	a.healthMonitor.RecordMessageReceived()
}

func (a *WeixinAdapter) sendTextMessage(ctx context.Context, toUserID, contextToken, text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return a.sendMessageItem(ctx, toUserID, contextToken, MessageItem{
		Type: MessageItemTypeText,
		TextItem: &TextItem{
			Text: text,
		},
	})
}

func (a *WeixinAdapter) sendMessageItem(ctx context.Context, toUserID, contextToken string, item MessageItem) error {
	resp, err := a.api.SendMessage(ctx, SendMessageReq{
		Msg: WeixinMessage{
			ToUserID:     toUserID,
			ClientID:     "fangclaw-" + uuid.New().String(),
			MessageType:  MessageTypeBot,
			MessageState: MessageStateFinish,
			ItemList:     []MessageItem{item},
			ContextToken: contextToken,
		},
	})
	if err != nil {
		return err
	}
	if resp == nil {
		return fmt.Errorf("sendmessage returned nil response")
	}
	if resp.Ret != 0 || resp.Errcode != 0 {
		if isSessionExpiredStatus(resp.Ret, resp.Errcode) {
			a.pauseSession("sendmessage", resp.Ret, resp.Errcode, resp.Errmsg)
		}
		return fmt.Errorf("sendmessage failed: ret=%d errcode=%d errmsg=%s", resp.Ret, resp.Errcode, resp.Errmsg)
	}
	return nil
}

func (a *WeixinAdapter) restoreContextTokens() {
	tokens, err := loadContextTokens(a.contextTokensPath)
	if err != nil {
		return
	}
	if len(tokens) == 0 {
		return
	}
	for userID, token := range tokens {
		a.contextTokens.Store(userID, token)
	}
}

func (a *WeixinAdapter) persistContextTokens() {
	tokens := make(map[string]string)
	a.contextTokens.Range(func(k, v any) bool {
		if userID, ok := k.(string); ok {
			if token, ok := v.(string); ok {
				tokens[userID] = token
			}
		}
		return true
	})
	_ = saveContextTokens(a.contextTokensPath, tokens)
}

func loadGetUpdatesBuf(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	var decoded syncCursorFile
	if err := json.Unmarshal(data, &decoded); err != nil {
		return "", err
	}

	return decoded.GetUpdatesBuf, nil
}

func saveGetUpdatesBuf(path, cursor string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(syncCursorFile{GetUpdatesBuf: cursor})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func loadContextTokens(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var decoded contextTokensFile
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}
	return decoded.Tokens, nil
}

func saveContextTokens(path string, tokens map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(contextTokensFile{Tokens: tokens})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func isSessionExpiredStatus(ret, errcode int) bool {
	return ret == weixinSessionExpiredCode || errcode == weixinSessionExpiredCode
}

func (a *WeixinAdapter) pauseSession(operation string, ret, errcode int, errmsg string) time.Duration {
	a.pauseMu.Lock()
	defer a.pauseMu.Unlock()

	until := time.Now().Add(weixinSessionPauseDuration)
	if until.After(a.pauseUntil) {
		a.pauseUntil = until
	}

	return time.Until(a.pauseUntil)
}

func (a *WeixinAdapter) remainingPause() time.Duration {
	a.pauseMu.Lock()
	defer a.pauseMu.Unlock()

	if a.pauseUntil.IsZero() {
		return 0
	}
	remaining := time.Until(a.pauseUntil)
	if remaining <= 0 {
		a.pauseUntil = time.Time{}
		return 0
	}
	return remaining
}

func (a *WeixinAdapter) waitWhileSessionPaused(ctx context.Context) error {
	remaining := a.remainingPause()
	if remaining <= 0 {
		return nil
	}

	timer := time.NewTimer(remaining)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (a *WeixinAdapter) ensureSessionActive() error {
	remaining := a.remainingPause()
	if remaining <= 0 {
		return nil
	}
	return fmt.Errorf("weixin session paused (%d min remaining)", int((remaining+time.Minute-1)/time.Minute))
}

func (a *WeixinAdapter) getTypingTicket(ctx context.Context, userID string) (string, error) {
	now := time.Now()

	a.typingMu.Lock()
	entry, ok := a.typingCache[userID]
	if ok && now.Before(entry.nextFetchAt) {
		ticket := entry.ticket
		a.typingMu.Unlock()
		return ticket, nil
	}
	cachedTicket := entry.ticket
	retryDelay := entry.retryDelay
	a.typingMu.Unlock()

	contextToken := ""
	if v, ok := a.contextTokens.Load(userID); ok {
		contextToken, _ = v.(string)
	}

	resp, err := a.api.GetConfig(ctx, GetConfigReq{
		IlinkUserID:  userID,
		ContextToken: contextToken,
	})
	if err == nil && resp != nil && resp.Ret == 0 && resp.Errcode == 0 {
		ticket := strings.TrimSpace(resp.TypingTicket)
		a.typingMu.Lock()
		a.typingCache[userID] = typingTicketCacheEntry{
			ticket:      ticket,
			nextFetchAt: now.Add(weixinConfigCacheTTL),
			retryDelay:  weixinConfigRetryInitial,
		}
		a.typingMu.Unlock()
		return ticket, nil
	}

	if resp != nil && isSessionExpiredStatus(resp.Ret, resp.Errcode) {
		a.pauseSession("getconfig", resp.Ret, resp.Errcode, resp.Errmsg)
	}

	if retryDelay <= 0 {
		retryDelay = weixinConfigRetryInitial
	} else {
		retryDelay *= 2
		if retryDelay > weixinConfigRetryMax {
			retryDelay = weixinConfigRetryMax
		}
	}

	a.typingMu.Lock()
	a.typingCache[userID] = typingTicketCacheEntry{
		ticket:      cachedTicket,
		nextFetchAt: now.Add(retryDelay),
		retryDelay:  retryDelay,
	}
	a.typingMu.Unlock()

	return cachedTicket, nil
}
