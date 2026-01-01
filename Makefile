.PHONY: build run clean test fmt lint quality help install uninstall serve interactive

# バイナリ名
BINARY := hist

# インストール先
PREFIX := /usr/local/bin

# デフォルトターゲット
all: build

# ビルド
build:
	go build -o $(BINARY)

# 実行
run: build
	./$(BINARY)

# Webサーバーモードで実行
serve: build
	./$(BINARY) -serve

# インタラクティブモードで実行
interactive: build
	./$(BINARY) -interactive

# インストール
install: build
	cp $(BINARY) $(PREFIX)/$(BINARY)
	@echo "$(BINARY) を $(PREFIX) にインストールしました"

# アンインストール
uninstall:
	rm -f $(PREFIX)/$(BINARY)
	@echo "$(BINARY) を $(PREFIX) から削除しました"

# クリーンアップ
clean:
	rm -f $(BINARY)
	go clean

# テスト
test:
	go test -v ./...

# テスト（カバレッジ付き）
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "カバレッジレポートを coverage.html に出力しました"

# フォーマット
fmt:
	go fmt ./...

# リント
lint:
	golangci-lint run

# 品質チェック（テスト + フォーマット + リント）
quality: test fmt lint

# 依存関係の更新
deps:
	go mod tidy
	go mod download

# ヘルプ
help:
	@echo "使用可能なターゲット:"
	@echo "  build         - バイナリをビルド"
	@echo "  run           - ビルドして実行（CLI）"
	@echo "  serve         - Webサーバーモードで実行"
	@echo "  interactive   - インタラクティブモードで実行"
	@echo "  install       - $(PREFIX) にインストール"
	@echo "  uninstall     - $(PREFIX) から削除"
	@echo "  clean         - バイナリを削除"
	@echo "  test          - テストを実行"
	@echo "  test-coverage - カバレッジ付きテスト"
	@echo "  fmt           - コードをフォーマット"
	@echo "  lint          - リントを実行"
	@echo "  quality       - テスト + フォーマット + リント"
	@echo "  deps          - 依存関係を更新"
	@echo "  help          - このヘルプを表示"
