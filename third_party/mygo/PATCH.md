# 内置的 mygo（打过补丁）

这份是 [egoist/mygo](https://github.com/egoist/mygo) **v0.7.3** 的拷贝（MIT，见 LICENSE），
只做了一处改动：**让每个 session 可以单独走代理**。go.mod 用
`replace github.com/egoist/mygo => ./third_party/mygo` 指向它。

删除的目录（与本补丁无关，只是不需要）：`examples/ website/ docs/ packages/
plugins/ .github/ testdata/ scripts/` 和几个 npm/CI 文件。

## 补丁内容

1. `internal/platform/platform.go`：新增可选的 `ProxyStore` 接口（`SetProxy(spec string, bypass []string) error`）。
   没有代理的后端不用实现它，`Session.SetProxy` 用类型断言找。
2. `session.go`：新增公开 API `(*Session).SetProxy(spec string, bypass ...string) error`。
   三态：空字符串 = 跟随系统代理；`"direct"` = 直连（无视系统代理）；其它按
   `http://127.0.0.1:7890`、`https://…`、`socks5://user:pass@127.0.0.1:1080` 解析。
   bypass 里的域名及其子域直连。
3. `internal/darwin/proxy.go`（新增）：用 purego 调 Network.framework 的
   `nw_endpoint_create_host` / `nw_proxy_config_create_http_connect` /
   `nw_proxy_config_create_socksv5` / `nw_proxy_config_set_username_and_password` /
   `nw_proxy_config_add_excluded_domain`，把配置交给
   `-[WKWebsiteDataStore setProxyConfigurations:]`（macOS 14+；更早的系统返回错误）。
   Network.framework 在第一次设置代理时才 dlopen。

## 为什么需要它

WKWebView 没有 per-webview 的代理开关，而且不认 `HTTP(S)_PROXY` 环境变量（实测无效）；
macOS 上唯一能按会话分流的入口就是 `WKWebsiteDataStore.proxyConfigurations`，
而 mygo 原来的公开 API 里既没有这个字段，也没暴露 data store 指针。

## 升级 mygo 时

```sh
# 1) 取新版本源码
go mod download github.com/egoist/mygo@vX.Y.Z
# 2) 覆盖这份拷贝，保留 LICENSE 与 PATCH.md，按上面三条重新打补丁
# 3) 改 go.mod 里的版本号并构建
make test && make build
```
