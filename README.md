# 道具箱 (dogubako)

日常作業向けのデスクトップツール群です。GUI はクロスコンパイルしやすい [Guigui](https://github.com/guigui-gui/guigui) を使っています。

対応: **macOS** / **Ubuntu**。Windows は未対応です。

## できること

左サイドメニューからツールを選び、右側のメインパネルで操作します。サイドバー下部で **日本語 / English** を切り替えられます。初回はシステムの言語設定に従い、選択は次回起動時も保持されます。

### 画像

- 拡大縮小（幅・高さ・倍率、縦横比の維持）
- 切り取り（数値指定、または加工前プレビュー上のドラッグ）
- JPEG / PNG への形式変換（JPEG は品質指定）
- 入力: ファイル選択、ウィンドウへのドラッグ＆ドロップ、クリップボード（PNG）
- 出力: ファイル保存、またはクリップボード（PNG）
- 加工前 / 加工後のプレビュー

ショートカット（macOS は ⌘、Ubuntu は Ctrl）:

- O: ファイルを開く
- S: ファイルに保存
- V: クリップボードから貼り付け
- C: クリップボードにコピー

### 画面キャプチャ

- 画面全体 / ウィンドウ / 範囲選択
- 遅延（秒）と、キャプチャ前にこのウィンドウを隠すオプション
- キャプチャした画像は保存先へ自動保存
- 保存先フォルダの画像をリスト表示し、クリックでプレビュー
- クリップボードへコピー、名前を付けて保存、画像ツールへ送る

既定の保存先:

| 環境 | 場所 |
| --- | --- |
| macOS | `~/Pictures/Screenshots` |
| Ubuntu | `XDG_SCREENSHOTS_DIR`、なければ「ピクチャ」フォルダ（日本語環境では多くの場合 `~/画像`）の `Screenshots` |

Linux では `~/.config/user-dirs.dirs` を読むので、ホーム直下に英語の `Pictures` を新しく作りません。macOS の Cmd-Shift-3 既定であるデスクトップは使いません。

ショートカット（macOS は ⌘、Ubuntu は Ctrl）:

- S: 保存先へもう一度保存（別ファイル名）
- C: クリップボードにコピー

Ubuntu では `gnome-screenshot` があると一番確実です。無ければ `spectacle` / `maim` / `scrot` / `grim` / ImageMagick の `import` の順で探します。

```sh
sudo apt install gnome-screenshot
```

macOS ではシステムの `screencapture` を使います。初回は「画面収録」の許可が必要です（システム設定 → プライバシーとセキュリティ）。

## 必要環境

- Go 1.25 以降（ソースからビルドする場合）
- C コンパイラは不要です（`CGO_ENABLED=0` でビルドできます）

### Ubuntu で実行する場合

デスクトップ環境（GNOME など）と X11、または XWayland が必要です。実行時に次のパッケージを入れてください。

```sh
sudo apt install libgtk-3-0 libgl1 libx11-6 libxcursor1 libxi6 libxinerama1 libxrandr2 libxxf86vm1
```

ファイルダイアログは GTK 3 を使います。GTK が使えないときは `zenity` または `kdialog` にフォールバックします。

```sh
sudo apt install zenity
```

クリップボードの画像は X11 経由です。Wayland セッションでは XWayland 上で動かしてください。

## ビルド

```sh
# 開発中のホスト向け
make build

# macOS 向け（arm64 / amd64）
make build-macos

# Ubuntu / Linux 向け（amd64 / arm64）
make build-ubuntu
```

成果物は `dist/` に出力されます。

| 環境 | バイナリ |
| --- | --- |
| macOS Apple Silicon | `dist/dogubako-darwin-arm64` |
| macOS Intel | `dist/dogubako-darwin-amd64` |
| Ubuntu amd64 | `dist/dogubako-linux-amd64` |
| Ubuntu arm64 | `dist/dogubako-linux-arm64` |

Ubuntu では実行ビットを付けて起動します。

```sh
chmod +x dist/dogubako-linux-amd64
./dist/dogubako-linux-amd64
```

アプリケーションメニューに出す場合は、バイナリを `PATH` の通る場所へ置き、デスクトップエントリをコピーします。

```sh
sudo install -m 755 dist/dogubako-linux-amd64 /usr/local/bin/dogubako
cp packaging/dogubako.desktop ~/.local/share/applications/
```

```sh
make test
```

## ディレクトリ

- `cmd/dogubako` — エントリポイント
- `internal/app` — シェル（サイドメニューとメインパネル）、画像ツール、画面キャプチャ
- `internal/imageproc` — リサイズ・切り取り・エンコード
- `internal/capture` — OS の画面キャプチャコマンド呼び出し
- `internal/userdir` — ピクチャ / スクリーンショットフォルダの解決
- `internal/dialog` — ファイルダイアログ（GTK / zenity / kdialog）
- `packaging` — Ubuntu 向け `.desktop`
