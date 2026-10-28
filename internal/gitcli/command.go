package gitcli

import "strings"

// FormatCommand renders the git command a person ran.
// Implementation flags such as --no-pager, -C, and GIT_EDITOR are not included.
// An argument that contains whitespace or a quote is wrapped so the line stays readable.
func FormatCommand(args []string) string {
	var b strings.Builder
	b.WriteString("git")
	for _, a := range args {
		b.WriteByte(' ')
		b.WriteString(quoteGitArg(a))
	}
	return b.String()
}

func quoteGitArg(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\r\n'\"") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
