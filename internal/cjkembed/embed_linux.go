//go:build linux

package cjkembed

import "path/filepath"

func defaultCJKPaths() []string {
	// Japanese-only OTFs first: Super OTC (JP+KR+SC+TC+HK) is much larger
	// and gogpu keeps a copy of whatever LoadFont is given.
	return []string{
		filepath.Join("/usr/share/fonts/opentype/noto/NotoSansCJKjp-Regular.otf"),
		filepath.Join("/usr/share/fonts/opentype/noto-cjk/NotoSansCJKjp-Regular.otf"),
		filepath.Join("/usr/share/fonts/noto-cjk/NotoSansCJKjp-Regular.otf"),
		filepath.Join("/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"),
		filepath.Join("/usr/share/fonts/opentype/noto-cjk/NotoSansCJK-Regular.ttc"),
		filepath.Join("/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc"),
		filepath.Join("/usr/share/fonts/google-noto-cjk/NotoSansCJK-Regular.ttc"),
		filepath.Join("/usr/share/fonts/truetype/noto-cjk/NotoSansCJK-Regular.ttc"),
		filepath.Join("/usr/share/fonts/truetype/wqy/wqy-microhei.ttc"),
		filepath.Join("/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf"),
	}
}
