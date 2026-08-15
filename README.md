# 道具箱 (dogubako)

日常作業向けのデスクトップツール群です。GUI はクロスコンパイルしやすい [Guigui](https://github.com/guigui-gui/guigui) を使っています。

対応: **macOS** / **Ubuntu**。Windows は未対応です。

## できること

左サイドメニューからツールを選び、右側のメインパネルで操作します。

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
- `internal/app` — シェル（サイドメニューとメインパネル）と画像ツール UI
- `internal/imageproc` — リサイズ・切り取り・エンコード
- `internal/dialog` — ファイルダイアログ（GTK / zenity / kdialog）
- `packaging` — Ubuntu 向け `.desktop`
