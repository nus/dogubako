package gitcli

// Uncommitted is the synthetic graph node for a dirty working tree.
// It is not a Git object. It is drawn as the first row, with a gray
// line down to the checked-out commit.
const Uncommitted = "uncommitted"

// Edge connects a lane on one row to a lane on the next row.
// Color is the branch colour index. Gray marks the uncommitted line.
// LockTop keeps a lane change against the upper commit when a detail
// panel opens between the two rows.
type Edge struct {
	From, To int
	Color    int
	Gray     bool
	LockTop  bool
}

// GraphRow is one commit plus the lanes needed to draw it.
// Color is the branch colour of the commit dot.
type GraphRow struct {
	Commit   Commit
	Lane     int
	Color    int
	Incoming []Edge
	Outgoing []Edge
}

// LayoutGraph assigns columns for a newest-first commit list.
// A branch keeps one color, and the first parent continues that branch.
// A parent that is not in commits still reserves a column down to the
// last loaded row. head is accepted so callers can pass the checked-out
// commit; the columns themselves do not depend on it.
func LayoutGraph(commits []Commit, head string) []GraphRow {
	if len(commits) == 0 {
		return nil
	}
	return layoutGraph(commits)
}

// WithUncommitted inserts the uncommitted node as the first row, with
// head as its parent. The gray line runs down to that commit.
func WithUncommitted(commits []Commit, head string) []GraphRow {
	if head == "" || len(commits) == 0 {
		return LayoutGraph(commits, head)
	}
	all := make([]Commit, len(commits)+1)
	all[0] = Commit{Hash: Uncommitted, Parents: []string{head}}
	copy(all[1:], commits)
	return LayoutGraph(all, head)
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

const graphNull = -1

type graphVertex struct {
	id         int
	parents    []int
	nextParent int
	branch     int
	x          int
	nextX      int
	conns      []graphConn
	committed  bool
}

type graphConn struct {
	to     int
	branch int
	set    bool
}

type graphLine struct {
	x1, y1 int
	x2, y2 int
	locked bool
}

type graphBranch struct {
	color          int
	end            int
	lines          []graphLine
	numUncommitted int
}

func layoutGraph(commits []Commit) []GraphRow {
	index := make(map[string]int, len(commits))
	for i, c := range commits {
		if _, ok := index[c.Hash]; !ok {
			index[c.Hash] = i
		}
	}
	vertices := make([]graphVertex, len(commits))
	for i, c := range commits {
		parents := make([]int, len(c.Parents))
		for j, p := range c.Parents {
			if id, ok := index[p]; ok {
				parents[j] = id
			} else {
				parents[j] = graphNull
			}
		}
		vertices[i] = graphVertex{
			id:        i,
			parents:   parents,
			branch:    -1,
			committed: c.Hash != Uncommitted,
		}
	}

	var branches []graphBranch
	available := []int{}
	i := 0
	steps := 0
	limit := len(vertices)*len(vertices) + 1
	for i < len(vertices) {
		v := &vertices[i]
		if v.nextParent < len(v.parents) || v.branch < 0 {
			before := v.nextParent
			onBranch := v.branch >= 0
			determinePath(vertices, &branches, &available, i)
			steps++
			if steps > limit {
				break
			}
			if v.nextParent == before && onBranch == (v.branch >= 0) {
				// A parent above this row cannot be reached. Skip it
				// instead of laying the same path out forever.
				if v.nextParent < len(v.parents) {
					v.nextParent++
				} else if v.branch < 0 {
					v.branch = 0
				}
			}
			continue
		}
		i++
	}

	rows := make([]GraphRow, len(commits))
	for i, c := range commits {
		v := &vertices[i]
		color := 0
		if v.branch >= 0 && v.branch < len(branches) {
			color = branches[v.branch].color
		}
		rows[i] = GraphRow{Commit: c, Lane: v.x, Color: color}
	}
	for bi := range branches {
		b := &branches[bi]
		for li, line := range b.lines {
			if line.y2 != line.y1+1 || line.y1 < 0 || line.y2 >= len(rows) {
				continue
			}
			edge := Edge{
				From:    line.x1,
				To:      line.x2,
				Color:   b.color,
				Gray:    li < b.numUncommitted,
				LockTop: line.locked,
			}
			rows[line.y1].Outgoing = append(rows[line.y1].Outgoing, edge)
			rows[line.y2].Incoming = append(rows[line.y2].Incoming, edge)
		}
	}
	return rows
}

func determinePath(vertices []graphVertex, branches *[]graphBranch, available *[]int, startAt int) {
	null := &graphVertex{id: graphNull, branch: -1}
	vertex := &vertices[startAt]
	parent := vertex.parentOf(vertices, null)
	last := vertex.point()
	if vertex.branch < 0 {
		last = vertex.nextPoint()
	}

	if parent != nil && parent.id != graphNull && vertex.merge() && vertex.branch >= 0 && parent.branch >= 0 {
		found := false
		parentBranch := parent.branch
		for i := startAt + 1; i < len(vertices); i++ {
			cur := &vertices[i]
			curPoint, ok := cur.pointConnecting(parent.id, parentBranch)
			if ok {
				found = true
			} else {
				curPoint = cur.nextPoint()
			}
			locked := true
			if !found && cur.id != parent.id {
				locked = last.x < curPoint.x
			}
			(*branches)[parentBranch].addLine(last, curPoint, vertex.committed, locked)
			cur.reserve(curPoint.x, parent.id, parentBranch)
			last = curPoint
			if found {
				vertex.registerParent()
				break
			}
		}
		return
	}

	color := availableColor(available, startAt)
	branchID := len(*branches)
	*branches = append(*branches, graphBranch{color: color})
	vertex.addToBranch(branchID, last.x)
	vertex.reserve(last.x, vertex.id, branchID)
	i := startAt + 1
	for ; i < len(vertices); i++ {
		cur := &vertices[i]
		curPoint := cur.nextPoint()
		if parent != nil && cur.id == parent.id && parent.branch >= 0 {
			curPoint = parent.point()
		}
		(*branches)[branchID].addLine(last, curPoint, vertex.committed, last.x < curPoint.x)
		connect := graphNull
		if parent != nil {
			connect = parent.id
		}
		cur.reserve(curPoint.x, connect, branchID)
		last = curPoint
		if parent != nil && cur.id == parent.id {
			vertex.registerParent()
			parentOnBranch := parent.branch >= 0
			parent.addToBranch(branchID, curPoint.x)
			vertex = parent
			parent = vertex.parentOf(vertices, null)
			if parent == nil || parentOnBranch {
				break
			}
		}
	}
	if i == len(vertices) && parent != nil && parent.id == graphNull {
		vertex.registerParent()
	}
	(*branches)[branchID].end = i
	(*available)[color] = i
}

// parentOf is the next parent still to be drawn. A parent that is not in
// the loaded commits is the shared null vertex.
func (v *graphVertex) parentOf(vertices []graphVertex, null *graphVertex) *graphVertex {
	if v.nextParent >= len(v.parents) {
		return nil
	}
	id := v.parents[v.nextParent]
	if id == graphNull {
		return null
	}
	return &vertices[id]
}

func (v *graphVertex) merge() bool { return len(v.parents) > 1 }

func (v *graphVertex) point() graphPoint { return graphPoint{v.x, v.id} }

func (v *graphVertex) nextPoint() graphPoint { return graphPoint{v.nextX, v.id} }

func (v *graphVertex) pointConnecting(to, branch int) (graphPoint, bool) {
	for x, c := range v.conns {
		if c.set && c.to == to && c.branch == branch {
			return graphPoint{x, v.id}, true
		}
	}
	return graphPoint{}, false
}

func (v *graphVertex) reserve(x, to, branch int) {
	if x != v.nextX {
		return
	}
	if x >= len(v.conns) {
		conns := make([]graphConn, x+1)
		copy(conns, v.conns)
		v.conns = conns
	}
	v.conns[x] = graphConn{to: to, branch: branch, set: true}
	v.nextX = x + 1
}

func (v *graphVertex) addToBranch(branch, x int) {
	if v.branch < 0 {
		v.branch = branch
		v.x = x
	}
}

func (v *graphVertex) registerParent() { v.nextParent++ }

type graphPoint struct{ x, y int }

func (b *graphBranch) addLine(p1, p2 graphPoint, committed, locked bool) {
	b.lines = append(b.lines, graphLine{p1.x, p1.y, p2.x, p2.y, locked})
	if committed {
		if p2.x == 0 && p2.y < b.numUncommitted {
			b.numUncommitted = p2.y
		}
		return
	}
	b.numUncommitted++
}

func availableColor(available *[]int, startAt int) int {
	for i, end := range *available {
		if startAt > end {
			return i
		}
	}
	*available = append(*available, 0)
	return len(*available) - 1
}
