.PHONY: build run clean test fmt lint quality help

# バイナリ名
BINARY := hist

# デフォルトターゲット
all: build

# ビルド
build:
	go build -o $(BINARY)

# 実行
run: build
	./$(BINARY)

# クリーンアップ
clean:
	rm -f $(BINARY)

# テスト
test:
	go test -v ./...

# フォーマット
fmt:
	go fmt ./...

# リント
lint:
	golangci-lint run

# 品質チェック（テスト + フォーマット + リント）
quality: test fmt lint

# ヘルプ
help:
	@echo "使用可能なターゲット:"
	@echo "  build    - バイナリをビルド"
	@echo "  run      - ビルドして実行"
	@echo "  clean    - バイナリを削除"
	@echo "  test     - テストを実行"
	@echo "  fmt      - コードをフォーマット"
	@echo "  lint     - リントを実行"
	@echo "  quality  - テスト + フォーマット + リント"
	@echo "  help     - このヘルプを表示"
