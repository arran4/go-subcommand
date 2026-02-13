package stringdiff

import (
	"bytes"
	"fmt"
	"hash/adler32"
	"strings"
	"unicode"
)

type MaxLines int
type Term bool
type Interactive bool
type LineUpFunc func(left, right []string) []DiffRow

type DiffRow struct {
	Left   string
	Right  string
	Buffer string
}

func Diff(left, right string, ops ...any) string {
	maxLines := 1000
	term := false
	// interactive := false
	var lineUpFunc LineUpFunc

	for _, op := range ops {
		switch v := op.(type) {
		case MaxLines:
			maxLines = int(v)
		case Term:
			term = bool(v)
		case Interactive:
			// interactive = bool(v)
		case LineUpFunc:
			lineUpFunc = v
		}
	}

	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")

	var rows []DiffRow
	if lineUpFunc != nil {
		rows = lineUpFunc(leftLines, rightLines)
	} else {
		rows = alignLines(leftLines, rightLines, maxLines)
	}

	return formatOutput(rows, term)
}

func alignLines(left, right []string, maxLines int) []DiffRow {
	n, m := len(left), len(right)
	var rows []DiffRow

	i, j := 0, 0
	for i < n || j < m {
		// Exact match
		if i < n && j < m && left[i] == right[j] {
			rows = append(rows, DiffRow{Left: left[i], Right: right[j], Buffer: "=="})
			i++
			j++
			continue
		}

		// Lookahead
		bestOffset := -1
		side := 0 // 1 = right advanced (insertion), 2 = left advanced (deletion)

		// Check if left[i] appears later in right
		for k := 1; k < maxLines; k++ {
			if j+k < m && i < n && hashLine(left[i]) == hashLine(right[j+k]) {
				if left[i] == right[j+k] {
					bestOffset = k
					side = 1
					break
				}
			}
			if i+k < n && j < m && hashLine(left[i+k]) == hashLine(right[j]) {
				if left[i+k] == right[j] {
					bestOffset = k
					side = 2
					break
				}
			}
		}

		if bestOffset != -1 {
			if side == 1 {
				// Insertion on right
				for k := 0; k < bestOffset; k++ {
					rows = append(rows, DiffRow{Left: "", Right: right[j+k], Buffer: calculateBuffer("", right[j+k])})
				}
				j += bestOffset
			} else {
				// Deletion on left
				for k := 0; k < bestOffset; k++ {
					rows = append(rows, DiffRow{Left: left[i+k], Right: "", Buffer: calculateBuffer(left[i+k], "")})
				}
				i += bestOffset
			}
			continue
		}

		// No match found nearby, consume both
		lVal := ""
		if i < n {
			lVal = left[i]
			i++
		}
		rVal := ""
		if j < m {
			rVal = right[j]
			j++
		}
		rows = append(rows, DiffRow{Left: lVal, Right: rVal, Buffer: calculateBuffer(lVal, rVal)})
	}
	return rows
}

func hashLine(s string) uint32 {
	return adler32.Checksum([]byte(s))
}

func calculateBuffer(l, r string) string {
	if l == r {
		return "=="
	}
	if l == "" || r == "" {
		isSpace := true
		target := l + r
		for _, r := range target {
			if !unicode.IsSpace(r) {
				isSpace = false
				break
			}
		}
		if isSpace {
			return "1w"
		}
		return "1d"
	}

	lRunes := []rune(l)
	rRunes := []rune(r)

	blocks := getDiffBlocks(lRunes, rRunes)

	count := len(blocks)
	typeMask := 0 // 1=char, 2=space, 4=eol

	for _, b := range blocks {
		typeMask |= b.Mask
	}

	suffix := "d"
	if typeMask == 4 {
		suffix = "$"
	} else if typeMask == 2 {
		suffix = "w"
	} else if (typeMask&1) != 0 && (typeMask&2) != 0 {
		suffix = "q"
	} else if typeMask == 1 {
		suffix = "d"
	} else {
		suffix = "q"
	}

	return fmt.Sprintf("%d%s", count, suffix)
}

type DiffBlock struct {
	Mask int // 1=char, 2=space
}

func getDiffBlocks(s1, s2 []rune) []DiffBlock {
	m, n := len(s1), len(s2)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if s1[i-1] == s2[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				if dp[i-1][j] > dp[i][j-1] {
					dp[i][j] = dp[i-1][j]
				} else {
					dp[i][j] = dp[i][j-1]
				}
			}
		}
	}

	var blocks []DiffBlock
	i, j := m, n
	inDiff := false

	for i > 0 || j > 0 {
		isDiff := false
		var char rune

		if i > 0 && j > 0 && s1[i-1] == s2[j-1] {
			// Match
			i--
			j--
			inDiff = false
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			// Insertion (in s2)
			char = s2[j-1]
			j--
			isDiff = true
		} else {
			// Deletion (in s1)
			char = s1[i-1]
			i--
			isDiff = true
		}

		if isDiff {
			mask := 1
			if char == '\r' || char == '\n' {
				mask = 4
			} else if unicode.IsSpace(char) {
				mask = 2
			}

			if !inDiff {
				blocks = append(blocks, DiffBlock{Mask: mask})
				inDiff = true
			} else {
				idx := len(blocks) - 1
				blocks[idx].Mask |= mask
			}
		}
	}

	return blocks
}

func formatOutput(rows []DiffRow, term bool) string {
	var buf bytes.Buffer
	maxLeft := 0
	maxBuffer := 0
	maxRight := 0

	for _, r := range rows {
		if len(r.Left) > maxLeft {
			maxLeft = len(r.Left)
		}
		if len(r.Buffer) > maxBuffer {
			maxBuffer = len(r.Buffer)
		}
		if len(r.Right) > maxRight {
			maxRight = len(r.Right)
		}
	}

	for _, r := range rows {
		lPad := strings.Repeat(" ", maxLeft-len(r.Left))
		bPad := strings.Repeat(" ", maxBuffer-len(r.Buffer))

		// Colorize if term is true
		leftStr := r.Left
		rightStr := r.Right
		bufferStr := r.Buffer

		if term && r.Buffer != "==" {
			// Simple red/green
			// \033[31m is red, \033[32m is green, \033[0m reset
			leftStr = fmt.Sprintf("\033[31m%s\033[0m", leftStr)
			rightStr = fmt.Sprintf("\033[32m%s\033[0m", rightStr)
			bufferStr = fmt.Sprintf("\033[33m%s\033[0m", bufferStr) // Yellow buffer

			// Adjust padding calculation because ansi codes add length but not visible width
			// This is tricky. For now, assume fixed width matching works on content length.
			// But I added ansi codes to the string, so pad calculation was based on raw string.
			// It should be fine as long as I pad *before* adding colors or calculate pad based on stripped length.
			// Here I pad first.
		}

		line := fmt.Sprintf("%s%s | %s%s | %s", leftStr, lPad, bufferStr, bPad, rightStr)
		buf.WriteString(line)
		buf.WriteString("\n")
	}
	return buf.String()
}
