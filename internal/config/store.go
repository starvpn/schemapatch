package config

import (
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Store 配置存储
type Store struct {
	configPath string
	config     *AppConfig
	mu         sync.RWMutex
}

// NewStore 创建配置存储
func NewStore() (*Store, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}

	// 确保配置目录存在
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}

	configPath := filepath.Join(configDir, "config.yaml")

	store := &Store{
		configPath: configPath,
	}

	// 加载配置
	if err := store.Load(); err != nil {
		// 如果配置不存在，创建默认配置
		if os.IsNotExist(err) {
			store.config = DefaultAppConfig()
			if err := store.Save(); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return store, nil
}

// getConfigDir 获取配置目录
func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".schemapatch"), nil
}

// Load 加载配置
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return err
	}

	var config AppConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}

	s.config = &config
	return nil
}

// Save 保存配置
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := yaml.Marshal(s.config)
	if err != nil {
		return err
	}

	return os.WriteFile(s.configPath, data, 0644)
}

// GetConfig 获取配置
func (s *Store) GetConfig() *AppConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// SetConfig 设置配置
func (s *Store) SetConfig(config *AppConfig) error {
	s.mu.Lock()
	s.config = config
	s.mu.Unlock()
	return s.Save()
}

// GetProject 获取项目
func (s *Store) GetProject(projectID string) *Project {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.config.Projects {
		if s.config.Projects[i].ID == projectID {
			return &s.config.Projects[i]
		}
	}
	return nil
}

// AddProject 添加项目
func (s *Store) AddProject(project Project) error {
	s.mu.Lock()
	s.config.Projects = append(s.config.Projects, project)
	s.mu.Unlock()
	return s.Save()
}

// UpdateProject 更新项目
func (s *Store) UpdateProject(project Project) error {
	s.mu.Lock()
	for i := range s.config.Projects {
		if s.config.Projects[i].ID == project.ID {
			s.config.Projects[i] = project
			break
		}
	}
	s.mu.Unlock()
	return s.Save()
}

// DeleteProject 删除项目
func (s *Store) DeleteProject(projectID string) error {
	s.mu.Lock()
	for i := range s.config.Projects {
		if s.config.Projects[i].ID == projectID {
			s.config.Projects = append(s.config.Projects[:i], s.config.Projects[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
	return s.Save()
}

// GetActiveProject 获取当前活动项目
func (s *Store) GetActiveProject() *Project {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.config.ActiveProject == "" && len(s.config.Projects) > 0 {
		return &s.config.Projects[0]
	}

	for i := range s.config.Projects {
		if s.config.Projects[i].ID == s.config.ActiveProject {
			return &s.config.Projects[i]
		}
	}
	return nil
}

// SetActiveProject 设置当前活动项目
func (s *Store) SetActiveProject(projectID string) error {
	s.mu.Lock()
	s.config.ActiveProject = projectID
	s.mu.Unlock()
	return s.Save()
}

// ExportProject 导出项目配置
func (s *Store) ExportProject(projectID, filePath string) error {
	project := s.GetProject(projectID)
	if project == nil {
		return os.ErrNotExist
	}

	data, err := yaml.Marshal(project)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

// ImportProject 导入项目配置
func (s *Store) ImportProject(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var project Project
	if err := yaml.Unmarshal(data, &project); err != nil {
		return err
	}

	// 生成新的ID避免冲突
	project.ID = generateID()

	return s.AddProject(project)
}
