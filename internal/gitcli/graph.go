package gitcli

// Uncommitted is the synthetic graph node for a dirty working tree.
// It is not a Git object.
const Uncommitted = "uncommitted"

// Edge connects a lane on one row to a lane on the next row.
// Gray marks the segment from the uncommitted node to HEAD.
type Edge struct {
	From, To int
	Gray     bool
}

// GraphRow is one commit plus the lanes needed to draw it.
type GraphRow struct {
	Commit   Commit
	Lane     int
	Incoming []Edge
	Outgoing []Edge
}

// LayoutGraph assigns columns (lanes) for a newest-first commit list.
func LayoutGraph(commits []Commit) []GraphRow {
	if len(commits) == 0 {
		return nil
	}
	known := make(map[string]struct{}, len(commits))
	for _, c := range commits {
		known[c.Hash] = struct{}{}
	}

	active := []string{}
	rows := make([]GraphRow, len(commits))
	for i, c := range commits {
		lane := indexOf(active, c.Hash)
		if lane < 0 {
			lane = firstEmpty(active)
			if lane < 0 {
				lane = len(active)
				active = append(active, c.Hash)
			} else {
				active[lane] = c.Hash
			}
		}

		parents := make([]string, 0, len(c.Parents))
		for _, p := range c.Parents {
			if _, ok := known[p]; ok {
				parents = append(parents, p)
			}
		}

		next := append([]string(nil), active...)
		if len(parents) == 0 {
			next[lane] = ""
		} else {
			if j := indexOf(active, parents[0]); j >= 0 && j != lane {
				next[lane] = ""
			} else {
				next[lane] = parents[0]
			}
			for _, p := range parents[1:] {
				if indexOf(next, p) >= 0 {
					continue
				}
				e := firstEmpty(next)
				if e < 0 {
					next = append(next, p)
				} else {
					next[e] = p
				}
			}
		}

		var outgoing []Edge
		seen := map[Edge]bool{}
		add := func(e Edge) {
			if seen[e] {
				return
			}
			seen[e] = true
			outgoing = append(outgoing, e)
		}
		for _, p := range parents {
			if j := indexOf(next, p); j >= 0 {
				add(Edge{From: lane, To: j})
			}
		}
		for l, h := range active {
			if h == "" || l == lane {
				continue
			}
			if j := indexOf(next, h); j >= 0 {
				add(Edge{From: l, To: j})
			}
		}

		rows[i] = GraphRow{Commit: c, Lane: lane, Outgoing: outgoing}
		active = next
	}
	for i := 1; i < len(rows); i++ {
		rows[i].Incoming = rows[i-1].Outgoing
	}
	return rows
}

// WithUncommitted inserts a synthetic node on the row directly above head,
// with a gray edge into that commit. Other commits keep their own edges.
func WithUncommitted(commits []Commit, head string) []GraphRow {
	rows := LayoutGraph(commits)
	if head == "" {
		return rows
	}
	return insertUncommitted(rows, head)
}

// insertUncommitted places the uncommitted node immediately above head.
// When head is not the newest commit, the node uses a side lane so the
// commits above head stay connected to head.
func insertUncommitted(rows []GraphRow, head string) []GraphRow {
	idx := -1
	for i := range rows {
		if rows[i].Commit.Hash == head {
			idx = i
			break
		}
	}
	if idx < 0 {
		return rows
	}
	headLane := rows[idx].Lane
	if idx == 0 {
		node := GraphRow{
			Commit:   Commit{Hash: Uncommitted, Parents: []string{head}},
			Lane:     headLane,
			Outgoing: []Edge{{From: headLane, To: headLane, Gray: true}},
		}
		rows[0].Incoming = []Edge{{From: headLane, To: headLane, Gray: true}}
		return append([]GraphRow{node}, rows...)
	}

	uLane := maxLane(rows) + 1
	incoming := append([]Edge(nil), rows[idx].Incoming...)
	seen := map[[2]int]bool{}
	outgoing := make([]Edge, 0, len(incoming)+1)
	for _, e := range incoming {
		key := [2]int{e.To, e.To}
		if seen[key] {
			continue
		}
		seen[key] = true
		outgoing = append(outgoing, Edge{From: e.To, To: e.To})
	}
	outgoing = append(outgoing, Edge{From: uLane, To: headLane, Gray: true})
	node := GraphRow{
		Commit:   Commit{Hash: Uncommitted, Parents: []string{head}},
		Lane:     uLane,
		Incoming: incoming,
		Outgoing: outgoing,
	}
	rows[idx].Incoming = append([]Edge(nil), outgoing...)
	out := make([]GraphRow, 0, len(rows)+1)
	out = append(out, rows[:idx]...)
	out = append(out, node)
	out = append(out, rows[idx:]...)
	return out
}

func maxLane(rows []GraphRow) int {
	maxLane := 0
	for _, r := range rows {
		if r.Lane > maxLane {
			maxLane = r.Lane
		}
		for _, e := range r.Incoming {
			if e.From > maxLane {
				maxLane = e.From
			}
			if e.To > maxLane {
				maxLane = e.To
			}
		}
		for _, e := range r.Outgoing {
			if e.From > maxLane {
				maxLane = e.From
			}
			if e.To > maxLane {
				maxLane = e.To
			}
		}
	}
	return maxLane
}

// LaneCount is the number of columns needed to draw rows.
func LaneCount(rows []GraphRow) int {
	maxLane := 0
	for _, r := range rows {
		if r.Lane > maxLane {
			maxLane = r.Lane
		}
		for _, e := range r.Incoming {
			if e.From > maxLane {
				maxLane = e.From
			}
			if e.To > maxLane {
				maxLane = e.To
			}
		}
		for _, e := range r.Outgoing {
			if e.From > maxLane {
				maxLane = e.From
			}
			if e.To > maxLane {
				maxLane = e.To
			}
		}
	}
	return maxLane + 1
}

func indexOf(lanes []string, hash string) int {
	for i, h := range lanes {
		if h == hash {
			return i
		}
	}
	return -1
}

func firstEmpty(lanes []string) int {
	for i, h := range lanes {
		if h == "" {
			return i
		}
	}
	return -1
}
