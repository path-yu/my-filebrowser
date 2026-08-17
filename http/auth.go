package fbhttp

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/golang-jwt/jwt/v5/request"

	fbAuth "github.com/filebrowser/filebrowser/v2/auth"
	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/users"
)

const (
	// DefaultTokenExpirationTime：默认用户会话时长 = 30 天（720 小时）。
	// 对于"只要每月至少访问一次就一直在线"的持久化登录体感，配合下方
	// expiresSoon 窗口（剩余 < 总时长 1/10，即 ~3 天）会自动在到期前静默续新。
	DefaultTokenExpirationTime = time.Hour * 24 * 30

	maxAuthBodySize = 1 << 20 // 1 MiB
)

// renewWindow 根据总过期时长计算"提前续期"窗口：
//   - 下限 1 小时（避免总时长只有几小时时窗口过小，用户偶尔打开一次就错过）
//   - 上限总时长 / 10（30 天默认 → 3 天）
// 这样保证长周期 token 在每月访问一次的节奏下仍能平滑续期，表现为"持久登录不过期"。
func renewWindow(totalDuration time.Duration) time.Duration {
	const minWindow = time.Hour
	window := totalDuration / 10
	if window < minWindow {
		return minWindow
	}
	return window
}

type userInfo struct {
	ID                    uint              `json:"id"`
	Locale                string            `json:"locale"`
	ViewMode              users.ViewMode    `json:"viewMode"`
	SingleClick           bool              `json:"singleClick"`
	RedirectAfterCopyMove bool              `json:"redirectAfterCopyMove"`
	Perm                  users.Permissions `json:"perm"`
	Commands              []string          `json:"commands"`
	LockPassword          bool              `json:"lockPassword"`
	HideDotfiles          bool              `json:"hideDotfiles"`
	DateFormat            bool              `json:"dateFormat"`
	Username              string            `json:"username"`
	AceEditorTheme        string            `json:"aceEditorTheme"`
}

type authToken struct {
	User userInfo `json:"user"`
	jwt.RegisteredClaims
}

type extractor []string

func (e extractor) ExtractToken(r *http.Request) (string, error) {
	token, _ := request.HeaderExtractor{"X-Auth"}.ExtractToken(r)

	// Checks if the token isn't empty and if it contains two dots.
	// The former prevents incompatibility with URLs that previously
	// used basic auth.
	if token != "" && strings.Count(token, ".") == 2 {
		return token, nil
	}

	if r.Method == http.MethodGet {
		cookie, _ := r.Cookie("auth")
		if cookie != nil && strings.Count(cookie.Value, ".") == 2 {
			return cookie.Value, nil
		}
	}

	return "", request.ErrNoTokenInRequest
}

func renewableErr(err error, d *data) bool {
	if d.settings.AuthMethod != fbAuth.MethodProxyAuth || err == nil {
		return false
	}

	if d.settings.LogoutPage == settings.DefaultLogoutPage {
		return false
	}

	if !errors.Is(err, jwt.ErrTokenExpired) {
		return false
	}

	return true
}

func withUser(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		keyFunc := func(_ *jwt.Token) (interface{}, error) {
			return d.settings.Key, nil
		}

		var tk authToken
		p := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
		token, err := request.ParseFromRequest(r, &extractor{}, keyFunc, request.WithClaims(&tk), request.WithParser(p))
		if (err != nil || !token.Valid) && !renewableErr(err, d) {
			return http.StatusUnauthorized, nil
		}

		// 计算"是否临近过期需要续期"：
	// 1) 根据签发时间和过期时间推回本次 token 的总时长；若缺数据则使用服务端默认值兜底。
	// 2) 总时长 / 10，且不低于 1 小时（短会话场景仍有足够窗口）。
	totalDuration := DefaultTokenExpirationTime
	if tk.ExpiresAt != nil && tk.IssuedAt != nil {
		if d := tk.ExpiresAt.Time.Sub(tk.IssuedAt.Time); d > 0 {
			totalDuration = d
		}
	}
	var expiresSoon bool
	if tk.ExpiresAt != nil {
		expiresSoon = time.Until(tk.ExpiresAt.Time) < renewWindow(totalDuration)
	}
	updated := tk.IssuedAt != nil && tk.IssuedAt.Unix() < d.store.Users.LastUpdate(tk.User.ID)

		if expiresSoon || updated {
			w.Header().Add("X-Renew-Token", "true")
		}

		d.user, err = d.store.Users.Get(d.server.Root, tk.User.ID)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		// 挂载点（多根目录）功能：把 user.Fs 包一层 MountOverlayFs，
		// 这样 /files/<挂载名>/... 会自动路由到各虚拟目录对应的物理路径。
		// ScopedFs 的安全边界对每个挂载点仍然独立生效，不会穿透。
		applyUserFsMounts(d)
		return fn(w, r, d)
	}
}

func withAdmin(fn handleFunc) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Admin {
			return http.StatusForbidden, nil
		}

		return fn(w, r, d)
	})
}

func loginHandler(tokenExpireTime time.Duration) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)
		}

		auther, err := d.store.Auth.Get(d.settings.AuthMethod)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		user, err := auther.Auth(r, d.store.Users, d.settings, d.server)
		switch {
		case errors.Is(err, os.ErrPermission):
			return http.StatusForbidden, nil
		case err != nil:
			return http.StatusInternalServerError, err
		}

		return printToken(w, r, d, user, tokenExpireTime)
	}
}

type signupBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var signupHandler = func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.settings.Signup {
		return http.StatusMethodNotAllowed, nil
	}

	if r.Body == nil {
		return http.StatusBadRequest, nil
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)

	info := &signupBody{}
	err := json.NewDecoder(r.Body).Decode(info)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if info.Password == "" || info.Username == "" {
		return http.StatusBadRequest, nil
	}

	user := &users.User{
		Username: info.Username,
	}

	d.settings.Defaults.Apply(user)

	// Users signed up via the signup handler should never become admins, even
	// if that is the default permission.
	user.Perm.Admin = false

	// Self-registered users should not inherit execution capabilities from
	// default settings, regardless of what the administrator has configured
	// as the default. Execution rights must be explicitly granted by an admin.
	user.Perm.Execute = false
	user.Commands = []string{}

	pwd, err := users.ValidateAndHashPwd(info.Password, d.settings.MinimumPasswordLength)
	if err != nil {
		return http.StatusBadRequest, err
	}

	user.Password = pwd
	if d.settings.CreateUserDir {
		user.Scope = ""
	}

	userHome, err := d.settings.MakeUserDir(user.Username, user.Scope, d.server.Root)
	if err != nil {
		log.Printf("create user: failed to mkdir user home dir: [%s]", userHome)
		return http.StatusInternalServerError, err
	}
	user.Scope = userHome
	log.Printf("new user: %s, home dir: [%s].", user.Username, userHome)

	err = d.store.Users.Save(user)
	if errors.Is(err, fberrors.ErrExist) {
		return http.StatusConflict, err
	} else if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}

func renewHandler(tokenExpireTime time.Duration) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		w.Header().Set("X-Renew-Token", "false")
		return printToken(w, r, d, d.user, tokenExpireTime)
	})
}

func printToken(w http.ResponseWriter, r *http.Request, d *data, user *users.User, tokenExpirationTime time.Duration) (int, error) {
	claims := &authToken{
		User: userInfo{
			ID:                    user.ID,
			Locale:                user.Locale,
			ViewMode:              user.ViewMode,
			SingleClick:           user.SingleClick,
			RedirectAfterCopyMove: user.RedirectAfterCopyMove,
			Perm:                  user.Perm,
			LockPassword:          user.LockPassword,
			Commands:              user.Commands,
			HideDotfiles:          user.HideDotfiles,
			DateFormat:            user.DateFormat,
			Username:              user.Username,
			AceEditorTheme:        user.AceEditorTheme,
		},
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenExpirationTime)),
			Issuer:    "File Browser",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(d.settings.Key)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Write auth cookie so that browser-native requests (<img>, <a>, <link>,
	// direct address bar navigation) can authenticate preview/raw endpoints
	// without relying on the X-Auth header (which only fetch/axios can carry).
	// This avoids the LazyImage fallback of downloading the full image bytes
	// via fetch → Blob → createObjectURL (which is why the DOM used to show
	// blob:http://... URLs instead of direct server paths, and why opening a
	// preview URL in a new browser tab returned 401 Unauthorized).
	cookiePath := "/"
	if d.server.BaseURL != "" && d.server.BaseURL != "/" {
		cookiePath = d.server.BaseURL
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "auth",
		Value:    signed,
		Path:     cookiePath,
		Expires:  claims.ExpiresAt.Time,
		HttpOnly: false, // needs to be readable by X-Renew-Token check in JS? Not strictly. Keep false for devtools visibility.
		Secure:   r != nil && r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte(signed)); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
