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
- 保存先フォルダの画像をサムネイル付きリストで表示し、クリックでプレビュー
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

Ubuntu では AppImage に実行ビットを付けて起動します。GTK / OpenGL / X11 はホストのデスクトップ環境のものを使います（上記の実行時パッケージ）。

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

## メモリ使用量

日本語表示のため `guigui/basicwidget/cjkfont` が Noto Sans CJK（SC / TC / HK / JP / KR の 5 書体、圧縮約 19 MiB）を **起動時に展開** します。ヒープの大半はここにあります。

`make memstat-nocjk` は比較用です。既定フォント（Inter）にかな・漢字・CJK 約物のグリフがなく、Guigui は OS のフォントにもフォールバックしません。CJK なしでも言語切替やウィンドウタイトル（OS 描画）は動きますが、キャンバス上の日本語は欠字（`.notdef` の豆腐）になります。日本語 UI の本番代替にはなりません。

再計測（ウィンドウなしは `-frames=0`。ウィンドウありはディスプレイまたは Xvfb が必要です）:

```sh
make memstat                          # 道具箱、CJK あり（本番と同じ）
make memstat MEMSTATFLAGS='-frames=0' # init 直後だけ
make memstat-nocjk MEMSTATFLAGS='-widget=empty -frames=30'
```

Linux（amd64、GC 後）での一例:

| 条件 | ヒープ in-use | RSS |
| --- | ---: | ---: |
| init、CJK あり | 179 MiB | 208 MiB |
| init、CJK なし（`-tags nocjk`） | 9 MiB | 24 MiB |
| 空ウィンドウ 30 フレーム、CJK あり | 179 MiB | 331 MiB |
| 道具箱 30 フレーム、CJK あり | 190 MiB | 356 MiB |

pprof では in-use ヒープの約 88% が `go-text/typesetting/font.NewFont`（CJK の OpenType テーブル）です。ウィンドウを開いたあとの RSS 増分は GPU / オフスクリーン側で、ソフトウェアレンダラ（llvmpipe）では大きく出ます。

## ディレクトリ

- `cmd/dogubako` — エントリポイント
- `cmd/memstat` — Guigui 起動後のヒープ / RSS を測る（`make memstat`。CJK フォントなしは `make memstat-nocjk`）
- `internal/memstat` — ヒープ / RSS のスナップショット
- `internal/app` — シェル（サイドメニューとメインパネル）、画像ツール、画面キャプチャ
- `internal/appicon` — アプリ／パッケージ用アイコン
- `internal/imageproc` — リサイズ・切り取り・エンコード
- `internal/capture` — OS の画面キャプチャコマンド呼び出し
- `internal/userdir` — ピクチャ / スクリーンショットフォルダの解決
- `internal/dialog` — ファイルダイアログ（GTK / zenity / kdialog）
- `packaging` — Ubuntu 向け `.desktop` / AppImage、macOS 向け `.app` / `.dmg`
