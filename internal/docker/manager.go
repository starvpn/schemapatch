package docker

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ContainerConfig 容器配置
type ContainerConfig struct {
	MySQLVersion string        // MySQL版本
	MySQLImage   string        // 镜像名称，如 mysql:8.0.35
	Port         int           // 映射端口，0表示随机
	RootPassword string        // root密码
	Database     string        // 数据库名
	Charset      string        // 字符集
	Collation    string        // 排序规则
	SQLMode      string        // SQL模式
	Timeout      time.Duration // 启动超时
}

// DefaultContainerConfig 默认容器配置
func DefaultContainerConfig() ContainerConfig {
	return ContainerConfig{
		MySQLVersion: "8.0",
		MySQLImage:   "mysql:8.0",
		Port:         0, // 随机端口
		RootPassword: "schemapatch_test_pwd",
		Database:     "test_db",
		Charset:      "utf8mb4",
		Collation:    "utf8mb4_unicode_ci",
		Timeout:      60 * time.Second,
	}
}

// Container 容器信息
type Container struct {
	ID       string
	Name     string
	Port     int
	Host     string
	Status   string
	Config   ContainerConfig
}

// Manager Docker管理器
type Manager struct {
	containers map[string]*Container
}

// NewManager 创建Docker管理器
func NewManager() *Manager {
	return &Manager{
		containers: make(map[string]*Container),
	}
}

// CheckDockerAvailable 检查Docker是否可用
func (m *Manager) CheckDockerAvailable(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Docker不可用: %w\n输出: %s", err, string(output))
	}
	return nil
}

// PullImage 拉取镜像
func (m *Manager) PullImage(ctx context.Context, image string) error {
	zap.S().Infof("正在拉取镜像: %s", image)

	cmd := exec.CommandContext(ctx, "docker", "pull", image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("拉取镜像失败: %w\n输出: %s", err, string(output))
	}

	zap.S().Infof("镜像拉取完成: %s", image)
	return nil
}

// CreateContainer 创建容器
func (m *Manager) CreateContainer(ctx context.Context, config ContainerConfig) (*Container, error) {
	containerName := fmt.Sprintf("schemapatch_test_%d", time.Now().UnixNano())

	// 构建docker run命令
	args := []string{
		"run", "-d",
		"--name", containerName,
		"-e", fmt.Sprintf("MYSQL_ROOT_PASSWORD=%s", config.RootPassword),
		"-e", fmt.Sprintf("MYSQL_DATABASE=%s", config.Database),
	}

	// 端口映射
	if config.Port > 0 {
		args = append(args, "-p", fmt.Sprintf("%d:3306", config.Port))
	} else {
		args = append(args, "-p", "3306") // 随机端口
	}

	// 镜像名称必须在所有 docker run 选项之后
	args = append(args, config.MySQLImage)

	// MySQL启动参数必须在镜像名称之后
	if config.Charset != "" {
		args = append(args, "--character-set-server="+config.Charset)
	}
	if config.Collation != "" {
		args = append(args, "--collation-server="+config.Collation)
	}

	zap.S().Infof("创建容器: docker %s", strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("创建容器失败: %w\n输出: %s", err, string(output))
	}

	containerID := strings.TrimSpace(string(output))

	// 获取映射端口
	port, err := m.getContainerPort(ctx, containerID)
	if err != nil {
		m.RemoveContainer(ctx, containerID)
		return nil, fmt.Errorf("获取容器端口失败: %w", err)
	}

	container := &Container{
		ID:     containerID,
		Name:   containerName,
		Port:   port,
		Host:   "127.0.0.1",
		Status: "created",
		Config: config,
	}

	m.containers[containerID] = container
	zap.S().Infof("容器创建成功: %s, 端口: %d", containerID[:12], port)

	return container, nil
}

// getContainerPort 获取容器映射端口
func (m *Manager) getContainerPort(ctx context.Context, containerID string) (int, error) {
	cmd := exec.CommandContext(ctx, "docker", "port", containerID, "3306")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, err
	}

	// 解析输出，格式: 0.0.0.0:32768
	portStr := strings.TrimSpace(string(output))
	parts := strings.Split(portStr, ":")
	if len(parts) < 2 {
		return 0, fmt.Errorf("无法解析端口: %s", portStr)
	}

	var port int
	fmt.Sscanf(parts[len(parts)-1], "%d", &port)
	return port, nil
}

// WaitForMySQL 等待MySQL就绪
func (m *Manager) WaitForMySQL(ctx context.Context, container *Container) error {
	zap.S().Info("等待MySQL就绪...")

	deadline := time.Now().Add(container.Config.Timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("等待MySQL超时")
			}

			// 尝试连接
			cmd := exec.CommandContext(ctx, "docker", "exec", container.ID,
				"mysqladmin", "ping", "-h", "localhost",
				"-u", "root", fmt.Sprintf("-p%s", container.Config.RootPassword))
			if err := cmd.Run(); err == nil {
				container.Status = "ready"
				zap.S().Info("MySQL已就绪")
				return nil
			}
		}
	}
}

// ExecuteSQL 在容器中执行SQL
func (m *Manager) ExecuteSQL(ctx context.Context, container *Container, sql string) (*ExecutionResult, error) {
	startTime := time.Now()

	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", container.ID,
		"mysql", "-u", "root", fmt.Sprintf("-p%s", container.Config.RootPassword),
		container.Config.Database)

	// 通过stdin传递SQL
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, sql)
	}()

	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	result := &ExecutionResult{
		Success:   err == nil,
		Output:    string(output),
		Duration:  duration,
		Error:     "",
	}

	if err != nil {
		result.Error = err.Error()
		// 尝试提取MySQL错误信息
		if strings.Contains(result.Output, "ERROR") {
			result.Error = result.Output
		}
	}

	return result, nil
}

// ExecuteSQLWithDelimiter 使用自定义分隔符执行SQL（用于存储过程/函数）
func (m *Manager) ExecuteSQLWithDelimiter(ctx context.Context, container *Container, sql, delimiter string) (*ExecutionResult, error) {
	startTime := time.Now()

	// 使用 --delimiter 参数设置自定义分隔符
	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", container.ID,
		"mysql", "-u", "root", fmt.Sprintf("-p%s", container.Config.RootPassword),
		fmt.Sprintf("--delimiter=%s", delimiter),
		container.Config.Database)

	// 通过stdin传递SQL
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, sql)
	}()

	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	result := &ExecutionResult{
		Success:  err == nil,
		Output:   string(output),
		Duration: duration,
		Error:    "",
	}

	if err != nil {
		result.Error = err.Error()
		// 尝试提取MySQL错误信息
		if strings.Contains(result.Output, "ERROR") {
			result.Error = result.Output
		}
	}

	return result, nil
}

// ExecuteSQLStatements 执行多条SQL语句
func (m *Manager) ExecuteSQLStatements(ctx context.Context, container *Container, statements []string, callback func(int, int, string, error)) error {
	total := len(statements)

	for i, stmt := range statements {
		if callback != nil {
			callback(i+1, total, stmt, nil)
		}

		result, err := m.ExecuteSQL(ctx, container, stmt)
		if err != nil {
			if callback != nil {
				callback(i+1, total, stmt, err)
			}
			return fmt.Errorf("执行第 %d 条语句失败: %w", i+1, err)
		}

		if !result.Success {
			err := fmt.Errorf(result.Error)
			if callback != nil {
				callback(i+1, total, stmt, err)
			}
			return fmt.Errorf("执行第 %d 条语句失败: %s", i+1, result.Error)
		}
	}

	return nil
}

// GetContainerLogs 获取容器日志
func (m *Manager) GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error) {
	args := []string{"logs"}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}
	args = append(args, containerID)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// StopContainer 停止容器
func (m *Manager) StopContainer(ctx context.Context, containerID string) error {
	zap.S().Infof("停止容器: %s", containerID[:12])

	cmd := exec.CommandContext(ctx, "docker", "stop", containerID)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("停止容器失败: %w\n输出: %s", err, string(output))
	}

	if container, ok := m.containers[containerID]; ok {
		container.Status = "stopped"
	}

	return nil
}

// RemoveContainer 删除容器
func (m *Manager) RemoveContainer(ctx context.Context, containerID string) error {
	zap.S().Infof("删除容器: %s", containerID[:12])

	// 先尝试停止
	m.StopContainer(ctx, containerID)

	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerID)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("删除容器失败: %w\n输出: %s", err, string(output))
	}

	delete(m.containers, containerID)
	return nil
}

// Cleanup 清理所有容器
func (m *Manager) Cleanup(ctx context.Context) {
	for id := range m.containers {
		m.RemoveContainer(ctx, id)
	}
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	Success  bool          `json:"success"`
	Output   string        `json:"output"`
	Error    string        `json:"error"`
	Duration time.Duration `json:"duration"`
}
