// Package git reads a repository's branch, changes and last commit.
package git

import (
	"bufio"
	"bytes"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Info is a repository snapshot.
type Info struct {
	Repo      bool
	Branch    string
	Changed   int // files with any change
	Staged    int
	Modified  int
	Untracked int
	Conflicts int
	Ahead     int
	Behind    int
	Subject   string    // last commit subject
	When      time.Time // last commit time
}

// Clean reports whether the working tree has no changes.
func (i Info) Clean() bool { return i.Changed == 0 }

// Status inspects dir. A directory outside a repository yields Info{Repo: false}.
func Status(dir string) Info {
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain=v2", "--branch").Output()
	if err != nil {
		return Info{}
	}
	info := parse(out)
	if log, err := exec.Command("git", "-C", dir, "log", "-1", "--format=%ct%x00%s").Output(); err == nil {
		info.When, info.Subject = parseLog(log)
	}
	return info
}

func parse(out []byte) Info {
	info := Info{Repo: true}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			info.Branch = strings.TrimPrefix(line, "# branch.head ")
		case strings.HasPrefix(line, "# branch.ab "):
			fields := strings.Fields(strings.TrimPrefix(line, "# branch.ab "))
			if len(fields) == 2 {
				info.Ahead, _ = strconv.Atoi(strings.TrimPrefix(fields[0], "+"))
				info.Behind, _ = strconv.Atoi(strings.TrimPrefix(fields[1], "-"))
			}
		case strings.HasPrefix(line, "#") || line == "":
		case line[0] == '?':
			info.Untracked++
			info.Changed++
		case line[0] == 'u':
			info.Conflicts++
			info.Changed++
		default: // "1 XY ..." ordinary change, "2 XY ..." rename or copy
			info.Changed++
			if fields := strings.Fields(line); len(fields) > 1 && len(fields[1]) == 2 {
				if fields[1][0] != '.' {
					info.Staged++
				}
				if fields[1][1] != '.' {
					info.Modified++
				}
			}
		}
	}
	return info
}

func parseLog(out []byte) (time.Time, string) {
	unix, subject, ok := strings.Cut(strings.TrimSpace(string(out)), "\x00")
	if !ok {
		return time.Time{}, ""
	}
	sec, err := strconv.ParseInt(unix, 10, 64)
	if err != nil {
		return time.Time{}, subject
	}
	return time.Unix(sec, 0), subject
}
