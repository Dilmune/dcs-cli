package workspace

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	logoWidth                = 32
	logoHeight               = 17
	welcomeColumnGap         = 4
	minimumWelcomeMenuWidth  = 38
	compactWelcomeColumnGap  = 2
	minimumCompactMenuWidth  = 20
	welcomeHeaderHeight      = 3
	welcomeBottomGap         = 1
	minimumWelcomeLogoWidth  = 16
	minimumWelcomeLogoHeight = 9
	maximumWelcomeLogoWidth  = 20
)

// Pre-sampled from the same approved artwork at terminal-cell resolutions.
// The standard 32-column mark stays exact; resizes do not stretch its pixels.
func logoRows(width int) []logoRow {
	switch width {
	case 16:
		return []logoRow{
			{upper: "       cc       ", lower: "      cccc      "},
			{upper: "    cc cc cc    ", lower: "   cccc  cccc   "},
			{upper: "cccc  aaaa  cccc", lower: "scc cc aa cc ccs"},
			{upper: "ss cccc  cccc ss", lower: " s scc cc ccs ss"},
			{upper: "s  ss cccc ss  c", lower: "ss  s sccsss  cc"},
			{upper: "as a  ssss  a c ", lower: "aa aaa ss aaa  a"},
			{upper: "aa  aaa  aaaa aa", lower: "   ss aaaaa a a "},
			{upper: "   ss aaa c a   ", lower: "    sss ccc     "},
			{upper: "      sscc      ", lower: "       sc       "},
		}
	case 18:
		return []logoRow{
			{upper: "        cc        ", lower: "       cccc       "},
			{upper: "     cc cc cc     ", lower: "    cccc  ccccc   "},
			{upper: " ccc cc aa cc ccc ", lower: "ccccc  aaaa  ccccc"},
			{upper: "ssc ccc aa ccc css", lower: "sssccccc  cccccsss"},
			{upper: " ssssc  cc ccssss ", lower: "s  sssccccccsss  c"},
			{upper: "ss  ssssccssss  cc", lower: " ssaa ssssss aacc "},
			{upper: "aa  aa ssss aaa  a", lower: "aa  aaa   aaaaa aa"},
			{upper: " a ssaaaaaaaaaaaa ", lower: "   ss  aaaa  aa   "},
			{upper: "    s s aa cca    ", lower: "      sssccc      "},
			{upper: "       sscc       ", lower: "        sc        "},
		}
	case 20:
		return []logoRow{
			{upper: "         cc         ", lower: "        cccc        "},
			{upper: "      c cccc c      ", lower: "    ccccc  ccccc    "},
			{upper: "  c  ccc aa ccc  c  ", lower: "ccccc   aaaa   ccccc"},
			{upper: "sccc  c aaaa c  cccs", lower: "sss cccc    cccc sss"},
			{upper: "sss cccc    cccc sss", lower: "  s ssc cccc css ss "},
			{upper: "ss  ss cccccc ss   c", lower: "ss   s ssccss ss ccc"},
			{upper: " ss a  ssssss  a cc ", lower: "aa  aa  ssss  aa c a"},
			{upper: "aaa aaaa ss aaaa  aa", lower: " aas aaaa  aaaaa aaa"},
			{upper: "   sss aaaaaa  a a  ", lower: "    ss  aaaa c a    "},
			{upper: "     s ss  ccc      ", lower: "       ssscccc      "},
			{upper: "        sscc        ", lower: "         sc         "},
		}
	case 24:
		return []logoRow{
			{upper: "           cc           ", lower: "          cccc          "},
			{upper: "         cccccc         ", lower: "      ccc  cc  ccc      "},
			{upper: "     cccccc  cccccc     ", lower: "  cc  ccc  aa cccc  cc  "},
			{upper: "cccccc   aaaaaa   cccccc", lower: "sccccc c aaaaaa   cccccs"},
			{upper: "sssc  cccc aa  ccc  csss", lower: "sss ccccccc  ccccccc sss"},
			{upper: " ss ssccc  cc  cccss sss", lower: "s s sssc cccccc csss s  "},
			{upper: "ss  ssssscccccc ssss  cc", lower: "ssc  ssssssccss sss  ccc"},
			{upper: " ss aa ssssssss s    ccc", lower: "a ss aa  ssssss  aaa c  "},
			{upper: "aa   aaa  ssss aaaaa  aa", lower: "aaa  aaaaa ss aaaaaa aaa"},
			{upper: " aa s  aaaa  aaaaaaa aaa", lower: "    sss aaaaaaaa  aa a  "},
			{upper: "    sss  aaaaa  c aa    ", lower: "     ss ss aa ccc a     "},
			{upper: "        sss  cccc       ", lower: "        sssscccc        "},
			{upper: "          sscc          ", lower: "           sc           "},
		}
	case 28:
		return []logoRow{
			{upper: "             cc             ", lower: "            ccccc           "},
			{upper: "          cccccccc          ", lower: "        cc  cccc  cc        "},
			{upper: "      cccccc cc cccccc      ", lower: "     ccccccc    ccccccc     "},
			{upper: "  ccc  cccc  aa  cccc  ccc  ", lower: "ccccccc    aaaaaa    ccccccc"},
			{upper: "scccccc    aaaaaa    ccccccs", lower: "ssscc  cccc aaaa cccc  ccsss"},
			{upper: "ssss ccccccc    ccccccc ssss", lower: "ssss scccccc    ccccccs ssss"},
			{upper: "  ss ssccc  cccc  cccss sss ", lower: "s  s ssss  cccccc  ssss ss c"},
			{upper: "ssc  ssss sccccccc ssss   cc", lower: "sss   sss ssccccss sss  cccc"},
			{upper: " ssc a  s ssssssss s    cccc", lower: "a ss aaa  ssssssss  aaa ccc "},
			{upper: "aa    aaa  ssssss  aaaa c  a", lower: "aaa   aaaaa ssss aaaaaa  aaa"},
			{upper: "aaaa   aaaaa    aaaaaaa aaaa", lower: " aaa sc aaaaaaaaaaaaaaa aaa "},
			{upper: "   a sss  aaaaaaaa   aa aa  ", lower: "     ssss  aaaaaa cc aa     "},
			{upper: "      sssss aaa  ccc aa     ", lower: "       sssss   ccccc        "},
			{upper: "         sssssccccc         ", lower: "          sssscccc          "},
			{upper: "            sscc            ", lower: "             sc             "},
		}
	case 32:
		return []logoRow{
			{upper: "               cc               ", lower: "              ccccc             "},
			{upper: "            cccccccc            ", lower: "          c  ccccccc            "},
			{upper: "        cccc  cccc  cccc        ", lower: "      cccccccc    cccccccc      "},
			{upper: "    c  ccccccc    ccccccc  c    ", lower: "  cccc  cccc  aaaa  cccc  cccc  "},
			{upper: " ccccccc    aaaaaaaa    cccccccc", lower: "sccccccc    aaaaaaaa    cccccccs"},
			{upper: "ssscccc  ccc  aaaa  ccc  ccccsss", lower: "ssssc  cccccc  aa  cccccc  cssss"},
			{upper: "sssss cccccccc    cccccccc sssss", lower: " ssss scccccc  cc  ccccccs sssss"},
			{upper: "  sss ssscc  cccccc cccsss ssss ", lower: "ss  s ssss  cccccccc  ssss ss  c"},
			{upper: "sss   ssss sccccccccs ssss   ccc", lower: "sssc   sss sssccccsss sss   cccc"},
			{upper: " sss  a ss ssssccssss ss   ccccc", lower: "a ss  aa   ssssssssss   aa cccc "},
			{upper: "aa  s aaaa sssssssss  aaaa cc  a", lower: "aaa    aaaa  ssssss  aaaaa    aa"},
			{upper: "aaaa   aaaaaa sss  aaaaaaa  aaaa", lower: "aaaa s  aaaaaa    aaaaaaaa aaaaa"},
			{upper: "  aa ssc aaaaaaaaaaaaaaaaa aaaa ", lower: "   a ssss  aaaaaaaaaa  aaa aa   "},
			{upper: "     sssss  aaaaaaaa c aaa      ", lower: "      ssss s  aaaa  cc aaa      "},
			{upper: "        ss sss a  cccc aa       ", lower: "         s ssss  ccccc          "},
			{upper: "           ssssscccccc          ", lower: "            sssscccc            "},
			{upper: "              sscc              ", lower: "               sc               "},
		}
	case 36:
		return []logoRow{
			{upper: "                 cc                 ", lower: "                cccc                "},
			{upper: "              cccccccc              ", lower: "             cccccccccc             "},
			{upper: "          ccc  ccccccc  cc          ", lower: "        cccccc  cccc  cccccc        "},
			{upper: "       ccccccccc    ccccccccc       ", lower: "    c  ccccccccc    ccccccccc  c    "},
			{upper: "   cccc  ccccc  aaaa  ccccc  cccc   ", lower: " cccccccc cc   aaaaaa   cc  ccccccc "},
			{upper: "cccccccccc   aaaaaaaaaa   cccccccccc", lower: "ssccccccc  c  aaaaaaaa  c  cccccccss"},
			{upper: "ssscccc  cccc  aaaaaa  cccc  ccccsss", lower: "sssss   ccccccc  aa  ccccccc  ccssss"},
			{upper: "sssss cccccccccc    cccccccccc sssss", lower: "sssss ssccccccc  cc  ccccccccs sssss"},
			{upper: "  sss sssccccc  cccc  cccccsss ssss ", lower: "c  ss sssssc  cccccccc  csssss sss  "},
			{upper: "ssc s sssss  cccccccccc ssssss s  cc", lower: "sssc  sssss  sccccccccs ssssss   ccc"},
			{upper: "ssss    sss  ssscccccss sssss  ccccc", lower: " sss     ss  ssssccssss sss    ccccc"},
			{upper: "  ssc aaa    ssssssssss s   a  ccccc", lower: "aa ss  aaa   ssssssssss   aaaa ccc  "},
			{upper: "aaa sc aaaa  ssssssssss aaaaaa c   a", lower: "aaaa   aaaaaa  ssssss  aaaaaaa   aaa"},
			{upper: "aaaaa   aaaaaa  ssss  aaaaaaaa aaaaa", lower: "aaaaa c  aaaaaaa    aaaaaaaaaa aaaaa"},
			{upper: " aaaa ss  aaaaaaa  aaaaaaaaaaa aaaa ", lower: "   aa sssc  aaaaaaaaaaaa  aaaa aaa  "},
			{upper: "      sssss  aaaaaaaaaa   aaaa a    ", lower: "      sssss c aaaaaaaa cc aaaa      "},
			{upper: "       ssss ss  aaaa  ccc aaa       ", lower: "         ss sss  aa ccccc aa        "},
			{upper: "          s sssss  cccccc           ", lower: "            ssssssccccccc           "},
			{upper: "             sssssccccc             ", lower: "              sssscccc              "},
			{upper: "                sscc                ", lower: "                 sc                 "},
		}
	case 40:
		return []logoRow{
			{upper: "                   cc                   ", lower: "                  cccc                  "},
			{upper: "                cccccccc                ", lower: "              cccccccccccc              "},
			{upper: "            c  cccccccccc  c            ", lower: "          ccccc  cccccc  ccccc          "},
			{upper: "        cccccccc   cc   ccccccc         ", lower: "       ccccccccccc    ccccccccccc       "},
			{upper: "     c  ccccccccc      ccccccccc  c     ", lower: "   cccc   cccccc  aaaa  cccccc   cccc   "},
			{upper: " ccccccccc  cc  aaaaaaaa  cc   cccccccc ", lower: "ccccccccccc   aaaaaaaaaaa    ccccccccccc"},
			{upper: "sscccccccc  c  aaaaaaaaaa     cccccccccs", lower: "sssccccc   cccc  aaaaaa  cccc   cccccsss"},
			{upper: "ssssscc  ccccccc  aaaa  ccccccc  ccsssss", lower: "ssssss  cccccccccc    cccccccccc  ssssss"},
			{upper: "ssssss scccccccccc    ccccccccccs ssssss", lower: " sssss ssccccccc  cccc  cccccccss ssssss"},
			{upper: "  ssss sssscccc  cccccc  ccccssss sssss ", lower: "ss  ss sssssc  cccccccccc  csssss sss   "},
			{upper: "sss  s ssssss cccccccccccc ssssss ss  cc", lower: "sssc   ssssss sscccccccccs ssssss    ccc"},
			{upper: "ssss    sssss sssccccccsss sssss   ccccc", lower: "ssssc     sss sssssccsssss sss    cccccc"},
			{upper: "  sss  aa  ss ssssssssssss ss  a  cccccc", lower: "a  ssc aaaa   ssssssssssss   aaaa ccccc "},
			{upper: "aa  ss aaaaa  ssssssssssss  aaaaa ccc  a", lower: "aaaa  c aaaaaa  ssssssss  aaaaaaa c   aa"},
			{upper: "aaaaa    aaaaaa  ssssss  aaaaaaaa   aaaa", lower: "aaaaa    aaaaaaa   ss   aaaaaaaaa aaaaaa"},
			{upper: " aaaa ss  aaaaaaaa    aaaaaaaaaaa aaaaaa", lower: "  aaa sssc  aaaaaaa  aaaaaaa aaaa aaaaa "},
			{upper: "    a ssssc  aaaaaaaaaaaaaa  aaaa aaaa  ", lower: "      ssssss  aaaaaaaaaaa    aaaa aa    "},
			{upper: "      ssssss    aaaaaaaa  cc aaaa       ", lower: "        ssss ss  aaaaaa  ccc aaaa       "},
			{upper: "         sss ssss aaa  ccccc aaa        ", lower: "          ss sssss    cccccc a          "},
			{upper: "             ssssssscccccccc            ", lower: "             sssssssccccccc             "},
			{upper: "               ssssscccccc              ", lower: "                sssscccc                "},
			{upper: "                  sscc                  ", lower: "                   sc                   "},
		}
	case 44:
		return []logoRow{
			{upper: "                     cc                     ", lower: "                    cccc                    "},
			{upper: "                  cccccccc                  ", lower: "                cccccccccccc                "},
			{upper: "                 ccccccccccc                ", lower: "            cccc  cccccccc  cccc            "},
			{upper: "          ccccccc   cccc   ccccccc          ", lower: "         cccccccccc  cc  cccccccccc         "},
			{upper: "        cccccccccccc    cccccccccccc        ", lower: "     cc   ccccccccc  aa  ccccccccc   cc     "},
			{upper: "   cccccc   cccc    aaaa   cccccc  cccccc   ", lower: " cccccccccc  cc   aaaaaaaa   cc  cccccccccc "},
			{upper: "cccccccccccc    aaaaaaaaaaaa    cccccccccccc", lower: "scccccccccc      aaaaaaaaaa      ccccccccccs"},
			{upper: "ssscccccc   cccc  aaaaaaaa   ccc   ccccccsss", lower: "sssssccc  ccccccc   aaaa   ccccccc  cccsssss"},
			{upper: "ssssss   cccccccccc  aa  cccccccccc   ssssss", lower: "ssssss  cccccccccccc    cccccccccccc  ssssss"},
			{upper: "ssssss  scccccccccc  cc  ccccccccccs  ssssss", lower: "  ssss  ssccccccc   cccc   cccccccss  ssssss"},
			{upper: "c  sss  sssscccc  cccccccc  ccccssss  ssss  ", lower: "ss   s  ssssss   cccccccccc   ssssss  sss  c"},
			{upper: "sssc    ssssss ccccccccccccc  ssssss  s   cc", lower: "ssss    ssssss ssccccccccccs  ssssss    cccc"},
			{upper: "ssssc    sssss sssscccccccss  sssss    ccccc", lower: "sssss      sss sssssccccssss  ssss    cccccc"},
			{upper: " ssssc  aa  ss sssssscssssss  ss  a   cccccc", lower: "a  sss  aaa    sssssssssssss     aaa  ccccc "},
			{upper: "aa  ssc aaaaa  sssssssssssss   aaaaa  ccc   ", lower: "aaaa  s  aaaaa  ssssssssssss  aaaaaa  c   aa"},
			{upper: "aaaaa    aaaaaa   ssssssss  aaaaaaaa    aaaa", lower: "aaaaaa    aaaaaaa   ssss   aaaaaaaaa   aaaaa"},
			{upper: "aaaaaa c  aaaaaaaaa  ss  aaaaaaaaaaa  aaaaaa", lower: " aaaaa ss  aaaaaaaaa    aaaaaaaaaaaa  aaaaaa"},
			{upper: "  aaaa sssc  aaaaaaaa aaaaaaaaa aaaa  aaaaa ", lower: "    aa ssssc   aaaaaaaaaaaaaaa  aaaa  aaa   "},
			{upper: "     a ssssss   aaaaaaaaaaaa    aaaa  aa    ", lower: "       sssssss   aaaaaaaaaa  cc aaaa        "},
			{upper: "        ssssss s  aaaaaaa   ccc aaaa        ", lower: "         sssss sss  aaaa  ccccc aaa         "},
			{upper: "           sss ssss  a   cccccc aa          ", lower: "            ss ssssss  cccccccc a           "},
			{upper: "               sssssssccccccccc             ", lower: "               sssssssccccccc               "},
			{upper: "                 sssssccccc                 ", lower: "                  sssscccc                  "},
			{upper: "                    sscc                    ", lower: "                     sc                     "},
		}
	case 48:
		return []logoRow{
			{upper: "                       cc                       ", lower: "                      ccccc                     "},
			{upper: "                    cccccccc                    ", lower: "                  cccccccccccc                  "},
			{upper: "                 cccccccccccccc                 ", lower: "              cc   cccccccccc   cc              "},
			{upper: "            cccccc  cccccccc  cccccc            ", lower: "          ccccccccc   cccc   cccccccc           "},
			{upper: "         cccccccccccc      cccccccccccc         ", lower: "         ccccccccccccc    ccccccccccccc         "},
			{upper: "    cccc   ccccccccc   aa   ccccccccc   cccc    ", lower: "   ccccccc   ccccc   aaaaaa   ccccc   ccccccc   "},
			{upper: " ccccccccccc  cc   aaaaaaaaaa   cc  ccccccccccc ", lower: "ccccccccccccc    aaaaaaaaaaaaaa    ccccccccccccc"},
			{upper: "sccccccccccc      aaaaaaaaaaaaa     cccccccccccs", lower: "ssscccccccc   cc   aaaaaaaaaa   cc   ccccccccsss"},
			{upper: "ssssccccc   cccccc   aaaaaa   cccccc   cccccssss", lower: "ssssssc   cccccccccc   aa   cccccccccc   cssssss"},
			{upper: "sssssss  ccccccccccccc    ccccccccccccc  sssssss", lower: "sssssss sccccccccccccc    cccccccccccccs sssssss"},
			{upper: " ssssss sssccccccccc   cc   cccccccccsss sssssss", lower: "   ssss sssscccccc   cccccc   ccccccssss ssssss "},
			{upper: "c   sss ssssssccc  cccccccccc  cccssssss sssss  ", lower: "ssc  ss sssssss   cccccccccccc  csssssss sss   c"},
			{upper: "sssc  s sssssss  cccccccccccccc ssssssss ss  ccc", lower: "ssssc   sssssss  sscccccccccccs ssssssss    cccc"},
			{upper: "sssss     sssss  sssccccccccsss ssssss    cccccc", lower: "sssssc     ssss  sssssccccsssss sssss    ccccccc"},
			{upper: " sssss  aa   ss  ssssssccssssss sss      ccccccc", lower: "   sssc aaaa  s  ssssssssssssss s    aa  cccccc "},
			{upper: "aa  sss  aaaa    ssssssssssssss    aaaaa cccc   ", lower: "aaa  ssc aaaaaa  ssssssssssssss  aaaaaaa ccc   a"},
			{upper: "aaaa   c  aaaaaa   ssssssssss   aaaaaaaa c   aaa", lower: "aaaaaa    aaaaaaaa  sssssss   aaaaaaaaaa   aaaaa"},
			{upper: "aaaaaa    aaaaaaaaa   ssss   aaaaaaaaaaa aaaaaaa", lower: "aaaaaa  c  aaaaaaaaaa  ss  aaaaaaaaaaaaa aaaaaaa"},
			{upper: " aaaaa  ss   aaaaaaaaa    aaaaaaaaaaaaaa aaaaaaa", lower: "   aaa  sssc  aaaaaaaaaaaaaaaaaaaa aaaaa aaaaaa "},
			{upper: "    aa  ssssc   aaaaaaaaaaaaaaaa   aaaaa aaaa   ", lower: "        ssssssc  aaaaaaaaaaaaaa    aaaaa aa     "},
			{upper: "        sssssss   aaaaaaaaaaa   cc aaaaa a      ", lower: "         ssssss cs  aaaaaaaa   ccc aaaaa        "},
			{upper: "          sssss sss  aaaaa   ccccc aaaa         ", lower: "           ssss ssss   aa   cccccc aa           "},
			{upper: "             ss ssssss    cccccccc a            ", lower: "              s sssssssscccccccccc              "},
			{upper: "                ssssssssccccccccc               ", lower: "                 sssssssccccccc                 "},
			{upper: "                   sssssccccc                   ", lower: "                    sssscccc                    "},
			{upper: "                      sscc                      ", lower: "                       sc                       "},
		}
	}
	return nil
}

func asciiLogoRows(width int) []string {
	rows := logoRows(width)
	lines := make([]string, len(rows))
	for i, row := range rows {
		var line strings.Builder
		for column := range width {
			upper, lower := row.upper[column], row.lower[column]
			switch {
			case upper == ' ' && lower == ' ':
				line.WriteByte(' ')
			case upper == ' ':
				line.WriteByte('_')
			case lower == ' ':
				line.WriteByte('\'')
			case upper == 'a' || lower == 'a':
				line.WriteByte('@')
			case upper == 's' && lower == 's':
				line.WriteByte(':')
			default:
				line.WriteByte('#')
			}
		}
		lines[i] = line.String()
	}
	return lines
}

func (m model) renderLogo(width int) []string {
	if m.styles.asciiLogo {
		return asciiLogoRows(width)
	}
	rows := logoRows(width)
	lines := make([]string, len(rows))
	for i, row := range rows {
		var output strings.Builder
		for column := range width {
			upper, lower := row.upper[column], row.lower[column]
			switch {
			case upper == ' ' && lower == ' ':
				output.WriteByte(' ')
			case upper == lower:
				// Background fill avoids the font-dependent gaps of full-block glyphs.
				style := m.logoStyle(upper)
				output.WriteString(style.Background(style.GetForeground()).Render(" "))
			case upper == ' ':
				output.WriteString(m.logoStyle(lower).Render("▄"))
			case lower == ' ':
				output.WriteString(m.logoStyle(upper).Render("▀"))
			default:
				output.WriteString(m.logoStyle(upper).Background(m.logoStyle(lower).GetForeground()).Render("▀"))
			}
		}
		lines[i] = output.String()
	}
	return lines
}

func (m model) logoStyle(pixel byte) lipgloss.Style {
	if pixel == 'a' {
		return m.styles.logoAccent
	}
	if pixel == 's' {
		return m.styles.logoSide
	}
	return m.styles.logoFace
}

func logoSizes() []logoSize {
	return []logoSize{{48, 26}, {44, 24}, {40, 22}, {36, 20}, {32, 17}, {28, 15}, {24, 13}, {20, 11}, {18, 10}, {16, 9}}
}

func (m model) isWelcomeOverview() bool {
	return m.welcome && m.isHome() && !m.searching && !m.help && !m.loading && !m.hasError()
}

func (l welcomeLayout) hasLogo() bool { return l.logo.width > 0 }

func (m model) responsiveWelcome(width, height int) welcomeLayout {
	if !m.isWelcomeOverview() {
		return welcomeLayout{}
	}
	// Extra screen space belongs to the workspace, not a larger brand illustration.
	maxWidth := min(maximumWelcomeLogoWidth, max(minimumWelcomeLogoWidth, width/5))
	maxHeight := max(minimumWelcomeLogoHeight, (height-welcomeBottomGap)/2)
	for _, size := range logoSizes() {
		if size.width > maxWidth || size.height > maxHeight {
			continue
		}
		if size.width+welcomeColumnGap+minimumWelcomeMenuWidth <= width && size.height <= height-welcomeBottomGap {
			return welcomeLayout{logo: size, gap: welcomeColumnGap}
		}
		// Keep the title and account full-width when a narrow menu needs the space.
		if size.width+compactWelcomeColumnGap+minimumCompactMenuWidth <= width && size.height <= height-welcomeHeaderHeight-welcomeBottomGap {
			return welcomeLayout{logo: size, gap: compactWelcomeColumnGap, headerAbove: true}
		}
	}
	return welcomeLayout{}
}

func (m model) welcomeView(width, capacity int, layout welcomeLayout) []string {
	logo := m.renderLogo(layout.logo.width)
	menuWidth := width - layout.logo.width - layout.gap
	var copy []string
	if !layout.headerAbove {
		copy = m.headerView(menuWidth)[1:]
	}
	copy = append(copy, m.homeView(menuWidth, capacity-len(copy), !layout.headerAbove)...)
	lines := make([]string, max(len(logo), len(copy)))
	for i := range lines {
		left, right := "", ""
		if i < len(logo) {
			left = logo[i]
		}
		if i < len(copy) {
			right = copy[i]
		}
		lines[i] = pad(left, layout.logo.width) + strings.Repeat(" ", layout.gap) + right
	}
	return lines
}
