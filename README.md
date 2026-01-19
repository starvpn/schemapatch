# SchemaPatch

MySQL数据库Schema对比与升级工具，支持开发/生产环境对比、升级SQL生成、Docker虚拟环境验证。

## 功能特性

- 🔍 **Schema对比**: 自动对比两个MySQL数据库的结构差异
- 📝 **SQL生成**: 基于规则自动生成升级SQL脚本
- 🐳 **Docker验证**: 在隔离的Docker环境中验证升级脚本
- 🖥️ **图形界面**: 基于Fyne的跨平台GUI
- ⚠️ **风险评估**: 自动识别危险操作并给出警告

## 支持的对象类型

- 表 (Tables): 列、索引、外键、表属性
- 视图 (Views)
- 存储过程 (Procedures)
- 函数 (Functions)
- 触发器 (Triggers)

## 安装

### 前置要求

- Go 1.21+
- Docker (用于验证功能)
- MySQL 5.7+ / 8.0+

### 从源码编译

```bash
# 克隆项目
git clone https://github.com/starvpn/schemapatch.git
cd schemapatch

# 安装依赖
go mod download

# 编译
go build -o schemapatch ./cmd/schemapatch

# 运行
./schemapatch
```

## 使用方法

### 1. 配置数据库环境

启动程序后，在左右两侧面板分别配置：
- **开发环境 (Source)**: 包含新功能的数据库
- **生产环境 (Target)**: 需要升级的数据库

### 2. 执行对比

点击 "开始对比" 按钮，程序将：
1. 连接两个数据库
2. 提取Schema信息
3. 分析差异
4. 在差异树中显示结果

### 3. 生成升级脚本

点击 "生成脚本" 按钮，程序将：
1. 根据差异生成SQL语句
2. 按依赖顺序排列
3. 在预览区域显示

### 4. Docker验证 (可选)

点击 "Docker验证" 按钮，程序将：
1. 启动MySQL容器
2. 导入目标环境Schema
3. 执行升级脚本
4. 验证结果

### 5. 导出脚本

验证通过后，点击 "导出脚本" 保存SQL文件。

## 配置文件

配置文件位于 `~/.schemapatch/config.yaml`

```yaml
projects:
  - id: "proj_001"
    name: "MyApp"
    environments:
      - id: "env_dev"
        name: "开发环境"
        type: "dev"
        host: "localhost"
        port: 3306
        username: "root"
        database: "myapp_dev"
        
      - id: "env_prod"
        name: "生产环境"
        type: "prod"
        host: "prod-server"
        port: 3306
        username: "readonly"
        database: "myapp_prod"
        
    ignore_rules:
      tables:
        - "temp_*"
        - "log_*"
      ignore_comments: true
      ignore_auto_increment: true
```

## 风险等级说明

| 图标 | 等级 | 说明 |
|-----|------|------|
| 🟢 | 安全 | 新增操作，通常安全 |
| 🟡 | 警告 | 修改操作，需要关注 |
| 🔴 | 危险 | 删除/收缩操作，可能丢失数据 |

## 项目结构

```
schemapatch/
├── cmd/schemapatch/     # 应用入口
├── internal/
│   ├── config/          # 配置管理
│   ├── extractor/       # Schema提取
│   ├── diff/            # 差异分析
│   ├── sqlgen/          # SQL生成
│   ├── docker/          # Docker验证
│   └── gui/             # Fyne GUI
├── docs/                # 文档
└── configs/             # 配置模板
```

## 开发

### 运行测试

```bash
go test ./...
```

### 本地开发

```bash
# 开发模式运行
go run ./cmd/schemapatch
```

## 注意事项

1. **生产环境连接**: 建议使用只读账号连接生产环境
2. **备份**: 执行升级前务必备份数据库
3. **验证**: 建议在Docker环境验证通过后再执行
4. **危险操作**: 删除表/列等操作需要特别确认

## License

MIT License
