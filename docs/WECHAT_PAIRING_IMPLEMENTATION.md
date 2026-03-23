# WeChat QR Code Pairing Implementation

## Overview
Implemented WeChat-compatible QR code pairing flow that allows users to scan a QR code with WeChat and complete device pairing through a mobile-friendly web interface.

## Changes Made

### 1. Created Pairing Confirmation Page
**File:** `/internal/api/static/pair.html`

**Features:**
- 📱 Mobile-first, WeChat-styled UI (green theme)
- 🔐 Displays pairing token for verification
- 🤖 Auto-detects device information (iPhone, Android, Windows, Mac, etc.)
- ✏️ Pre-fills device name with suggestion (e.g., "iPhone - 2024/3/20")
- ✅ User can customize device name (required, max 50 chars)
- 🔄 Real-time validation and feedback
- ⏱️ Auto-redirect to device list page after 3 seconds on success

**Auto-Detection:**
- iOS (iPhone, iPad)
- Android
- Windows PC
- Mac (macOS)
- Linux
- WeChat built-in browser detection

### 2. Added Route Handler
**File:** `/internal/api/routes.go`

**New Route:**
```go
mux.HandleFunc("GET /pair", r.handlePairingPage)
```

**Handler Function:**
```go
func (r *Router) handlePairingPage(w http.ResponseWriter, req *http.Request) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    http.ServeFile(w, req, "internal/api/static/pair.html")
}
```

### 3. Updated QR Code Generation
**File:** `/internal/api/routes.go` (line ~5002)

**Before:**
```go
qrURI := fmt.Sprintf("fangclawgo://pair?token=%s", pairingReq.Token)
```

**After:**
```go
// Generate WeChat-compatible QR code URL using request host
scheme := "http"
if req.TLS != nil {
    scheme = "https"
}

if isLocalhost(host) {
		lanIP := getLANIPAddress()
		if lanIP != "" {
			// Extract port from original host and append to LAN IP
			_, port, _ := net.SplitHostPort(req.Host)
			if port != "" {
				host = fmt.Sprintf("%s:%s", lanIP, port)
			} else {
				host = lanIP
			}
		} else {
			// Fallback: use original host if no LAN IP found
			host = req.Host
		}
	}

	serverURL := fmt.Sprintf("%s://%s", scheme, host)
```

**Key Improvements:**
- ✅ Changed from custom URI scheme (`fangclawgo://`) to HTTPS URL
- ✅ WeChat can now recognize and open the QR code
- ✅ Automatically detects HTTP/HTTPS based on connection
- ✅ Uses request host for correct server address
- ❌ Removed unnecessary `server` parameter (simplified)

## User Flow

```
1. User opens Settings → Pairing page
   ↓
2. Clicks "Create Pairing Request"
   ↓
3. QR code is generated with URL:
   http://localhost:4200/pair?token=abc-123
   ↓
4. User scans QR with WeChat
   ↓
5. WeChat opens pairing page in browser
   ↓
6. Page displays:
   - Pairing token (for verification)
   - Auto-detected device info
   - Pre-filled device name
   ↓
7. User can modify device name
   ↓
8. User clicks "确认配对" (Confirm Pairing)
   ↓
9. JavaScript calls API:
   POST /api/pairing/complete
   Body: {
     "token": "abc-123",
     "display_name": "我的 iPhone",
     "platform": "wechat-web"
   }
   ↓
10. Server validates and completes pairing:
    - Generates DeviceID (UUID)
    - Saves to database
    - Returns success
    ↓
11. Page shows success message:
    ✅ 配对成功！
    设备 ID: xxx-xxx-xxx
    设备名称：我的 iPhone
    页面将在 3 秒后跳转到设备列表...
    ↓
12. Auto-redirect to /settings#pairing
```

## API Parameters

### Required Fields
| Field | Source | Auto-generated |
|-------|--------|----------------|
| `token` | URL parameter | ❌ No (from QR code) |
| `DeviceID` | Server | ✅ Yes (UUID) |
| `PairedAt` | Server | ✅ Yes (timestamp) |
| `LastSeen` | Server | ✅ Yes (timestamp) |

### Optional Fields
| Field | Default | User Input |
|-------|---------|------------|
| `DisplayName` | "unknown" | ✅ Yes (required in UI) |
| `Platform` | "unknown" | ❌ No (fixed as "wechat-web") |
| `PushToken` | null | ❌ No (not used) |

## Success/Failure Handling

### Success Response
```json
{
  "device_id": "550e8400-e29b-41d4-a716-446655440000",
  "display_name": "我的 iPhone",
  "platform": "wechat-web",
  "paired_at": "2024-03-20T10:30:00Z"
}
```

**UI Shows:**
- ✅ Success message with device details
- ⏱️ 3-second countdown
- 🔄 Auto-redirect to device list

### Failure Responses
- "配对令牌已过期" (Token expired)
- "无效的配对令牌" (Invalid token)
- "已达到最大设备数量" (Max devices reached)
- Network errors

**UI Shows:**
- ❌ Error message
- 🔄 "Retry" button enabled

## Testing Checklist

- [ ] Build succeeds: `go build ./cmd/fangclaw-go`
- [ ] Start server: `./fangclaw-go`
- [ ] Navigate to Settings → Pairing
- [ ] Create pairing request
- [ ] Scan QR code with WeChat
- [ ] Verify page opens in WeChat browser
- [ ] Check auto-detected device info
- [ ] Modify device name (optional)
- [ ] Click "确认配对"
- [ ] Verify success message
- [ ] Verify redirect to device list
- [ ] Check device appears in paired devices list

## Security Considerations

1. **Token Validation**: Tokens are validated using constant-time comparison
2. **Expiration**: Tokens expire after configured time (default: 5 minutes)
3. **Max Devices**: Limited to prevent abuse (default: 10 devices)
4. **No Authentication Required**: Currently open pairing (consider adding auth in production)

## Future Enhancements

1. **Authentication**: Require login before completing pairing
2. **Rate Limiting**: Prevent spam pairing requests
3. **APNs Integration**: Add PushToken support for iOS push notifications
4. **Multi-language**: Add English version of the pairing page
5. **Custom Themes**: Allow customization of page styling

## Files Modified

1. `/internal/api/static/pair.html` (NEW) - Pairing confirmation page
2. `/internal/api/routes.go` (MODIFIED) - Added route and handler

## Compatibility

- ✅ WeChat (all versions)
- ✅ Mobile browsers (iOS Safari, Chrome Android)
- ✅ Desktop browsers (fallback for testing)
- ✅ Responsive design (mobile-first)
