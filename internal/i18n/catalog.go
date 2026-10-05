package i18n

import "fmt"

// Key identifies a translatable string.
type Key string

const (
	AppTitle        Key = "app.title"
	SidebarCollapse Key = "sidebar.collapse"
	SidebarExpand   Key = "sidebar.expand"
	ToolImage       Key = "tool.image"
	ToolScreenshot  Key = "tool.screenshot"
	ToolAndroid     Key = "tool.android"
	ToolAndroidShot Key = "tool.android_shot"
	ToolStopwatch   Key = "tool.stopwatch"
	ToolGit         Key = "tool.git"
	ToolMTP         Key = "tool.mtp"

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
	PreviewFit     Key = "preview.fit"

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

	AndroidDevices          Key = "android.devices"
	AndroidFiles            Key = "android.files"
	AndroidRefresh          Key = "android.refresh"
	AndroidUp               Key = "android.up"
	AndroidPull             Key = "android.pull"
	AndroidPushFile         Key = "android.push_file"
	AndroidPushFolder       Key = "android.push_folder"
	AndroidEmpty            Key = "android.empty"
	AndroidColName          Key = "android.col_name"
	AndroidColSize          Key = "android.col_size"
	AndroidColModified      Key = "android.col_modified"
	AndroidNoDevices        Key = "android.no_devices"
	AndroidHint             Key = "android.hint"
	AndroidShotCapture      Key = "android_shot.capture"
	AndroidShotHint         Key = "android_shot.hint"
	AndroidShotEmpty        Key = "android_shot.empty"
	AndroidShotLive         Key = "android_shot.live"
	AndroidShotLiveWait     Key = "android_shot.live_wait"
	AndroidShotLiveDownload Key = "android_shot.live_download"
	AndroidShotFill         Key = "android_shot.fill"
	AndroidShotRestore      Key = "android_shot.restore"

	GitOpen          Key = "git.open"
	GitRecent        Key = "git.recent"
	GitNewTab        Key = "git.new_tab"
	GitReload        Key = "git.reload"
	GitFetch         Key = "git.fetch"
	GitPull          Key = "git.pull"
	GitPush          Key = "git.push"
	GitCommit        Key = "git.commit"
	GitGraph         Key = "git.graph"
	GitAmend         Key = "git.amend"
	GitCancel        Key = "git.cancel"
	GitDelete        Key = "git.delete"
	GitDeleteConfirm Key = "git.delete_confirm"
	GitRename        Key = "git.rename"
	GitRenamePrompt  Key = "git.rename_prompt"
	GitBranches      Key = "git.branches"
	GitTags          Key = "git.tags"
	GitCreateTag     Key = "git.create_tag"
	GitTagPrompt     Key = "git.tag_prompt"
	GitPushTag       Key = "git.push_tag"
	GitRemoteDelete  Key = "git.remote_delete"
	GitTagDeleteAsk  Key = "git.tag_delete_ask"
	GitTagRemoteAsk  Key = "git.tag_remote_ask"
	GitDetails       Key = "git.details"
	GitMessage       Key = "git.message"
	GitStageAll      Key = "git.stage_all"
	GitStage         Key = "git.stage"
	GitUnstage       Key = "git.unstage"
	GitEmpty         Key = "git.empty"
	GitNoCommits     Key = "git.no_commits"
	GitNoRepo        Key = "git.no_repo"
	GitHint          Key = "git.hint"
	GitDetached      Key = "git.detached"
	GitWorkingTree   Key = "git.working_tree"
	GitUncommitted   Key = "git.uncommitted"
	GitAheadBehind   Key = "git.ahead_behind"
	GitRemotes       Key = "git.remotes"
	GitAuthor        Key = "git.author"
	GitRefs          Key = "git.refs"
	GitSHA           Key = "git.sha"
	GitParents       Key = "git.parents"
	GitChanges       Key = "git.changes"
	GitChangeContent Key = "git.change_content"
	GitNoChanges     Key = "git.no_changes"

	StopwatchStart   Key = "stopwatch.start"
	StopwatchPause   Key = "stopwatch.pause"
	StopwatchReset   Key = "stopwatch.reset"
	StopwatchCopy    Key = "stopwatch.copy"
	StopwatchHint    Key = "stopwatch.hint"
	StopwatchStopped Key = "stopwatch.stopped"
	StopwatchRunning Key = "stopwatch.running"
	StopwatchPaused  Key = "stopwatch.paused"

	MTPDevices       Key = "mtp.devices"
	MTPRefresh       Key = "mtp.refresh"
	MTPUp            Key = "mtp.up"
	MTPPull          Key = "mtp.pull"
	MTPPushFile      Key = "mtp.push_file"
	MTPPushFolder    Key = "mtp.push_folder"
	MTPEmpty         Key = "mtp.empty"
	MTPColName       Key = "mtp.col_name"
	MTPColSize       Key = "mtp.col_size"
	MTPColModified   Key = "mtp.col_modified"
	MTPNoDevices     Key = "mtp.no_devices"
	MTPHint          Key = "mtp.hint"
	MTPCancel        Key = "mtp.cancel"
	DialogOpenDir    Key = "dialog.open_dir"
	DialogSaveAny    Key = "dialog.save_any"
	DialogOpenAny    Key = "dialog.open_any"
	DialogOpenFolder Key = "dialog.open_folder"

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
	StatusAdbDeviceReady           Key = "status.adb_device_ready"
	StatusAdbCapturing             Key = "status.adb_capturing"
	StatusAdbCaptureFailed         Key = "status.adb_capture_failed"
	StatusAdbLiveStarting          Key = "status.adb_live_starting"
	StatusAdbOpenH264Download      Key = "status.adb_openh264_download"
	StatusAdbLive                  Key = "status.adb_live"
	StatusAdbLiveH264              Key = "status.adb_live_h264"
	StatusAdbLiveFailed            Key = "status.adb_live_failed"
	StatusAdbLiveStopped           Key = "status.adb_live_stopped"
	StatusMTPConnectFailed         Key = "status.mtp_connect_failed"
	StatusMTPNoDevices             Key = "status.mtp_no_devices"
	StatusMTPDeviceOffline         Key = "status.mtp_device_offline"
	StatusMTPListing               Key = "status.mtp_listing"
	StatusMTPListingProgress       Key = "status.mtp_listing_progress"
	StatusMTPListed                Key = "status.mtp_listed"
	StatusMTPListFailed            Key = "status.mtp_list_failed"
	StatusMTPCopying               Key = "status.mtp_copying"
	StatusMTPCopyingPercent        Key = "status.mtp_copying_percent"
	StatusMTPCopyingProgress       Key = "status.mtp_copying_progress"
	StatusMTPCopied                Key = "status.mtp_copied"
	StatusMTPCopyFailed            Key = "status.mtp_copy_failed"
	StatusMTPCopyCancelled         Key = "status.mtp_copy_cancelled"
	StatusMTPNoSelection           Key = "status.mtp_no_selection"
	StatusMTPSelectOnline          Key = "status.mtp_select_online"

	StatusGitLoading        Key = "status.git_loading"
	StatusGitOpened         Key = "status.git_opened"
	StatusGitOpenFailed     Key = "status.git_open_failed"
	StatusGitWorking        Key = "status.git_working"
	StatusGitOpFailed       Key = "status.git_op_failed"
	StatusGitCommitOk       Key = "status.git_commit_ok"
	StatusGitCommitFailed   Key = "status.git_commit_failed"
	StatusGitAmendOk        Key = "status.git_amend_ok"
	StatusGitAmendFailed    Key = "status.git_amend_failed"
	StatusGitStageOk        Key = "status.git_stage_ok"
	StatusGitStageFailed    Key = "status.git_stage_failed"
	StatusGitUnstageOk      Key = "status.git_unstage_ok"
	StatusGitUnstageFailed  Key = "status.git_unstage_failed"
	StatusGitFetchOk        Key = "status.git_fetch_ok"
	StatusGitFetchFailed    Key = "status.git_fetch_failed"
	StatusGitPullOk         Key = "status.git_pull_ok"
	StatusGitPullFailed     Key = "status.git_pull_failed"
	StatusGitPushOk         Key = "status.git_push_ok"
	StatusGitPushFailed     Key = "status.git_push_failed"
	StatusGitCheckoutOk     Key = "status.git_checkout_ok"
	StatusGitCheckoutFailed Key = "status.git_checkout_failed"
	StatusGitCheckoutDirty  Key = "status.git_checkout_dirty"
	StatusGitTagSwitchOk    Key = "status.git_tag_switch_ok"
	StatusGitTagSwitchErr   Key = "status.git_tag_switch_err"
	StatusGitDeleteOk       Key = "status.git_delete_ok"
	StatusGitDeleteFailed   Key = "status.git_delete_failed"
	StatusGitDeleteCurrent  Key = "status.git_delete_current"
	StatusGitRenameOk       Key = "status.git_rename_ok"
	StatusGitRenameFailed   Key = "status.git_rename_failed"
	StatusGitTagOk          Key = "status.git_tag_ok"
	StatusGitTagFailed      Key = "status.git_tag_failed"
	StatusGitTagPushFailed  Key = "status.git_tag_push_failed"
	StatusGitTagDeleted     Key = "status.git_tag_deleted"
	StatusGitTagDeleteErr   Key = "status.git_tag_delete_err"
	StatusGitTagPushed      Key = "status.git_tag_pushed"
	StatusGitTagPushErr     Key = "status.git_tag_push_err"
	StatusGitTagRemoteOk    Key = "status.git_tag_remote_ok"
	StatusGitTagRemoteErr   Key = "status.git_tag_remote_err"
	StatusGitNeedMessage    Key = "status.git_need_message"
	StatusGitNoChanges      Key = "status.git_no_changes"
)

var catalogs = map[Lang]map[Key]string{
	JA: {
		AppTitle:                       "道具箱",
		SidebarCollapse:                "サイドバーを折りたたむ（%s）",
		SidebarExpand:                  "サイドバーを開く（%s）",
		ToolImage:                      "画像",
		ToolScreenshot:                 "画面キャプチャ",
		ToolAndroid:                    "Android ファイル",
		ToolAndroidShot:                "Android 画面",
		ToolStopwatch:                  "ストップウォッチ",
		ToolGit:                        "Git",
		ToolMTP:                        "MTP ファイル",
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
		PreviewFit:                     "全体",
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
		AndroidHint:                    "ADB プロトコルで端末のファイルを閲覧・コピーします。",
		AndroidShotCapture:             "キャプチャ",
		AndroidShotHint:                "ADB でライブ表示・撮影します。",
		AndroidShotEmpty:               "ライブを開始するか、キャプチャするか、左のリストから画像を選んでください。",
		AndroidShotLive:                "ライブ",
		AndroidShotLiveWait:            "端末の画面を取得しています…",
		AndroidShotLiveDownload:        "OpenH264 をダウンロードしています… %d%%",
		AndroidShotFill:                "いっぱいに表示",
		AndroidShotRestore:             "元に戻す",
		StopwatchStart:                 "スタート",
		StopwatchPause:                 "一時停止",
		StopwatchReset:                 "リセット",
		StopwatchCopy:                  "コピー",
		StopwatchHint:                  "スペースでスタート / 一時停止。一時停止中は %s でコピー、R でリセット",
		StopwatchStopped:               "停止中",
		StopwatchRunning:               "計測中",
		StopwatchPaused:                "一時停止",
		GitOpen:                        "リポジトリを開く",
		GitRecent:                      "最近開いたリポジトリ",
		GitNewTab:                      "新しいタブ",
		GitReload:                      "再読み込み",
		GitFetch:                       "Fetch",
		GitPull:                        "Pull",
		GitPush:                        "Push",
		GitCommit:                      "コミット",
		GitGraph:                       "Graph",
		GitAmend:                       "Amend",
		GitCancel:                      "キャンセル",
		GitDelete:                      "削除",
		GitDeleteConfirm:               "ブランチ「%s」を削除しますか？",
		GitRename:                      "名前を変更",
		GitRenamePrompt:                "新しいブランチ名",
		GitBranches:                    "ブランチ",
		GitTags:                        "タグ",
		GitCreateTag:                   "タグを作成",
		GitTagPrompt:                   "タグ名",
		GitPushTag:                     "リモートにプッシュ",
		GitRemoteDelete:                "リモートから削除",
		GitTagDeleteAsk:                "タグ「%s」を削除しますか？",
		GitTagRemoteAsk:                "リモートからタグ「%s」を削除しますか？",
		GitDetails:                     "コミット詳細",
		GitMessage:                     "コミットメッセージ",
		GitStageAll:                    "変更をすべて含める",
		GitStage:                       "Stage",
		GitUnstage:                     "Unstage",
		GitEmpty:                       "Git リポジトリのフォルダを開いてください。",
		GitNoCommits:                   "表示するコミットがありません。",
		GitNoRepo:                      "リポジトリが開かれていません",
		GitHint:                        "ブランチをクリックすると先端のコミットを表示。グラフのブランチをダブルクリックすると切り替え。タグをダブルクリックすると切り替え。右クリックで名前の変更と削除。コミットをクリックして詳細。チェックを外すと remote をグラフから隠します。",
		GitDetached:                    "detached HEAD",
		GitWorkingTree:                 "作業ツリー",
		GitUncommitted:                 "未コミットの変更",
		GitAheadBehind:                 "ahead %d / behind %d",
		GitRemotes:                     "リモート",
		GitAuthor:                      "作者",
		GitRefs:                        "参照",
		GitSHA:                         "SHA",
		GitParents:                     "親",
		GitChanges:                     "変更",
		GitChangeContent:               "変更内容",
		GitNoChanges:                   "変更はありません。",
		MTPDevices:                     "デバイス",
		MTPRefresh:                     "再読み込み",
		MTPUp:                          "上へ",
		MTPPull:                        "PCへコピー",
		MTPPushFile:                    "PCのファイルをコピー",
		MTPPushFolder:                  "PCのフォルダをコピー",
		MTPEmpty:                       "フォルダは空です。端末をファイル転送（MTP）モードにして接続してください。",
		MTPColName:                     "名前",
		MTPColSize:                     "サイズ",
		MTPColModified:                 "更新日時",
		MTPNoDevices:                   "接続中の MTP デバイスはありません",
		MTPHint:                        "USB の MTP で端末のファイルを閲覧・コピーします。USB デバッグは不要です。",
		MTPCancel:                      "キャンセル",
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
		StatusAdbDeviceReady:           "%s を使います",
		StatusAdbCapturing:             "端末の画面を撮影しています…",
		StatusAdbCaptureFailed:         "画面の撮影に失敗しました: %v",
		StatusAdbLiveStarting:          "ライブプレビューを開始しています…",
		StatusAdbOpenH264Download:      "OpenH264 をダウンロードしています… %d%%",
		StatusAdbLive:                  "ライブプレビュー中（%d×%d）",
		StatusAdbLiveH264:              "ライブプレビュー中（%d×%d、H.264）",
		StatusAdbLiveFailed:            "ライブプレビューに失敗しました: %v",
		StatusAdbLiveStopped:           "ライブプレビューを停止しました",
		StatusMTPConnectFailed:         "MTP デバイスを開けません: %v",
		StatusMTPNoDevices:             "MTP デバイスがありません。ファイル転送モードで USB 接続してください",
		StatusMTPDeviceOffline:         "このデバイスはまだ使えません（%s）",
		StatusMTPListing:               "読み込んでいます…",
		StatusMTPListingProgress:       "読み込んでいます… %d%%（%d / %d 件）",
		StatusMTPListed:                "%s を表示しています（%d 件）",
		StatusMTPListFailed:            "一覧を取得できません: %v",
		StatusMTPCopying:               "コピーしています…",
		StatusMTPCopyingPercent:        "コピーしています… %d%%",
		StatusMTPCopyingProgress:       "コピーしています… %d%%（%d / %d 件）",
		StatusMTPCopied:                "コピーしました（%d 件）: %s",
		StatusMTPCopyFailed:            "コピーに失敗しました: %v",
		StatusMTPCopyCancelled:         "コピーをキャンセルしました",
		StatusMTPNoSelection:           "コピーするファイルまたはフォルダを選んでください",
		StatusMTPSelectOnline:          "ストレージまたはフォルダを選んでください",
		StatusGitLoading:               "読み込んでいます…",
		StatusGitOpened:                "%s を開きました（コミット %d 件）",
		StatusGitOpenFailed:            "リポジトリを開けません: %v",
		StatusGitWorking:               "Git を実行しています…",
		StatusGitOpFailed:              "Git の操作に失敗しました: %v",
		StatusGitCommitOk:              "コミットしました",
		StatusGitCommitFailed:          "コミットに失敗しました: %v",
		StatusGitAmendOk:               "amend しました",
		StatusGitAmendFailed:           "amend に失敗しました: %v",
		StatusGitStageOk:               "ステージしました",
		StatusGitStageFailed:           "ステージに失敗しました: %v",
		StatusGitUnstageOk:             "アンステージしました",
		StatusGitUnstageFailed:         "アンステージに失敗しました: %v",
		StatusGitFetchOk:               "fetch しました",
		StatusGitFetchFailed:           "fetch に失敗しました: %v",
		StatusGitPullOk:                "pull しました",
		StatusGitPullFailed:            "pull に失敗しました: %v",
		StatusGitPushOk:                "push しました",
		StatusGitPushFailed:            "push に失敗しました: %v",
		StatusGitCheckoutOk:            "ブランチを切り替えました",
		StatusGitCheckoutFailed:        "ブランチの切り替えに失敗しました: %v",
		StatusGitCheckoutDirty:         "作業ツリーに変更があるため、ブランチを切り替えられません",
		StatusGitTagSwitchOk:           "タグに切り替えました",
		StatusGitTagSwitchErr:          "タグへの切り替えに失敗しました: %v",
		StatusGitDeleteOk:              "ブランチを削除しました",
		StatusGitDeleteFailed:          "ブランチの削除に失敗しました: %v",
		StatusGitDeleteCurrent:         "チェックアウト中のブランチは削除できません",
		StatusGitRenameOk:              "ブランチ名を変更しました",
		StatusGitRenameFailed:          "ブランチ名の変更に失敗しました: %v",
		StatusGitTagOk:                 "タグを作成しました",
		StatusGitTagFailed:             "タグの作成に失敗しました: %v",
		StatusGitTagPushFailed:         "タグは作成しましたが、リモートへの送信に失敗しました: %v",
		StatusGitTagDeleted:            "タグを削除しました",
		StatusGitTagDeleteErr:          "タグの削除に失敗しました: %v",
		StatusGitTagPushed:             "タグをプッシュしました",
		StatusGitTagPushErr:            "タグのプッシュに失敗しました: %v",
		StatusGitTagRemoteOk:           "リモートからタグを削除しました",
		StatusGitTagRemoteErr:          "リモートからのタグ削除に失敗しました: %v",
		StatusGitNeedMessage:           "コミットメッセージを入力してください",
		StatusGitNoChanges:             "コミットする変更がありません",
	},
	EN: {
		AppTitle:                       "Dogubako",
		SidebarCollapse:                "Collapse sidebar (%s)",
		SidebarExpand:                  "Expand sidebar (%s)",
		ToolImage:                      "Image",
		ToolScreenshot:                 "Screenshot",
		ToolAndroid:                    "Android Files",
		ToolAndroidShot:                "Android Screen",
		ToolStopwatch:                  "Stopwatch",
		ToolGit:                        "Git",
		ToolMTP:                        "MTP Files",
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
		PreviewFit:                     "Fit",
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
		AndroidHint:                    "Browse and copy device files over the ADB protocol.",
		AndroidShotCapture:             "Capture",
		AndroidShotHint:                "Live preview and capture over ADB.",
		AndroidShotEmpty:               "Start live preview, capture a screenshot, or choose one from the list.",
		AndroidShotLive:                "Live",
		AndroidShotLiveWait:            "Fetching the device screen…",
		AndroidShotLiveDownload:        "Downloading OpenH264… %d%%",
		AndroidShotFill:                "Fill area",
		AndroidShotRestore:             "Restore",
		StopwatchStart:                 "Start",
		StopwatchPause:                 "Pause",
		StopwatchReset:                 "Reset",
		StopwatchCopy:                  "Copy",
		StopwatchHint:                  "Space to start / pause. When paused, copy with %s, reset with R",
		StopwatchStopped:               "Stopped",
		StopwatchRunning:               "Running",
		StopwatchPaused:                "Paused",
		GitOpen:                        "Open Repository",
		GitRecent:                      "Recent repositories",
		GitNewTab:                      "New Tab",
		GitReload:                      "Reload",
		GitFetch:                       "Fetch",
		GitPull:                        "Pull",
		GitPush:                        "Push",
		GitCommit:                      "Commit",
		GitGraph:                       "Graph",
		GitAmend:                       "Amend",
		GitCancel:                      "Cancel",
		GitDelete:                      "Delete",
		GitDeleteConfirm:               "Delete branch \"%s\"?",
		GitRename:                      "Rename",
		GitRenamePrompt:                "New branch name",
		GitBranches:                    "Branches",
		GitTags:                        "Tags",
		GitCreateTag:                   "Create tag",
		GitTagPrompt:                   "Tag name",
		GitPushTag:                     "Push to remote",
		GitRemoteDelete:                "Delete from remote",
		GitTagDeleteAsk:                "Delete tag \"%s\"?",
		GitTagRemoteAsk:                "Delete tag \"%s\" from the remote?",
		GitDetails:                     "Commit details",
		GitMessage:                     "Commit message",
		GitStageAll:                    "Include all changes",
		GitStage:                       "Stage",
		GitUnstage:                     "Unstage",
		GitEmpty:                       "Open a folder that is a Git repository.",
		GitNoCommits:                   "No commits to show.",
		GitNoRepo:                      "No repository open",
		GitHint:                        "Click a branch to show its tip commit. Double-click a branch on the graph to switch. Double-click a tag to switch. Right-click a branch to rename or delete. Click a commit for details. Uncheck a remote to hide it from the graph.",
		GitDetached:                    "detached HEAD",
		GitWorkingTree:                 "Working tree",
		GitUncommitted:                 "Uncommitted changes",
		GitAheadBehind:                 "ahead %d / behind %d",
		GitRemotes:                     "Remotes",
		GitAuthor:                      "Author",
		GitRefs:                        "Refs",
		GitSHA:                         "SHA",
		GitParents:                     "Parents",
		GitChanges:                     "Changes",
		GitChangeContent:               "Changes",
		GitNoChanges:                   "No changes.",
		MTPDevices:                     "Devices",
		MTPRefresh:                     "Reload",
		MTPUp:                          "Up",
		MTPPull:                        "Copy to PC",
		MTPPushFile:                    "Copy File from PC",
		MTPPushFolder:                  "Copy Folder from PC",
		MTPEmpty:                       "This folder is empty. Connect a device in file transfer (MTP) mode.",
		MTPColName:                     "Name",
		MTPColSize:                     "Size",
		MTPColModified:                 "Modified",
		MTPNoDevices:                   "No MTP devices connected",
		MTPHint:                        "Browse and copy files over USB MTP. USB debugging is not required.",
		MTPCancel:                      "Cancel",
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
		StatusAdbDeviceReady:           "Using %s",
		StatusAdbCapturing:             "Capturing the device screen…",
		StatusAdbCaptureFailed:         "Screen capture failed: %v",
		StatusAdbLiveStarting:          "Starting live preview…",
		StatusAdbOpenH264Download:      "Downloading OpenH264… %d%%",
		StatusAdbLive:                  "Live preview (%d×%d)",
		StatusAdbLiveH264:              "Live preview (%d×%d, H.264)",
		StatusAdbLiveFailed:            "Live preview failed: %v",
		StatusAdbLiveStopped:           "Live preview stopped",
		StatusMTPConnectFailed:         "Could not open the MTP device: %v",
		StatusMTPNoDevices:             "No MTP devices. Plug in a device in file transfer mode.",
		StatusMTPDeviceOffline:         "This device is not ready (%s)",
		StatusMTPListing:               "Loading…",
		StatusMTPListingProgress:       "Loading… %d%% (%d / %d)",
		StatusMTPListed:                "Showing %s (%d items)",
		StatusMTPListFailed:            "Could not list files: %v",
		StatusMTPCopying:               "Copying…",
		StatusMTPCopyingPercent:        "Copying… %d%%",
		StatusMTPCopyingProgress:       "Copying… %d%% (%d / %d)",
		StatusMTPCopied:                "Copied %d item(s) to %s",
		StatusMTPCopyFailed:            "Copy failed: %v",
		StatusMTPCopyCancelled:         "Copy cancelled",
		StatusMTPNoSelection:           "Select a file or folder to copy",
		StatusMTPSelectOnline:          "Select a storage or folder",
		StatusGitLoading:               "Loading…",
		StatusGitOpened:                "Opened %s (%d commits)",
		StatusGitOpenFailed:            "Could not open the repository: %v",
		StatusGitWorking:               "Running git…",
		StatusGitOpFailed:              "Git operation failed: %v",
		StatusGitCommitOk:              "Committed",
		StatusGitCommitFailed:          "Commit failed: %v",
		StatusGitAmendOk:               "Amended",
		StatusGitAmendFailed:           "Amend failed: %v",
		StatusGitStageOk:               "Staged",
		StatusGitStageFailed:           "Could not stage: %v",
		StatusGitUnstageOk:             "Unstaged",
		StatusGitUnstageFailed:         "Could not unstage: %v",
		StatusGitFetchOk:               "Fetched",
		StatusGitFetchFailed:           "Fetch failed: %v",
		StatusGitPullOk:                "Pulled",
		StatusGitPullFailed:            "Pull failed: %v",
		StatusGitPushOk:                "Pushed",
		StatusGitPushFailed:            "Push failed: %v",
		StatusGitCheckoutOk:            "Switched branch",
		StatusGitCheckoutFailed:        "Could not switch branch: %v",
		StatusGitCheckoutDirty:         "Cannot switch branch while the working tree has changes",
		StatusGitTagSwitchOk:           "Switched to the tag",
		StatusGitTagSwitchErr:          "Could not switch to the tag: %v",
		StatusGitDeleteOk:              "Deleted branch",
		StatusGitDeleteFailed:          "Could not delete branch: %v",
		StatusGitDeleteCurrent:         "Cannot delete the checked-out branch",
		StatusGitRenameOk:              "Renamed branch",
		StatusGitRenameFailed:          "Could not rename branch: %v",
		StatusGitTagOk:                 "Created tag",
		StatusGitTagFailed:             "Could not create tag: %v",
		StatusGitTagPushFailed:         "Created the tag, but could not push it: %v",
		StatusGitTagDeleted:            "Deleted tag",
		StatusGitTagDeleteErr:          "Could not delete tag: %v",
		StatusGitTagPushed:             "Pushed tag",
		StatusGitTagPushErr:            "Could not push tag: %v",
		StatusGitTagRemoteOk:           "Deleted the tag from the remote",
		StatusGitTagRemoteErr:          "Could not delete the tag from the remote: %v",
		StatusGitNeedMessage:           "Enter a commit message",
		StatusGitNoChanges:             "No changes to commit",
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
