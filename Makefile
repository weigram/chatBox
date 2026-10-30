# chatBox 用 mygo：纯 Go，没有 cgo，也没有前端构建。
#
# 需要 Go 1.27.3 或更新（go.mod 的要求），用的就是系统装的 Go；
# 想换一个二进制，make GO=/path/to/go。

GO    ?= go
CACHE := $(CURDIR)/.cache

# 官方 proxy 在国内常常不通，换 goproxy.cn。
export GOPROXY ?= https://goproxy.cn,direct
export GOFLAGS ?= -mod=mod

# 默认用 Go 自己的缓存目录（macOS 上是 ~/Library/Caches/go-build、~/go/pkg/mod）。
# 只能写当前目录的环境（沙箱、临时机），或者不想污染全局缓存时，用 make LOCAL=1：
# 缓存落到仓库里的 .cache/。
ifeq ($(LOCAL),1)
export GOCACHE    ?= $(CACHE)/go-build
export GOMODCACHE ?= $(CACHE)/mod
export GOTMPDIR   ?= $(CACHE)/tmp
endif

.PHONY: all run dev test vet fmt build dmg watch icon clean

all: run

## run: 直接跑应用
run:
	$(GO) run .

## dev: mygo 的开发模式，改了代码自动重启
dev:
	$(GO) tool mygo dev

## test: 视图与数据层的单元测试，不需要窗口
test:
	$(GO) test ./...

## vet: 静态检查
vet:
	$(GO) vet ./...

## fmt: 格式化
fmt:
	$(GO) fmt ./...

## build: 打包成本机的应用（macOS 上是 build/darwin-arm64/chatBox.app）
build:
	$(GO) tool mygo build -skip-dmg

## dmg: 打包，并在 macOS 上生成磁盘映像
dmg:
	$(GO) tool mygo build

## watch: 常驻，改完任何 .go / 资源就自动 make build
watch:
	$(GO) run ./tools/watchbuild

## icon: 重新绘制 resources/icon.png
icon:
	$(GO) run ./tools/mkicon

## clean: 清掉构建产物
clean:
	rm -rf build .mygo
