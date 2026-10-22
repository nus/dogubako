package app

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/guigui-gui/guigui/basicwidget"
)

// Syntax colors are derived locally from the diff text already on screen.
// The repository host does not run a highlighter.

// synKind is one colored token class. Plain text stays at the default color.
type synKind uint8

const (
	synNone synKind = iota
	synKeyword
	synString
	synComment
	synNumber
	synType
	synIdent
)

// synSpan is a token in a displayed diff body, in byte offsets.
type synSpan struct {
	start, end int
	kind       synKind
}

// diffSyntax caches token spans and the painted body styles.
// A theme change repaints. It does not lex the file again.
type diffSyntax struct {
	shown                string
	split                bool
	path                 string
	dark                 bool
	ready                bool
	compact, left, right []synSpan
	compactStyle         basicwidget.TextStyles
	leftStyle            basicwidget.TextStyles
	rightStyle           basicwidget.TextStyles
}

type syntaxSide uint8

const (
	syntaxCompact syntaxSide = iota
	syntaxOld
	syntaxNew
)

// go.mod matches the AMPL lexer (*.mod) unless a more specific name wins.
var goModLexer = chroma.MustNewLexer(
	&chroma.Config{
		Name:      "Go Module",
		Aliases:   []string{"go.mod", "gomod", "gosum", "gowork"},
		Filenames: []string{"go.mod", "go.sum", "go.work"},
		Priority:  2,
	},
	func() chroma.Rules {
		return chroma.Rules{
			"root": {
				{Pattern: `\s+`, Type: chroma.Text},
				{Pattern: `//[^\n]*`, Type: chroma.Comment},
				{Pattern: `"(?:\\.|[^"\\])*"`, Type: chroma.LiteralString},
				{Pattern: chroma.Words(``, `\b`, `module`, `go`, `require`, `replace`, `exclude`, `retract`, `toolchain`, `godebug`, `ignore`, `tool`, `use`), Type: chroma.Keyword},
				{Pattern: `\b\d+\.\d+(?:\.\d+)?\b`, Type: chroma.LiteralNumber},
				{Pattern: `[^\s]+`, Type: chroma.Text},
			},
		}
	},
)

func init() {
	lexers.Register(goModLexer)
}

func (v *gitDiffView) compactBodyStyle() basicwidget.TextStyles {
	v.ensureSyntax()
	return v.syntax.compactStyle
}

func (v *gitDiffView) splitBodyStyle(left bool) basicwidget.TextStyles {
	v.ensureSyntax()
	if left {
		return v.syntax.leftStyle
	}
	return v.syntax.rightStyle
}

func (v *gitDiffView) ensureSyntax() {
	if v.syntax.ready && v.syntax.shown == v.shown && v.syntax.split == v.split && v.syntax.path == v.filePath && v.syntax.dark == v.dark {
		return
	}
	spansDirty := !v.syntax.ready || v.syntax.shown != v.shown || v.syntax.split != v.split || v.syntax.path != v.filePath
	v.syntax.shown = v.shown
	v.syntax.split = v.split
	v.syntax.path = v.filePath
	v.syntax.dark = v.dark
	v.syntax.ready = true
	if spansDirty {
		v.syntax.compact = nil
		v.syntax.left = nil
		v.syntax.right = nil
		if v.shown != "" && !diffIsBinary(v.shown) && !diffIsBinary(v.text) {
			oldPath, newPath := v.syntaxPaths()
			if v.split {
				v.syntax.left = syntaxSpans(v.leftBody, v.leftKinds, oldPath, newPath, syntaxOld)
				v.syntax.right = syntaxSpans(v.rightBody, v.rightKinds, oldPath, newPath, syntaxNew)
			} else {
				v.syntax.compact = syntaxSpans(v.compactBody, v.compactKinds, oldPath, newPath, syntaxCompact)
			}
		}
	}
	v.syntax.compactStyle = basicwidget.TextStyles{}
	v.syntax.leftStyle = basicwidget.TextStyles{}
	v.syntax.rightStyle = basicwidget.TextStyles{}
	if v.split {
		left := styleSplit(v.leftBody, v.leftKinds, v.dark, false)
		right := styleSplit(v.rightBody, v.rightKinds, v.dark, false)
		paintSyntax(&left, v.syntax.left, v.dark)
		paintSyntax(&right, v.syntax.right, v.dark)
		v.syntax.leftStyle = left
		v.syntax.rightStyle = right
		return
	}
	var styles basicwidget.TextStyles
	if v.compactMarks != "" {
		styles = styleSplit(v.compactBody, v.compactKinds, v.dark, false)
	} else {
		styles = styleDiff(v.compactBody, v.dark)
	}
	paintSyntax(&styles, v.syntax.compact, v.dark)
	v.syntax.compactStyle = styles
}

// syntaxPaths reads the file names from the diff header.
// A selected path fills in only when the header names neither side,
// so a new or deleted file keeps the /dev/null side unnamed.
func (v *gitDiffView) syntaxPaths() (oldPath, newPath string) {
	oldPath, newPath = diffSidePaths(v.shown)
	if oldPath == "" && newPath == "" && v.filePath != "" {
		return v.filePath, v.filePath
	}
	return oldPath, newPath
}

func diffSidePaths(text string) (oldPath, newPath string) {
	sawHeader := false
	inHunk := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			if sawHeader {
				return oldPath, newPath
			}
			sawHeader = true
			inHunk = false
			oldPath, newPath = pathsFromDiffGit(line)
			continue
		}
		if _, _, ok := parseHunkStarts(line); ok {
			inHunk = true
			continue
		}
		if inHunk {
			continue
		}
		switch {
		case strings.HasPrefix(line, "--- "):
			if p, ok := syntaxDiffPath(line[len("--- "):]); ok {
				oldPath = p
			} else if strings.TrimSpace(line[len("--- "):]) == "/dev/null" {
				oldPath = ""
			}
		case strings.HasPrefix(line, "+++ "):
			if p, ok := syntaxDiffPath(line[len("+++ "):]); ok {
				newPath = p
			} else if strings.TrimSpace(line[len("+++ "):]) == "/dev/null" {
				newPath = ""
			}
		}
	}
	return oldPath, newPath
}

func pathsFromDiffGit(line string) (oldPath, newPath string) {
	rest := strings.TrimPrefix(line, "diff --git ")
	a, b, ok := splitDiffGitPaths(rest)
	if !ok {
		return "", ""
	}
	oldPath, _ = syntaxDiffPath(a)
	newPath, _ = syntaxDiffPath(b)
	return oldPath, newPath
}

func splitDiffGitPaths(rest string) (a, b string, ok bool) {
	a, rest, ok = readGitPath(rest)
	if !ok {
		return "", "", false
	}
	b, rest, ok = readGitPath(rest)
	if !ok || strings.TrimSpace(rest) != "" {
		return "", "", false
	}
	return a, b, true
}

func readGitPath(s string) (path, rest string, ok bool) {
	s = strings.TrimLeft(s, " ")
	if s == "" {
		return "", "", false
	}
	if s[0] != '"' {
		i := strings.IndexByte(s, ' ')
		if i < 0 {
			return s, "", true
		}
		return s[:i], s[i:], true
	}
	for i := 1; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			continue
		}
		if s[i] == '"' {
			return s[:i+1], s[i+1:], true
		}
	}
	return "", "", false
}

func syntaxDiffPath(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' {
		u, err := strconv.Unquote(s)
		if err != nil {
			return "", false
		}
		s = u
	}
	if s == "" || s == "/dev/null" {
		return "", false
	}
	if strings.HasPrefix(s, "a/") || strings.HasPrefix(s, "b/") {
		s = s[len("a/"):]
	}
	if s == "" || s == "/dev/null" {
		return "", false
	}
	return s, true
}

func syntaxSpans(body string, kinds []byte, oldPath, newPath string, side syntaxSide) []synSpan {
	if body == "" || len(kinds) == 0 {
		return nil
	}
	oldLex := lexerFor(oldPath, newPath)
	newLex := lexerFor(newPath, oldPath)
	switch side {
	case syntaxOld:
		return highlightSide(body, kinds, oldLex, keepOld, emitKept)
	case syntaxNew:
		return highlightSide(body, kinds, newLex, keepNew, emitKept)
	default:
		// Deletions take the old file's tokens. Context and additions take the
		// new file's. Context is still fed to the old lexer so a comment that
		// opens above a deletion stays a comment, but those bytes are colored
		// from the new file, which is the copy the compact line shows.
		dels := highlightSide(body, kinds, oldLex, keepOld, emitDel)
		adds := highlightSide(body, kinds, newLex, keepNew, emitNew)
		return append(dels, adds...)
	}
}

func keepOld(kind byte) bool { return kind == ' ' || kind == '-' }
func keepNew(kind byte) bool { return kind == ' ' || kind == '+' }
func emitKept(byte) bool     { return true }
func emitDel(kind byte) bool { return kind == '-' }
func emitNew(kind byte) bool { return kind == ' ' || kind == '+' }

func lexerFor(path, fallback string) chroma.Lexer {
	if path != "" {
		return languageLexer(path)
	}
	return languageLexer(fallback)
}

// languageLexer picks a lexer from the file name. Content is not inspected:
// an extensionless file is colored only when its name is a known one.
func languageLexer(path string) chroma.Lexer {
	if path == "" || path == "/dev/null" {
		return nil
	}
	lexer := lexers.Match(path)
	if lexer == nil {
		return nil
	}
	if strings.EqualFold(lexer.Config().Name, "plaintext") {
		return nil
	}
	return chroma.Coalesce(lexer)
}

// srcSeg maps a run of source bytes onto the displayed body.
// emit is false for context that is included only so later tokens keep their state.
type srcSeg struct {
	src, body, n int
	emit         bool
}

func highlightSide(body string, kinds []byte, lexer chroma.Lexer, keep, emit func(byte) bool) []synSpan {
	if lexer == nil {
		return nil
	}
	var b strings.Builder
	var segs []srcSeg
	line := 0
	for start := 0; start < len(body); {
		end := diffLineEnd(body, start)
		var kind byte
		if line < len(kinds) {
			kind = kinds[line]
		}
		if keep(kind) {
			segs = append(segs, srcSeg{
				src:  b.Len(),
				body: start,
				n:    end - start,
				emit: emit(kind),
			})
			b.WriteString(body[start:end])
		}
		line++
		if end == len(body) {
			break
		}
		start = end
	}
	if b.Len() == 0 {
		return nil
	}
	return projectSpans(tokenSpans(lexer, b.String()), segs)
}

func tokenSpans(lexer chroma.Lexer, src string) []synSpan {
	it, err := lexer.Tokenise(&chroma.TokeniseOptions{State: "root", EnsureLF: false}, src)
	if err != nil || it == nil {
		return nil
	}
	var b strings.Builder
	b.Grow(len(src) + 1)
	var spans []synSpan
	for {
		t := it()
		if t == chroma.EOF {
			break
		}
		start := b.Len()
		b.WriteString(t.Value)
		if k := synKindOf(t.Type); k != synNone && t.Value != "" {
			spans = append(spans, synSpan{start: start, end: b.Len(), kind: k})
		}
	}
	got := b.String()
	if got != src && !(len(got) >= len(src) && got[:len(src)] == src && strings.Trim(got[len(src):], "\n") == "") {
		return nil
	}
	return clipSpans(spans, len(src))
}

func clipSpans(spans []synSpan, n int) []synSpan {
	out := spans[:0]
	for _, sp := range spans {
		if sp.start >= n {
			continue
		}
		if sp.end > n {
			sp.end = n
		}
		if sp.end > sp.start {
			out = append(out, sp)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func projectSpans(raw []synSpan, segs []srcSeg) []synSpan {
	var out []synSpan
	si := 0
	for _, sp := range raw {
		for si < len(segs) && segs[si].src+segs[si].n <= sp.start {
			si++
		}
		for j := si; j < len(segs) && segs[j].src < sp.end; j++ {
			seg := segs[j]
			if !seg.emit {
				continue
			}
			a := max(sp.start, seg.src)
			b := min(sp.end, seg.src+seg.n)
			if a >= b {
				continue
			}
			out = append(out, synSpan{
				start: seg.body + (a - seg.src),
				end:   seg.body + (b - seg.src),
				kind:  sp.kind,
			})
		}
	}
	return out
}

func synKindOf(t chroma.TokenType) synKind {
	switch t {
	case chroma.KeywordType, chroma.NameClass, chroma.NameException:
		return synType
	case chroma.NameFunction, chroma.NameFunctionMagic, chroma.NameDecorator,
		chroma.NameTag, chroma.NameAttribute, chroma.NameBuiltin, chroma.NameBuiltinPseudo,
		chroma.NameLabel, chroma.NameConstant:
		return synIdent
	}
	switch {
	case t.InCategory(chroma.Comment):
		return synComment
	case t.InCategory(chroma.Keyword):
		return synKeyword
	case t.InSubCategory(chroma.LiteralString):
		return synString
	case t.InSubCategory(chroma.LiteralNumber):
		return synNumber
	default:
		return synNone
	}
}

type syntaxColors struct {
	keyword, str, comment, number, typ, ident color.NRGBA
}

func (c syntaxColors) of(k synKind) color.NRGBA {
	switch k {
	case synKeyword:
		return c.keyword
	case synString:
		return c.str
	case synComment:
		return c.comment
	case synNumber:
		return c.number
	case synType:
		return c.typ
	case synIdent:
		return c.ident
	default:
		return color.NRGBA{}
	}
}

// syntaxPalette is dark enough on the light added and deleted washes,
// and light enough on the dark ones, including the page behind context lines.
func syntaxPalette(dark bool) syntaxColors {
	if dark {
		return syntaxColors{
			keyword: color.NRGBA{R: 0xff, G: 0x8b, B: 0x82, A: 0xff},
			str:     color.NRGBA{R: 0xb6, G: 0xe3, B: 0xff, A: 0xff},
			comment: color.NRGBA{R: 0xb7, G: 0xc0, B: 0xca, A: 0xff},
			number:  color.NRGBA{R: 0xf0, G: 0xd0, B: 0x70, A: 0xff},
			typ:     color.NRGBA{R: 0xc4, G: 0xb5, B: 0xfd, A: 0xff},
			ident:   color.NRGBA{R: 0xff, G: 0xb8, B: 0x6c, A: 0xff},
		}
	}
	return syntaxColors{
		keyword: color.NRGBA{R: 0x9b, G: 0x1c, B: 0x1c, A: 0xff},
		str:     color.NRGBA{R: 0x0b, G: 0x3a, B: 0x67, A: 0xff},
		comment: color.NRGBA{R: 0x3d, G: 0x44, B: 0x4d, A: 0xff},
		number:  color.NRGBA{R: 0x05, G: 0x50, B: 0xae, A: 0xff},
		typ:     color.NRGBA{R: 0x5e, G: 0x2c, B: 0xa5, A: 0xff},
		ident:   color.NRGBA{R: 0x7a, G: 0x3e, B: 0x00, A: 0xff},
	}
}

func paintSyntax(styles *basicwidget.TextStyles, spans []synSpan, dark bool) {
	if len(spans) == 0 {
		return
	}
	pal := syntaxPalette(dark)
	for _, sp := range spans {
		if sp.end <= sp.start {
			continue
		}
		styles.SetColorInRange(sp.start, sp.end, pal.of(sp.kind))
	}
}
