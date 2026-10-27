# go-mall

一个用来练手的微服务电商系统：Go 1.26 + Gin + GORM + MySQL 8，前端是纯 HTML + CSS + 原生 JS，没有构建步骤。

**当前状态：本地可跑通完整流程**（注册登录 → 浏览商品 → 加购 → 下单 → 支付/取消 → 后台管理）。

## 技术栈

| 层 | 选型 | 说明 |
|---|---|---|
| 语言 | Go 1.26 | 单模块，`go-mall` |
| Web | Gin v1.12 | 每个服务一个独立进程 |
| ORM | GORM v1.31 | 迁移用 `go:embed` + 自建 `schema_migrations` 表 |
| 数据库 | MySQL 8 | 单实例多 schema，每个服务一个账号 |
| 鉴权 | JWT (HS256) + bcrypt | 网关验签后把身份写进请求头 |
| 前端 | 原生 JS | hash 路由，`go:embed` 打包进网关 |
| 监控 | Prometheus | 各服务暴露 `/metrics` |

Redis 代码保留但默认关闭（`REDIS_ENABLED=false`），本地跑不需要装。

## 目录结构

```
go-mall/
├── pkg/                     公共库
│   ├── apperr/              业务错误码
│   ├── config/              环境变量加载
│   ├── database/            MySQL / Redis 连接
│   ├── httpx/               服务间 HTTP 调用
│   ├── jwt/                 Token 签发与校验
│   ├── middleware/          中间件（鉴权、日志、CORS、限流）
│   ├── migrate/             SQL 迁移执行器
│   ├── money/               金额类型（int64 存分）
│   ├── response/            统一响应格式
│   └── server/              Gin 引擎装配
├── services/
│   ├── gateway/             8080  API 网关 + 前端静态资源
│   │   ├── internal/proxy/  反向代理与身份注入
│   │   ├── internal/web/    前端文件（dist/）
│   │   └── cmd/server/
│   ├── user/                8081  注册、登录、个人资料、用户管理
│   ├── product/             8082  商品、分类、库存扣减、图片上传
│   ├── cart/                8083  购物车
│   └── order/               8084  订单、支付、取消
├── scripts/
│   ├── tasks.sh             常用命令
│   └── local-db-init.sh     在本机 MySQL 建库建账号
└── docs/architecture.md     架构说明
```

## 快速开始

### 1. 准备数据库

本机装好 MySQL 8，然后建库建账号（会提示输入 root 密码）：

```bash
./scripts/tasks.sh local-db
```

这一步会创建 `mall_user`、`mall_product`、`mall_cart`、`mall_order` 四个库和对应账号。
建表由各服务启动时自动执行，不用手动跑 SQL。

### 2. 配置

项目根目录的 `.env` 已经配好本地开发用的参数，主要看这几项：

```ini
MYSQL_HOST=127.0.0.1
MYSQL_PORT=3306
REDIS_ENABLED=false
```

如果本机 MySQL 端口或密码不同，改这里即可。

### 3. 启动

```bash
./scripts/tasks.sh start-all
```

会先编译 5 个服务，然后放到后台跑，每个服务的输出写到 `logs/<服务名>.log`。
脚本最后会打印一遍健康检查结果，全是 `status: ok` 就说明起来了。

想重启某个服务，直接 `stop-all` 再 `start-all`。已经占着端口的服务会自动跳过，不会启动第二份。

查看实时日志：

```bash
./scripts/tasks.sh log gateway    # 也可以写 user / product / cart / order
```

也可以手动一个一个启动（前台运行，Ctrl+C 停，适合调试）：

```bash
./scripts/tasks.sh run user
```

检查是否都起来了：

```bash
./scripts/tasks.sh health
```

### 4. 打开浏览器

**http://127.0.0.1:8080**

默认管理员账号：`admin` / `admin123456`
普通用户直接在登录页点"注册"。

停止全部服务：

```bash
./scripts/tasks.sh stop-all
```

## 常用命令

```bash
./scripts/tasks.sh start-all     # 编译 + 后台启动 + 自动健康检查
./scripts/tasks.sh health        # 看状态
./scripts/tasks.sh log user      # 看某个服务的实时日志
./scripts/tasks.sh stop-all      # 全停
./scripts/tasks.sh run user      # 单个前台跑，Ctrl+C 停，调试用
```

其他命令：

```bash
./scripts/tasks.sh build         # 只编译，不启动
./scripts/tasks.sh test          # 跑单元测试
./scripts/tasks.sh fmt           # 格式化代码
./scripts/tasks.sh vet           # 静态检查
./scripts/tasks.sh local-db      # 建库建账号
./scripts/tasks.sh clean         # 清理 bin/ 和 logs/
```

## 功能

前台（普通用户）：

- 商品列表、关键词搜索、按分类筛选、分页
- 加入购物车、改数量、删除、清空
- 下单（库存实时校验）、支付、取消订单（取消后库存退回）
- 我的订单列表

后台（管理员）：

- 商品的新建、编辑、上下架、删除
- 商品图片上传（存本地 `uploads/`，2MB 以内）
- 全部订单查询，按状态筛选

## 接口一览

统一前缀 `/api/v1`，统一响应：

```json
{ "code": 0, "message": "success", "data": {}, "request_id": "..." }
```

`code` 非 0 表示失败，前端直接展示 `message`。

| 模块 | 方法与路径 | 说明 |
|---|---|---|
| 认证 | `POST /auth/register` | 注册 |
| | `POST /auth/login` | 登录，返回 JWT |
| 用户 | `GET /users/me` | 当前用户 |
| | `PUT /users/me` | 改资料 |
| | `PUT /users/me/password` | 改密码 |
| | `GET /admin/users` | 用户列表（管理员） |
| 商品 | `GET /products` | 商品列表，支持 `keyword` `category_id` |
| | `GET /products/:id` | 商品详情 |
| | `GET /categories` | 分类列表 |
| | `POST /admin/products` | 新建商品（管理员） |
| | `PUT /admin/products/:id` | 修改商品（管理员） |
| | `PUT /admin/products/:id/status` | 上下架（管理员） |
| | `DELETE /admin/products/:id` | 删除商品（管理员） |
| | `POST /admin/uploads` | 上传图片（管理员） |
| 购物车 | `GET /cart` | 购物车内容 |
| | `POST /cart/items` | 加购 |
| | `PUT /cart/items/:product_id` | 改数量 |
| | `DELETE /cart/items/:product_id` | 删除一项 |
| | `DELETE /cart` | 清空 |
| 订单 | `POST /orders` | 下单 |
| | `GET /orders` | 我的订单 |
| | `GET /orders/:order_no` | 订单详情 |
| | `POST /orders/:order_no/pay` | 支付（模拟） |
| | `POST /orders/:order_no/cancel` | 取消 |
| | `GET /admin/orders` | 全部订单（管理员） |

错误码：

| code | 含义 |
|---|---|
| 40001 | 参数错误 |
| 40002 | 库存不足 |
| 40003 | 订单状态不允许该操作 |
| 40101 / 40102 | 未登录 / Token 失效 |
| 40301 | 无权限 |
| 40401 | 资源不存在 |
| 40901 | 冲突（重复提交） |
| 50001 | 服务内部错误 |
| 50301 | 依赖服务不可用 |

## 几个设计上的取舍

**金额用整数存"分"**。`money.Cents` 是 `int64`，数据库里是 `DECIMAL(10,2)`，JSON 里是字符串 `"99.90"`。
前端不需要做任何格式化，也不会出现 `0.1 + 0.2 = 0.30000000000000004`。

**下单时价格以服务端为准**。前端只传商品 ID 和数量，价格和库存都由商品服务返回，
避免改前端价格就能低价下单。

**扣库存用条件更新**。`UPDATE ... SET stock = stock - ? WHERE id = ? AND stock >= ?`，
一条语句完成判断和扣减，不会超卖。订单号在 `stock_deductions` 表里唯一，重复扣减会被拦住。

**网关是唯一的身份来源**。网关验 JWT 后注入 `X-User-Id` / `X-User-Role`，
同时会先删掉客户端自带的同名头，防止伪造身份。业务服务端口不对外暴露。

**支付合并在订单服务里**。没有单独的 payment 服务，`POST /orders/:no/pay` 直接把状态改成已支付。
演示项目够用，真接支付网关时再拆出去。

## 测试

```bash
./scripts/tasks.sh test        # 单元测试
./scripts/tasks.sh test-race   # 竞态检测
```

单元测试全部用内存假实现，不依赖数据库。
库存扣减有一个集成测试，需要真实 MySQL，设置 `TEST_MYSQL_DSN` 后才会跑：

```bash
TEST_MYSQL_DSN="root:123456@tcp(127.0.0.1:3306)/mall_product?parseTime=true" \
  go test ./services/product/internal/repository -run TestStock -v
```

## 常见问题

**登录报 `Access denied for user 'mall_user'`**
`.env` 里的 `USER_DB_PASSWORD` 和数据库账号密码对不上。重新跑一次 `./scripts/tasks.sh local-db`。

**服务起来了但打不开页面**
先看 `./scripts/tasks.sh health`。如果 gateway 显示未启动，看 `logs/gateway.log` 里的报错。
最常见的是端口被别的程序占了。

**页面能打开，但接口全 404 或报 50301**
网关正常，但业务服务没起来。`health` 里显示"未启动"的那个服务，去看它的日志：

```bash
./scripts/tasks.sh log product
```

**`stop-all` 提示"未运行"，但端口确实被占着**
说明占端口的不是本项目的进程。用 `netstat -ano | findstr :8080` 看看是哪个 PID。

**改了前端代码没生效**
前端文件是 `go:embed` 打进网关的，改完要重新编译：

```bash
go build -o bin/gateway.exe ./services/gateway/cmd/server
```

**商品图片打不开**
图片存在商品服务的工作目录 `uploads/` 下，网关转发 `/uploads/*` 过去。
确认商品服务是从项目根目录启动的。





