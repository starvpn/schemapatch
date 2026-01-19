package config

import (
	"time"
)

// EnvironmentType 环境类型
type EnvironmentType string

const (
	EnvTypeDev     EnvironmentType = "dev"
	EnvTypeStaging EnvironmentType = "staging"
	EnvTypeProd    EnvironmentType = "prod"
)

// Environment 数据库环境配置
type Environment struct {
	ID           string          `yaml:"id" json:"id"`
	Name         string          `yaml:"name" json:"name"`
	Type         EnvironmentType `yaml:"type" json:"type"`
	Host         string          `yaml:"host" json:"host"`
	Port         int             `yaml:"port" json:"port"`
	Username     string          `yaml:"username" json:"username"`
	Password     string          `yaml:"password" json:"password"` // 加密存储
	Database     string          `yaml:"database" json:"database"`
	Charset      string          `yaml:"charset" json:"charset"`
	MySQLVersion string          `yaml:"mysql_version" json:"mysql_version"`
	SSLEnabled   bool            `yaml:"ssl_enabled" json:"ssl_enabled"`
	SSLConfig    *SSLConfig      `yaml:"ssl_config,omitempty" json:"ssl_config,omitempty"`
}

// SSLConfig SSL配置
type SSLConfig struct {
	CAFile   string `yaml:"ca_file" json:"ca_file"`
	CertFile string `yaml:"cert_file" json:"cert_file"`
	KeyFile  string `yaml:"key_file" json:"key_file"`
}

// Project 项目配置
type Project struct {
	ID           string        `yaml:"id" json:"id"`
	Name         string        `yaml:"name" json:"name"`
	Environments []Environment `yaml:"environments" json:"environments"`
	IgnoreRules  IgnoreConfig  `yaml:"ignore_rules" json:"ignore_rules"`
	DockerConfig DockerConfig  `yaml:"docker" json:"docker"`
	CreatedAt    time.Time     `yaml:"created_at" json:"created_at"`
	UpdatedAt    time.Time     `yaml:"updated_at" json:"updated_at"`
}

// IgnoreConfig 忽略规则配置
type IgnoreConfig struct {
	Tables              []string `yaml:"tables" json:"tables"`                               // 忽略的表名 (支持通配符)
	Columns             []string `yaml:"columns" json:"columns"`                             // 忽略的列 (格式: table.column)
	Indexes             []string `yaml:"indexes" json:"indexes"`                             // 忽略的索引
	IgnoreComments      bool     `yaml:"ignore_comments" json:"ignore_comments"`             // 是否忽略注释变更
	IgnoreAutoIncrement bool     `yaml:"ignore_auto_increment" json:"ignore_auto_increment"` // 是否忽略自增值变更
	IgnoreCollation     bool     `yaml:"ignore_collation" json:"ignore_collation"`           // 是否忽略字符集变更
	IgnoreCharset       bool     `yaml:"ignore_charset" json:"ignore_charset"`               // 是否忽略编码变更
}

// DockerConfig Docker验证环境配置
type DockerConfig struct {
	MySQLImage string `yaml:"mysql_image" json:"mysql_image"` // 如 mysql:8.0.35
	Timeout    string `yaml:"timeout" json:"timeout"`         // 启动超时
	Cleanup    bool   `yaml:"cleanup" json:"cleanup"`         // 验证后是否清理容器
	Port       int    `yaml:"port" json:"port"`               // 映射端口，默认随机
}

// AppConfig 应用全局配置
type AppConfig struct {
	Projects       []Project `yaml:"projects" json:"projects"`
	ActiveProject  string    `yaml:"active_project" json:"active_project"`
	Theme          string    `yaml:"theme" json:"theme"`                     // light / dark
	Language       string    `yaml:"language" json:"language"`               // zh-CN / en-US
	EncryptionKey  string    `yaml:"encryption_key" json:"encryption_key"`   // 密码加密密钥
	LastOpenedPath string    `yaml:"last_opened_path" json:"last_opened_path"`
}

// DefaultEnvironment 创建默认环境配置
func DefaultEnvironment(envType EnvironmentType, name string) Environment {
	return Environment{
		ID:           generateID(),
		Name:         name,
		Type:         envType,
		Host:         "localhost",
		Port:         3306,
		Charset:      "utf8mb4",
		MySQLVersion: "8.0",
	}
}

// DefaultProject 创建默认项目配置
func DefaultProject(name string) Project {
	return Project{
		ID:   generateID(),
		Name: name,
		Environments: []Environment{
			DefaultEnvironment(EnvTypeDev, "开发环境"),
			DefaultEnvironment(EnvTypeProd, "生产环境"),
		},
		IgnoreRules: IgnoreConfig{
			IgnoreComments:      true,
			IgnoreAutoIncrement: true,
		},
		DockerConfig: DockerConfig{
			MySQLImage: "mysql:8.0",
			Timeout:    "60s",
			Cleanup:    true,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// DefaultAppConfig 创建默认应用配置
func DefaultAppConfig() *AppConfig {
	return &AppConfig{
		Projects: []Project{},
		Theme:    "dark",
		Language: "zh-CN",
	}
}

// generateID 生成唯一ID
func generateID() string {
	return time.Now().Format("20060102150405") + randomString(6)
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

// GetEnvironment 根据ID获取环境
func (p *Project) GetEnvironment(envID string) *Environment {
	for i := range p.Environments {
		if p.Environments[i].ID == envID {
			return &p.Environments[i]
		}
	}
	return nil
}

// AddEnvironment 添加环境
func (p *Project) AddEnvironment(env Environment) {
	if env.ID == "" {
		env.ID = generateID()
	}
	p.Environments = append(p.Environments, env)
	p.UpdatedAt = time.Now()
}

// RemoveEnvironment 移除环境
func (p *Project) RemoveEnvironment(envID string) bool {
	for i, env := range p.Environments {
		if env.ID == envID {
			p.Environments = append(p.Environments[:i], p.Environments[i+1:]...)
			p.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// DSN 生成MySQL连接字符串
func (e *Environment) DSN() string {
	// user:password@tcp(host:port)/database?charset=utf8mb4&parseTime=True
	dsn := e.Username
	if e.Password != "" {
		dsn += ":" + e.Password
	}
	dsn += "@tcp(" + e.Host + ":" + string(rune(e.Port)) + ")/" + e.Database
	dsn += "?charset=" + e.Charset + "&parseTime=True&loc=Local"

	if e.SSLEnabled && e.SSLConfig != nil {
		dsn += "&tls=custom"
	}

	return dsn
}
