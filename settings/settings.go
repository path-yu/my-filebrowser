package settings

import (
	"crypto/rand"
	"io/fs"
	"log"
	"strings"
	"time"

	"github.com/filebrowser/filebrowser/v2/rules"
)

const DefaultUsersHomeBasePath = "/users"
const DefaultLogoutPage = "/login"
const DefaultMinimumPasswordLength = 12
const DefaultFileMode = 0640
const DefaultDirMode = 0750

// AuthMethod describes an authentication method.
type AuthMethod string

// Settings contain the main settings of the application.
type Settings struct {
	Key                   []byte              `json:"key"`
	Signup                bool                `json:"signup"`
	HideLoginButton       bool                `json:"hideLoginButton"`
	CreateUserDir         bool                `json:"createUserDir"`
	UserHomeBasePath      string              `json:"userHomeBasePath"`
	Defaults              UserDefaults        `json:"defaults"`
	AuthMethod            AuthMethod          `json:"authMethod"`
	LogoutPage            string              `json:"logoutPage"`
	Branding              Branding            `json:"branding"`
	Tus                   Tus                 `json:"tus"`
	Commands              map[string][]string `json:"commands"`
	Shell                 []string            `json:"shell"`
	Rules                 []rules.Rule        `json:"rules"`
	MinimumPasswordLength uint                `json:"minimumPasswordLength"`
	FileMode              fs.FileMode         `json:"fileMode"`
	DirMode               fs.FileMode         `json:"dirMode"`
	HideDotfiles          bool                `json:"hideDotfiles"`
}

// GetRules implements rules.Provider.
func (s *Settings) GetRules() []rules.Rule {
	return s.Rules
}

// Server specific settings.
type Server struct {
	Root                  string            `json:"root"`
	// Mounts 定义在根目录（Root）下额外虚拟挂载的目录：
	//   key   = 挂载点虚拟名（纯文件名，不带 /），会显示在 /files/ 首页的目录列表里
	//   value = 实际物理路径（本地绝对路径 或 UNC 共享路径）
	// 用来实现"同时指定多个目录为根目录"的需求，且不会把父目录下其他内容暴露出来。
	// 例：Mounts["发申江图纸群PDF图纸"] = `\\Sjwh\技术部\发申江图纸群PDF图纸`
	//     → 访问 /files/发申江图纸群PDF图纸/xxx.pdf 实际走第二套 UNC。
	Mounts                map[string]string `json:"mounts,omitempty"`
	BaseURL               string            `json:"baseURL"`
	Socket                string            `json:"socket"`
	TLSKey                string            `json:"tlsKey"`
	TLSCert               string            `json:"tlsCert"`
	Port                  string            `json:"port"`
	Address               string            `json:"address"`
	Log                   string            `json:"log"`
	EnableThumbnails      bool              `json:"enableThumbnails"`
	ResizePreview         bool              `json:"resizePreview"`
	EnableExec            bool              `json:"enableExec"`
	TypeDetectionByHeader bool              `json:"typeDetectionByHeader"`
	ImageResolutionCal    bool              `json:"imageResolutionCalculation"`
	AuthHook              string              `json:"authHook"`
	TokenExpirationTime   string            `json:"tokenExpirationTime"`
}

// Clean cleans any variables that might need cleaning.
func (s *Server) Clean() {
	s.BaseURL = strings.TrimSuffix(s.BaseURL, "/")
}

func (s *Server) GetTokenExpirationTime(fallback time.Duration) time.Duration {
	if s.TokenExpirationTime == "" {
		return fallback
	}

	duration, err := time.ParseDuration(s.TokenExpirationTime)
	if err != nil {
		log.Printf("[WARN] Failed to parse tokenExpirationTime: %v", err)
		return fallback
	}
	return duration
}

// GenerateKey generates a key of 512 bits.
func GenerateKey() ([]byte, error) {
	b := make([]byte, 64)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}
