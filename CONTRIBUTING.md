# Contributing to FileCodeBox-Go

感谢你的关注！欢迎提交 Issue 和 Pull Request。

## 开发流程

```bash
# 1. Fork 并克隆
git clone https://github.com/ischenyu/FileCodeBox-Go.git
cd FileCodeBox-Go

# 2. 安装依赖
go mod tidy

# 3. 创建分支
git checkout -b feature/your-feature

# 4. 开发并测试
go vet ./...
go run .

# 5. 提交更改（使用 Conventional Commits）
git commit -m "feat: add new feature"
# 或 fix:, docs:, refactor:, perf:, test:, chore:

# 6. 推送并创建 PR
git push origin feature/your-feature
```

## 代码规范

- 遵循 Go 标准代码风格（`gofmt`）
- 所有公开函数和类型必须有注释
- 新增功能需要对应的单元测试
- `go vet ./...` 必须零错误通过
- 提交信息遵循 [Conventional Commits](https://www.conventionalcommits.org/)

## 项目结构

```
├── main.go                   # 入口
├── internal/                 # 内部包
│   ├── config/               # 运行时配置
│   ├── database/             # SQLite + GORM
│   ├── models/               # 数据模型
│   ├── storage/              # 存储后端接口与实现
│   ├── middleware/            # Gin 中间件
│   ├── handlers/             # HTTP 处理器
│   ├── server/               # 路由注册
│   ├── tasks/                # 后台任务
│   └── utils/                # 工具函数
└── themes/                   # 前端主题（构建时嵌入）
```

## 行为准则

- 尊重所有贡献者
- 建设性讨论，禁止人身攻击
- 新功能请先在 Issue 中讨论后再实现
