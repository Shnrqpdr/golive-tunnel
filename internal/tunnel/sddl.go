package tunnel

import (
	"regexp"
	"strings"
)

var sidPattern = regexp.MustCompile(`^S-1-[0-9]+(-[0-9]+)+$`)

// grantStartStop gives one user the right to query, start and stop (LC, RP, WP) the tunnel
// service only, so the program does not need administrator on every run. The entry goes in the
// DACL, before the audit part, and is not repeated when the setup runs again.
func grantStartStop(sddl, sid string) string {
	// Windows returns the rights in its own order (LCRPWP), not in the order they were written,
	// so an existing entry is recognized by its rights and not by its text.
	existing := regexp.MustCompile(`\(A;;([A-Z]+);;;` + regexp.QuoteMeta(sid) + `\)`)
	for _, m := range existing.FindAllStringSubmatch(sddl, -1) {
		rights := map[string]bool{}
		for i := 0; i+1 < len(m[1]); i += 2 {
			rights[m[1][i:i+2]] = true
		}
		if rights["LC"] && rights["RP"] && rights["WP"] {
			return sddl
		}
	}

	ace := "(A;;LCRPWP;;;" + sid + ")"
	if i := strings.Index(sddl, "S:"); i >= 0 {
		return sddl[:i] + ace + sddl[i:]
	}
	return sddl + ace
}
