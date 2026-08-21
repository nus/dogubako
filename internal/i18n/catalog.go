package i18n

import "fmt"

// Key identifies a translatable string.
type Key string

const (
	AppTitle       Key = "app.title"
	ToolImage      Key = "tool.image"
	ToolScreenshot Key = "tool.screenshot"
	ToolAndroid    Key = "tool.android"

	OpenFile       Key = "image.open_file"
	PasteClipboard Key = "image.paste_clipboard"
	InputHint      Key = "image.input_hint"
	Before         Key = "image.before"
	BeforeSize     Key = "image.before_size"
	After          Key = "image.after"
	AfterSize      Key = "image.after_size"
	AfterEmpty     Key = "image.after_empty"
	SaveFile       Key = "image.save_file"
	CopyClipboard  Key = "image.copy_clipboard"
	OutputHint     Key = "image.output_hint"
	Resize         Key = "image.resize"
	ScalePercent   Key = "image.scale_percent"
	WidthPx        Key = "image.width_px"
	HeightPx       Key = "image.height_px"
	KeepAspect     Key = "image.keep_aspect"
	ResetSize      Key = "image.reset_size"
	Crop           Key = "image.crop"
	CropEnable     Key = "image.crop_enable"
	CropX          Key = "image.crop_x"
	CropY          Key = "image.crop_y"
	Width          Key = "image.width"
	Height         Key = "image.height"
	ResetCrop      Key = "image.reset_crop"
	Rotate         Key = "image.rotate"
	RotateAngle    Key = "image.rotate_angle"
	Rotate90       Key = "image.rotate_90"
	ResetRotate    Key = "image.reset_rotate"
	Format         Key = "image.format"
	OutputFormat   Key = "image.output_format"
	JPEGQuality    Key = "image.jpeg_quality"

	ZoomFit Key = "preview.zoom_fit"

	ScreenshotCapture      Key = "screenshot.capture"
	ScreenshotCaptureRetry Key = "screenshot.capture_retry"
	ScreenshotMode         Key = "screenshot.mode"
	ScreenshotFull         Key = "screenshot.full"
	ScreenshotWindow       Key = "screenshot.window"
	ScreenshotRegion       Key = "screenshot.region"
	ScreenshotDelay        Key = "screenshot.delay"
	ScreenshotHideWindow   Key = "screenshot.hide_window"
	ScreenshotEmpty        Key = "screenshot.empty"
	ScreenshotPreview      Key = "screenshot.preview"
	ScreenshotPreviewSz    Key = "screenshot.preview_size"
	ScreenshotDest         Key = "screenshot.dest"
	ScreenshotFiles        Key = "screenshot.files"
	ScreenshotSaveAs       Key = "screenshot.save_as"
	ScreenshotCopy         Key = "screenshot.copy"
	ScreenshotSendImage    Key = "screenshot.send_image"
	ScreenshotShowFolder   Key = "screenshot.show_folder"

	AndroidDevices     Key = "android.devices"
	AndroidFiles       Key = "android.files"
	AndroidRefresh     Key = "android.refresh"
	AndroidUp          Key = "android.up"
	AndroidPull        Key = "android.pull"
	AndroidPushFile    Key = "android.push_file"
	AndroidPushFolder  Key = "android.push_folder"
	AndroidEmpty       Key = "android.empty"
	AndroidColName     Key = "android.col_name"
	AndroidColSize     Key = "android.col_size"
	AndroidColModified Key = "android.col_modified"
	AndroidNoDevices   Key = "android.no_devices"
	AndroidHint        Key = "android.hint"
	DialogOpenDir      Key = "dialog.open_dir"
	DialogSaveAny      Key = "dialog.save_any"
	DialogOpenAny      Key = "dialog.open_any"
	DialogOpenFolder   Key = "dialog.open_folder"

	DialogOpen   Key = "dialog.open"
	DialogSave   Key = "dialog.save"
	FilterImages Key = "dialog.filter_images"
	FilterImage  Key = "dialog.filter_image"

	StatusClipboardReadFailed      Key = "status.clipboard_read_failed"
	StatusNoImageToCopy            Key = "status.no_image_to_copy"
	StatusClipboardCopyFailed      Key = "status.clipboard_copy_failed"
	StatusClipboardCopied          Key = "status.clipboard_copied"
	StatusOpenFailed               Key = "status.open_failed"
	StatusSaveDialogFailed         Key = "status.save_dialog_failed"
	StatusNoFileDialog             Key = "status.no_file_dialog"
	StatusLoadFailed               Key = "status.load_failed"
	StatusLoaded                   Key = "status.loaded"
	StatusDropNoImage              Key = "status.drop_no_image"
	StatusClipboardNoImage         Key = "status.clipboard_no_image"
	StatusClipboardImageReadFailed Key = "status.clipboard_image_read_failed"
	StatusPasted                   Key = "status.pasted"
	StatusExportFailed             Key = "status.export_failed"
	StatusEncodeFailed             Key = "status.encode_failed"
	StatusSaveFailed               Key = "status.save_failed"
	StatusSaved                    Key = "status.saved"
	StatusCaptureInProgress        Key = "status.capture_in_progress"
	StatusCaptureCancelled         Key = "status.capture_cancelled"
	StatusCaptureTimeout           Key = "status.capture_timeout"
	StatusCaptureFailed            Key = "status.capture_failed"
	StatusCaptureNoTool            Key = "status.capture_no_tool"
	StatusCaptured                 Key = "status.captured"
	StatusNoCaptureToSave          Key = "status.no_capture_to_save"
	StatusNoCaptureToCopy          Key = "status.no_capture_to_copy"
	StatusDestFailed               Key = "status.dest_failed"
	StatusFolderOpenFailed         Key = "status.folder_open_failed"
	StatusAdbConnectFailed         Key = "status.adb_connect_failed"
	StatusAdbNoDevices             Key = "status.adb_no_devices"
	StatusAdbDeviceOffline         Key = "status.adb_device_offline"
	StatusAdbListing               Key = "status.adb_listing"
	StatusAdbListed                Key = "status.adb_listed"
	StatusAdbListFailed            Key = "status.adb_list_failed"
	StatusAdbCopying               Key = "status.adb_copying"
	StatusAdbCopied                Key = "status.adb_copied"
	StatusAdbCopyFailed            Key = "status.adb_copy_failed"
	StatusAdbNoSelection           Key = "status.adb_no_selection"
	StatusAdbSelectOnline          Key = "status.adb_select_online"
)

var catalogs = map[Lang]map[Key]string{
	JA: {
		AppTitle:                       "道具箱",
		ToolImage:                      "画像",
		ToolScreenshot:                 "画面キャプチャ",
		ToolAndroid:                    "Android ファイル",
		OpenFile:                       "ファイルを開く",
		PasteClipboard:                 "クリップボードから貼り付け",
		InputHint:                      "ファイル指定・ドロップ・%s で入力",
		Before:                         "加工前",
		BeforeSize:                     "加工前  %d×%d",
		After:                          "加工後",
		AfterSize:                      "加工後  %d×%d  %s",
		AfterEmpty:                     "画像を開くか、ウィンドウへドロップしてください。",
		SaveFile:                       "ファイルに保存",
		CopyClipboard:                  "クリップボードにコピー",
		OutputHint:                     "出力はファイルまたはクリップボード",
		Resize:                         "拡大縮小",
		ScalePercent:                   "倍率 (%)",
		WidthPx:                        "幅 (px)",
		HeightPx:                       "高さ (px)",
		KeepAspect:                     "縦横比を維持",
		ResetSize:                      "元のサイズに戻す",
		Crop:                           "切り取り",
		CropEnable:                     "切り取りを使う",
		CropX:                          "X",
		CropY:                          "Y",
		Width:                          "幅",
		Height:                         "高さ",
		ResetCrop:                      "切り取りをリセット",
		Rotate:                         "回転",
		RotateAngle:                    "角度 (°)",
		Rotate90:                       "90° 回転",
		ResetRotate:                    "回転をリセット",
		Format:                         "形式",
		OutputFormat:                   "出力形式",
		JPEGQuality:                    "JPEG 品質  %d",
		ZoomFit:                        "全体",
		ScreenshotCapture:              "キャプチャ",
		ScreenshotCaptureRetry:         "やり直す",
		ScreenshotMode:                 "対象",
		ScreenshotFull:                 "画面全体",
		ScreenshotWindow:               "ウィンドウ",
		ScreenshotRegion:               "範囲選択",
		ScreenshotDelay:                "遅延 (秒)",
		ScreenshotHideWindow:           "このウィンドウを隠す",
		ScreenshotEmpty:                "キャプチャするか、左のリストから画像を選んでください。",
		ScreenshotPreview:              "プレビュー",
		ScreenshotPreviewSz:            "プレビュー  %d×%d",
		ScreenshotDest:                 "保存先  %s",
		ScreenshotFiles:                "保存済み",
		ScreenshotSaveAs:               "名前を付けて保存",
		ScreenshotCopy:                 "クリップボードにコピー",
		ScreenshotSendImage:            "画像ツールへ送る",
		ScreenshotShowFolder:           "フォルダを開く",
		AndroidDevices:                 "デバイス",
		AndroidFiles:                   "ファイル",
		AndroidRefresh:                 "再読み込み",
		AndroidUp:                      "上へ",
		AndroidPull:                    "PCへコピー",
		AndroidPushFile:                "PCのファイルをコピー",
		AndroidPushFolder:              "PCのフォルダをコピー",
		AndroidEmpty:                   "フォルダは空です。デバイスを選び、USB デバッグを許可してください。",
		AndroidColName:                 "名前",
		AndroidColSize:                 "サイズ",
		AndroidColModified:             "更新日時",
		AndroidNoDevices:               "接続中のデバイスはありません",
		AndroidHint:                    "ADB プロトコルで端末のファイルを閲覧・コピーします（adb コマンドは使いません）。",
		DialogOpenDir:                  "保存先フォルダ",
		DialogSaveAny:                  "ファイルを保存",
		DialogOpenAny:                  "ファイルを選ぶ",
		DialogOpenFolder:               "フォルダを選ぶ",
		DialogOpen:                     "画像を開く",
		DialogSave:                     "画像を保存",
		FilterImages:                   "画像",
		FilterImage:                    "画像",
		StatusClipboardReadFailed:      "クリップボードを読めませんでした",
		StatusNoImageToCopy:            "コピーする画像がありません",
		StatusClipboardCopyFailed:      "クリップボードへのコピーに失敗しました",
		StatusClipboardCopied:          "クリップボードにコピーしました",
		StatusOpenFailed:               "ファイルを開けませんでした: %v",
		StatusSaveDialogFailed:         "保存ダイアログに失敗しました: %v",
		StatusNoFileDialog:             "ファイルダイアログを開けません。Ubuntu では zenity をインストールしてください",
		StatusLoadFailed:               "読み込みに失敗しました: %v",
		StatusLoaded:                   "%s を読み込みました（%d×%d）",
		StatusDropNoImage:              "ドロップされたファイルに画像がありません",
		StatusClipboardNoImage:         "クリップボードに画像がありません",
		StatusClipboardImageReadFailed: "クリップボードの画像を読めません: %v",
		StatusPasted:                   "クリップボードから貼り付けました（%d×%d）",
		StatusExportFailed:             "書き出せません: %v",
		StatusEncodeFailed:             "エンコードに失敗しました: %v",
		StatusSaveFailed:               "保存に失敗しました: %v",
		StatusSaved:                    "保存しました: %s",
		StatusCaptureInProgress:        "キャプチャしています…",
		StatusCaptureCancelled:         "キャプチャをキャンセルしました",
		StatusCaptureTimeout:           "キャプチャがタイムアウトしました",
		StatusCaptureFailed:            "キャプチャに失敗しました: %v",
		StatusCaptureNoTool:            "画面キャプチャのコマンドが見つかりません。Ubuntu では gnome-screenshot をインストールしてください",
		StatusCaptured:                 "キャプチャしました（%d×%d）",
		StatusNoCaptureToSave:          "保存するキャプチャがありません",
		StatusNoCaptureToCopy:          "コピーするキャプチャがありません",
		StatusDestFailed:               "保存先を用意できませんでした: %v",
		StatusFolderOpenFailed:         "フォルダを開けませんでした: %v",
		StatusAdbConnectFailed:         "ADB サーバーに接続できません: %v",
		StatusAdbNoDevices:             "接続中のデバイスがありません。USB デバッグを有効にして端末を接続してください",
		StatusAdbDeviceOffline:         "このデバイスはまだ使えません（%s）",
		StatusAdbListing:               "読み込んでいます…",
		StatusAdbListed:                "%s を表示しています（%d 件）",
		StatusAdbListFailed:            "一覧を取得できません: %v",
		StatusAdbCopying:               "コピーしています…",
		StatusAdbCopied:                "コピーしました（%d 件）: %s",
		StatusAdbCopyFailed:            "コピーに失敗しました: %v",
		StatusAdbNoSelection:           "コピーするファイルまたはフォルダを選んでください",
		StatusAdbSelectOnline:          "オンラインのデバイスを選んでください",
	},
	EN: {
		AppTitle:                       "Dogubako",
		ToolImage:                      "Image",
		ToolScreenshot:                 "Screenshot",
		ToolAndroid:                    "Android Files",
		OpenFile:                       "Open File",
		PasteClipboard:                 "Paste from Clipboard",
		InputHint:                      "Open, drop, or paste with %s",
		Before:                         "Before",
		BeforeSize:                     "Before  %d×%d",
		After:                          "After",
		AfterSize:                      "After  %d×%d  %s",
		AfterEmpty:                     "Open an image or drop a file onto the window.",
		SaveFile:                       "Save File",
		CopyClipboard:                  "Copy to Clipboard",
		OutputHint:                     "Save to a file or copy to the clipboard",
		Resize:                         "Resize",
		ScalePercent:                   "Scale (%)",
		WidthPx:                        "Width (px)",
		HeightPx:                       "Height (px)",
		KeepAspect:                     "Keep aspect ratio",
		ResetSize:                      "Reset size",
		Crop:                           "Crop",
		CropEnable:                     "Enable crop",
		CropX:                          "X",
		CropY:                          "Y",
		Width:                          "Width",
		Height:                         "Height",
		ResetCrop:                      "Reset crop",
		Rotate:                         "Rotate",
		RotateAngle:                    "Angle (°)",
		Rotate90:                       "Rotate 90°",
		ResetRotate:                    "Reset rotation",
		Format:                         "Format",
		OutputFormat:                   "Output format",
		JPEGQuality:                    "JPEG quality  %d",
		ZoomFit:                        "Fit",
		ScreenshotCapture:              "Capture",
		ScreenshotCaptureRetry:         "Retry",
		ScreenshotMode:                 "Target",
		ScreenshotFull:                 "Full screen",
		ScreenshotWindow:               "Window",
		ScreenshotRegion:               "Region",
		ScreenshotDelay:                "Delay (s)",
		ScreenshotHideWindow:           "Hide this window",
		ScreenshotEmpty:                "Capture a screenshot, or choose one from the list.",
		ScreenshotPreview:              "Preview",
		ScreenshotPreviewSz:            "Preview  %d×%d",
		ScreenshotDest:                 "Save to  %s",
		ScreenshotFiles:                "Saved",
		ScreenshotSaveAs:               "Save As",
		ScreenshotCopy:                 "Copy to Clipboard",
		ScreenshotSendImage:            "Send to Image tool",
		ScreenshotShowFolder:           "Open Folder",
		AndroidDevices:                 "Devices",
		AndroidFiles:                   "Files",
		AndroidRefresh:                 "Reload",
		AndroidUp:                      "Up",
		AndroidPull:                    "Copy to PC",
		AndroidPushFile:                "Copy File from PC",
		AndroidPushFolder:              "Copy Folder from PC",
		AndroidEmpty:                   "This folder is empty. Select a device and allow USB debugging.",
		AndroidColName:                 "Name",
		AndroidColSize:                 "Size",
		AndroidColModified:             "Modified",
		AndroidNoDevices:               "No devices connected",
		AndroidHint:                    "Browse and copy device files over the ADB protocol (the adb command is not used).",
		DialogOpenDir:                  "Destination Folder",
		DialogSaveAny:                  "Save File",
		DialogOpenAny:                  "Choose File",
		DialogOpenFolder:               "Choose Folder",
		DialogOpen:                     "Open Image",
		DialogSave:                     "Save Image",
		FilterImages:                   "Images",
		FilterImage:                    "Image",
		StatusClipboardReadFailed:      "Could not read the clipboard",
		StatusNoImageToCopy:            "No image to copy",
		StatusClipboardCopyFailed:      "Failed to copy to the clipboard",
		StatusClipboardCopied:          "Copied to the clipboard",
		StatusOpenFailed:               "Could not open the file: %v",
		StatusSaveDialogFailed:         "Save dialog failed: %v",
		StatusNoFileDialog:             "Cannot open a file dialog. On Ubuntu, install zenity.",
		StatusLoadFailed:               "Failed to load: %v",
		StatusLoaded:                   "Loaded %s (%d×%d)",
		StatusDropNoImage:              "No image in the dropped files",
		StatusClipboardNoImage:         "No image on the clipboard",
		StatusClipboardImageReadFailed: "Could not read the clipboard image: %v",
		StatusPasted:                   "Pasted from the clipboard (%d×%d)",
		StatusExportFailed:             "Could not export: %v",
		StatusEncodeFailed:             "Encoding failed: %v",
		StatusSaveFailed:               "Failed to save: %v",
		StatusSaved:                    "Saved: %s",
		StatusCaptureInProgress:        "Capturing…",
		StatusCaptureCancelled:         "Capture cancelled",
		StatusCaptureTimeout:           "Capture timed out",
		StatusCaptureFailed:            "Capture failed: %v",
		StatusCaptureNoTool:            "No screenshot command found. On Ubuntu, install gnome-screenshot.",
		StatusCaptured:                 "Captured (%d×%d)",
		StatusNoCaptureToSave:          "No screenshot to save",
		StatusNoCaptureToCopy:          "No screenshot to copy",
		StatusDestFailed:               "Could not prepare the save folder: %v",
		StatusFolderOpenFailed:         "Could not open the folder: %v",
		StatusAdbConnectFailed:         "Could not connect to the ADB server: %v",
		StatusAdbNoDevices:             "No devices connected. Enable USB debugging and plug in a device.",
		StatusAdbDeviceOffline:         "This device is not ready (%s)",
		StatusAdbListing:               "Loading…",
		StatusAdbListed:                "Showing %s (%d items)",
		StatusAdbListFailed:            "Could not list files: %v",
		StatusAdbCopying:               "Copying…",
		StatusAdbCopied:                "Copied %d item(s) to %s",
		StatusAdbCopyFailed:            "Copy failed: %v",
		StatusAdbNoSelection:           "Select a file or folder to copy",
		StatusAdbSelectOnline:          "Select an online device",
	},
}

// T returns the translated string for key in lang.
func T(lang Lang, key Key, args ...any) string {
	lang = Normalize(lang)
	tmpl, ok := catalogs[lang][key]
	if !ok || tmpl == "" {
		tmpl = catalogs[Default][key]
	}
	if tmpl == "" {
		return string(key)
	}
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}
