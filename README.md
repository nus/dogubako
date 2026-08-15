# 道具箱 (dogubako)

日常作業向けのデスクトップツール群です。GUI はクロスコンパイルしやすい [Guigui](https://github.com/guigui-gui/guigui) を使っています。

対応予定: macOS / Ubuntu / Windows。**最初のターゲットは macOS のみ**です。

## できること

左サイドメニューからツールを選び、右側のメインパネルで操作します。

### 画像

- 拡大縮小（幅・高さ・倍率、縦横比の維持）
- 切り取り（数値指定、または加工前プレビュー上のドラッグ）
- JPEG / PNG への形式変換（JPEG は品質指定）
- 入力: ファイル選択、ウィンドウへのドラッグ＆ドロップ、クリップボード（PNG）
- 出力: ファイル保存、またはクリップボード（PNG）
- 加工前 / 加工後のプレビュー

ショートカット（macOS は ⌘、その他は Ctrl）:

- O: ファイルを開く
- S: ファイルに保存
- V: クリップボードから貼り付け
- C: クリップボードにコピー

## 必要環境

- Go 1.25 以降
- macOS で実行する場合、追加の C コンパイラは不要です

## ビルド

```sh
# 開発中のホスト向け
make build

# macOS 向けクロスコンパイル（arm64 / amd64）
make build-macos
```

成果物は `dist/` に出力されます。macOS では `dist/dogubako-darwin-arm64` または `dist/dogubako-darwin-amd64` を実行してください。

```sh
make test
```

## ディレクトリ

- `cmd/dogubako` — エントリポイント
- `internal/app` — シェル（サイドメニューとメインパネル）と画像ツール UI
- `internal/imageproc` — リサイズ・切り取り・エンコード
- `internal/dialog` — ネイティブのファイルダイアログ
