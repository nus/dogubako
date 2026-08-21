# 道具箱 (dogubako)

日常作業向けのデスクトップツール群です。GUI はクロスコンパイルしやすい [gogpu/ui](https://github.com/gogpu/ui) を使っています。

対応: **macOS** / **Ubuntu**。Windows は未対応です。

## できること

左サイドメニューからツールを選び、右側のメインパネルで操作します。サイドバー下部で **日本語 / English** を切り替えられます。初回はシステムの言語設定に従い、選択は次回起動時も保持されます。

### 画像

- 拡大縮小（幅・高さ・倍率、縦横比の維持）
- 回転（90° ボタン、1° 刻みの角度指定）
- 切り取り（数値指定、または加工前プレビュー上のドラッグ）
- JPEG / PNG への形式変換（JPEG は品質指定）
- 入力: ファイル選択、ウィンドウへのドラッグ＆ドロップ、クリップボード（PNG）
- 出力: ファイル保存、またはクリップボード（PNG）
- 加工前 / 加工後のプレビュー
- プレビュー表示の拡大縮小（マウスホイールまたは ＋／−。全体表示でウィンドウに合わせる。加工後はドラッグで移動、加工前の移動は右ドラッグ。左ドラッグは切り取り）

ショートカット（macOS は ⌘、Ubuntu は Ctrl）:

- O: ファイルを開く
- S: ファイルに保存
- V: クリップボードから貼り付け
- C: クリップボードにコピー

### 画面キャプチャ

- 画面全体 / ウィンドウ / 範囲選択
- 遅延（秒）と、キャプチャ前にこのウィンドウを隠すオプション
- キャプチャした画像は保存先へ自動保存
- 保存先フォルダの画像をサムネイル付きリストで表示し、クリックでプレビュー
- プレビュー表示の拡大縮小（ホイールまたは ＋／−。ドラッグで移動、全体表示でウィンドウに合わせる）
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

Ubuntu ではデスクトップに合ったコマンドを優先します（GNOME なら `gnome-screenshot`、それ以外では `maim` / `scrot` など）。1つ目が画像を書かずに終わったときは次のコマンドを試します。範囲選択が止まったときは「やり直す」で中断して再試行できます。

```sh
sudo apt install gnome-screenshot scrot
```

macOS ではシステムの `screencapture` を使います。初回は「画面収録」の許可が必要です（システム設定 → プライバシーとセキュリティ）。

### Android ファイル

USB デバッグ（または無線デバッグ）が有効な Android 端末のファイルを、ADB プロトコルで閲覧・コピーします。`adb pull` / `adb push` などの **adb コマンドは呼び出しません**。pure Go の [go-adbkit](https://github.com/codeskyblue/go-adbkit) が、起動済みの ADB サーバー（`127.0.0.1:5037`）と通信します。

- 接続中のデバイス一覧から選択
- ファイル / フォルダのツリー表示
- サイズ、更新日時（列ヘッダーのクリックで昇順 / 降順を切り替え、境界のドラッグで列幅を変更できます）
- Android → PC、PC → Android のファイル / フォルダコピー

ADB サーバーは Android Studio や SDK Platform-Tools などが起動している必要があります。端末側で USB デバッグを許可してください。

## 必要環境

- Go 1.25 以降（ソースからビルドする場合）
- C コンパイラは不要です（`CGO_ENABLED=0` でビルドできます）

### Ubuntu で実行する場合

デスクトップ環境（GNOME など）と、X11 または Wayland が必要です。描画は Vulkan（なければソフトウェアフォールバック）を使います。実行時に次のパッケージを入れてください。

```sh
sudo apt install libvulkan1 fonts-noto-cjk zenity
```

Vulkan が使えないときは次でソフトウェア描画に切り替えられます。

```sh
export GOGPU_GRAPHICS_API=software
```

ファイルダイアログは `zenity`（なければ `kdialog`）を使います。macOS ではシステムのファイルパネル（osascript）を使います。

```sh
sudo apt install zenity
```

クリップボードの PNG は `xclip`（X11）または `wl-clipboard`（Wayland）を使います。

```sh
sudo apt install xclip
# または Wayland の場合:
sudo apt install wl-clipboard
```

## ビルド

```sh
# 開発中のホスト向け
make build

# macOS 向け Universal DMG
make package-macos

# Ubuntu 向け AppImage（amd64 / arm64）
make package-ubuntu
```

成果物は `dist/` に出力されます。

| 環境 | 成果物 |
| --- | --- |
| macOS Universal（DMG） | `dist/Dogubako-0.1.0-macos-universal.dmg` |
| Ubuntu amd64（AppImage） | `dist/Dogubako-0.1.0-linux-amd64.AppImage` |
| Ubuntu arm64（AppImage） | `dist/Dogubako-0.1.0-linux-arm64.AppImage` |

`v*` タグを push すると GitHub Actions がパッケージを作り、GitHub Release の Assets に次を載せます。

- `Dogubako-<version>-macos-universal.dmg`
- `Dogubako-<version>-linux-amd64.AppImage`
- `Dogubako-<version>-linux-arm64.AppImage`

バージョンは `make package-macos VERSION=1.2.3` や `make package-ubuntu VERSION=1.2.3` で変えられます。Universal バイナリは [konoui/lipo](https://github.com/konoui/lipo) で作るので、macOS の `lipo` は不要です。macOS の配布パッケージは Universal DMG のみです。

`make package-ubuntu` には `squashfs-tools`（`mksquashfs`）が必要です。AppImage のランタイムはビルド時に [type2-runtime](https://github.com/AppImage/type2-runtime) から取得します。オフラインで作るときは `APPIMAGE_RUNTIME_x86_64` / `APPIMAGE_RUNTIME_aarch64` に ELF を渡します。

`.dmg` を開いて **Dogubako** を Applications へドラッグします。日本語環境では Finder 上の名前は「道具箱」です。署名していない配布物は、初回だけコントロールキーを押しながら開くか、右クリック → 開く、で Gatekeeper を通します。

Apple Developer 証明書がある場合は、Mac 上で署名できます。

```sh
MACOS_SIGN_IDENTITY="Developer ID Application: …" make package-macos
```

`make package-macos` は Linux からも実行できます。その場合の `.dmg` は ISO 9660 / Joliet / Rock Ridge で、macOS の DiskImageMounter が開けます。GitHub Release の DMG は macOS runner 上の `hdiutil` で作る UDZO（HFS+）イメージで、開くと Applications へドラッグできる構成です。

Ubuntu では AppImage に実行ビットを付けて起動します。Vulkan / フォント / zenity はホストのデスクトップ環境のものを使います（上記の実行時パッケージ）。

```sh
chmod +x dist/Dogubako-0.1.0-linux-amd64.AppImage
./dist/Dogubako-0.1.0-linux-amd64.AppImage
```

FUSE でマウントできないときは次でも起動できます。

```sh
./dist/Dogubako-0.1.0-linux-amd64.AppImage --appimage-extract-and-run
```

開発用の単体バイナリは `make build` で作れます。

```sh
./dist/dogubako
```

アプリケーションメニューに出す場合は、バイナリを `PATH` の通る場所へ置き、デスクトップエントリをコピーします。

```sh
sudo install -m 755 dist/dogubako /usr/local/bin/dogubako
mkdir -p ~/.local/share/icons/hicolor/1024x1024/apps
install -m 644 internal/appicon/icon.png ~/.local/share/icons/hicolor/1024x1024/apps/dogubako.png
cp packaging/dogubako.desktop ~/.local/share/applications/
```

```sh
make test
```

## フォント

日本語表示用の Noto Sans CJK は Linux ではシステムフォント（`fonts-noto-cjk`）から開きます。macOS では `/System/Library/Fonts/ヒラギノ角ゴシック W3.ttc`（無ければ W4 以降）を開きます。フォルダ／ファイル／開閉マークは絵文字フォントを使わず、ベクトルで描きます。

## ディレクトリ

- `cmd/dogubako` — エントリポイント
- `internal/cjkembed` — Linux は Noto Sans CJK、macOS はヒラギノ角ゴシックをシステムフォントから開く
- `internal/app` — シェル（サイドメニューとメインパネル）、画像ツール、画面キャプチャ、Android ファイル
- `internal/adbfs` — ADB プロトコルによるデバイス一覧・ファイル同期（pure Go）
- `internal/appicon` — アプリ／パッケージ用アイコン
- `internal/imageproc` — リサイズ・切り取り・エンコード
- `internal/capture` — OS の画面キャプチャコマンド呼び出し
- `internal/clipimg` — クリップボードの PNG 読み書き（xclip / wl-clipboard / osascript）
- `internal/userdir` — ピクチャ / スクリーンショットフォルダの解決
- `internal/dialog` — ファイルダイアログ（macOS は osascript、Linux は zenity / kdialog）
- `packaging` — Ubuntu 向け `.desktop` / AppImage、macOS 向け `.app` / `.dmg`
