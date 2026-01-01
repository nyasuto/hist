package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	configDirName   = "hist"
	ignoreFileName  = "ignore.txt"
	configDirPerms  = 0755
	configFilePerms = 0644
)

// getConfigDir は設定ディレクトリのパスを返す
func getConfigDir() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("ホームディレクトリの取得に失敗: %w", err)
		}
		configHome = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(configHome, configDirName), nil
}

// getIgnoreListPath はイグノアリストファイルのパスを返す
func getIgnoreListPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, ignoreFileName), nil
}

// ensureConfigDir は設定ディレクトリが存在することを確認する
func ensureConfigDir() error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(configDir, configDirPerms)
}

// LoadIgnoreList はイグノアリストを読み込む
func LoadIgnoreList() ([]string, error) {
	path, err := getIgnoreListPath()
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("イグノアリストの読み込みに失敗: %w", err)
	}
	defer func() { _ = file.Close() }()

	var domains []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// 空行とコメント行をスキップ
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		domains = append(domains, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("イグノアリストの読み込みに失敗: %w", err)
	}

	return domains, nil
}

// SaveIgnoreList はイグノアリストを保存する
func SaveIgnoreList(domains []string) error {
	if err := ensureConfigDir(); err != nil {
		return err
	}

	path, err := getIgnoreListPath()
	if err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("イグノアリストの保存に失敗: %w", err)
	}
	defer func() { _ = file.Close() }()

	for _, domain := range domains {
		if _, err := fmt.Fprintln(file, domain); err != nil {
			return fmt.Errorf("イグノアリストの書き込みに失敗: %w", err)
		}
	}

	return nil
}

// AddToIgnoreList はドメインをイグノアリストに追加する
func AddToIgnoreList(domain string) error {
	domains, err := LoadIgnoreList()
	if err != nil {
		return err
	}

	// 重複チェック
	for _, d := range domains {
		if d == domain {
			return nil // 既に存在する
		}
	}

	domains = append(domains, domain)
	return SaveIgnoreList(domains)
}

// RemoveFromIgnoreList はドメインをイグノアリストから削除する
func RemoveFromIgnoreList(domain string) error {
	domains, err := LoadIgnoreList()
	if err != nil {
		return err
	}

	var newDomains []string
	for _, d := range domains {
		if d != domain {
			newDomains = append(newDomains, d)
		}
	}

	return SaveIgnoreList(newDomains)
}

// PrintIgnoreList はイグノアリストを表示する
func PrintIgnoreList() error {
	domains, err := LoadIgnoreList()
	if err != nil {
		return err
	}

	if len(domains) == 0 {
		fmt.Println("イグノアリストは空です")
		return nil
	}

	fmt.Println("イグノアリスト:")
	for _, d := range domains {
		fmt.Printf("  - %s\n", d)
	}
	return nil
}
