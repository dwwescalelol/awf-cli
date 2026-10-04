package render

import (
	"fmt"
	"html/template"
	"slices"
	"strings"

	"github.com/dwwescalelol/awf-cli/internal/awf"
)

// Graph geometry, in SVG user units.
const (
	nodeH     = 38
	minNodeW  = 128
	charW     = 7.8
	gapX      = 24
	gapY      = 64
	pad       = 20
	startGap  = 34
	laneGap   = 22
	labelChar = 7.0
	corner    = 8
	loopR     = 10
	loopLead  = 10
	loopInset = 24
)

type node struct {
	state     *awf.State
	rank, pos int
	pre       int
	x, y, w   float64
	unreached bool
}

func (n *node) cx() float64     { return n.x + n.w/2 }
func (n *node) bottom() float64 { return n.y + nodeH }
func (n *node) midY() float64   { return n.y + nodeH/2 }
func (n *node) right() float64  { return n.x + n.w }

type arc struct {
	from, to *node
	edge     awf.Edge
	back     bool
}

// layered places states in ranks by longest path from the start, with back
// edges removed, so the flow reads top to bottom. Edges between adjacent ranks
// run straight down; every other edge routes through a lane on the right.
type layered struct {
	nodes []*node
	byKey map[*awf.State]*node
	arcs  []arc
	ranks [][]*node
}

func layout(w *awf.Workflow) *layered {
	l := &layered{byKey: make(map[*awf.State]*node, len(w.States))}
	for _, s := range w.States {
		n := &node{state: s, pre: -1, w: max(minNodeW, float64(len(s.Task.Name))*charW+32)}
		l.nodes = append(l.nodes, n)
		l.byKey[s] = n
	}

	unreachable := make(map[*awf.State]bool)
	for _, s := range w.Unreachable() {
		unreachable[s] = true
	}

	// Depth-first from the start, then from any state it never entered, marks
	// back edges and yields a topological order of the rest.
	var order []*node
	onStack := make(map[*node]bool)
	counter := 0
	var visit func(n *node)
	visit = func(n *node) {
		n.pre = counter
		counter++
		onStack[n] = true
		for _, e := range n.state.Out {
			to := l.byKey[e.To]
			back := onStack[to]
			l.arcs = append(l.arcs, arc{from: n, to: to, edge: e, back: back})
			if to.pre < 0 {
				visit(to)
			}
		}
		onStack[n] = false
		order = append(order, n)
	}
	if w.Start != nil {
		visit(l.byKey[w.Start])
	}
	for _, n := range l.nodes {
		if n.pre < 0 {
			visit(n)
		}
		n.unreached = unreachable[n.state]
	}
	slices.Reverse(order)

	for _, n := range order {
		for _, a := range l.arcs {
			if a.from == n && !a.back {
				a.to.rank = max(a.to.rank, n.rank+1)
			}
		}
	}

	depth := 0
	for _, n := range l.nodes {
		depth = max(depth, n.rank+1)
	}
	l.ranks = make([][]*node, depth)
	for _, n := range l.nodes {
		l.ranks[n.rank] = append(l.ranks[n.rank], n)
	}
	for r, rank := range l.ranks {
		l.orderRank(r, rank)
	}
	l.place()
	return l
}

// orderRank sorts a rank by the mean position of its parents in the rank
// above, which keeps forward edges from crossing where it can.
func (l *layered) orderRank(r int, rank []*node) {
	weight := make(map[*node]float64, len(rank))
	for _, n := range rank {
		var sum, count float64
		for _, a := range l.arcs {
			if a.to == n && !a.back && a.from.rank == r-1 {
				sum += float64(a.from.pos)
				count++
			}
		}
		weight[n] = float64(n.pre)
		if count > 0 {
			weight[n] = sum/count*1000 + float64(n.pre)
		}
	}
	slices.SortStableFunc(rank, func(a, b *node) int {
		return compareFloat(weight[a], weight[b])
	})
	for i, n := range rank {
		n.pos = i
	}
}

func compareFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func (l *layered) place() {
	widths := make([]float64, len(l.ranks))
	core := 0.0
	for r, rank := range l.ranks {
		for i, n := range rank {
			if i > 0 {
				widths[r] += gapX
			}
			widths[r] += n.w
		}
		core = max(core, widths[r])
	}
	for r, rank := range l.ranks {
		x := pad + (core-widths[r])/2
		for _, n := range rank {
			n.x = x
			n.y = pad + startGap + float64(r)*(nodeH+gapY)
			x += n.w + gapX
		}
	}
}

// lanes assigns each routed arc a vertical lane right of the nodes. Arcs whose
// vertical spans do not overlap share a lane; shorter spans sit closer in.
func (l *layered) lanes(routed []int) map[int]int {
	slices.SortStableFunc(routed, func(a, b int) int {
		return compareFloat(l.span(a), l.span(b))
	})
	var taken [][][2]float64
	out := make(map[int]int, len(routed))
	for _, i := range routed {
		lo, hi := l.bounds(i)
		lane := 0
		for ; lane < len(taken); lane++ {
			free := true
			for _, iv := range taken[lane] {
				if lo <= iv[1]+nodeH/2 && iv[0] <= hi+nodeH/2 {
					free = false
					break
				}
			}
			if free {
				break
			}
		}
		if lane == len(taken) {
			taken = append(taken, nil)
		}
		taken[lane] = append(taken[lane], [2]float64{lo, hi})
		out[i] = lane
	}
	return out
}

func (l *layered) bounds(i int) (float64, float64) {
	a, b := l.arcs[i].from.midY(), l.arcs[i].to.midY()
	return min(a, b), max(a, b)
}

func (l *layered) span(i int) float64 {
	lo, hi := l.bounds(i)
	return hi - lo
}

// SVG draws the workflow's state machine. The start is marked by an entry
// arrow and each terminal state by a double outline, as in an automaton
// diagram. Each node links to its task's section of the page.
func SVG(w *awf.Workflow) template.HTML {
	l := layout(w)

	right := 0.0
	bottom := 0.0
	for _, n := range l.nodes {
		right = max(right, n.right())
		bottom = max(bottom, n.bottom())
	}

	// Self-loops draw locally. Other edges that skip or climb ranks route
	// through lanes: on the left when both ends are leftmost in their ranks,
	// otherwise on the right, so they pass no node on the way.
	var leftRouted, rightRouted []int
	for i, a := range l.arcs {
		switch {
		case a.from == a.to, !a.back && a.to.rank == a.from.rank+1:
		case !l.rightmost(a.from) || !l.rightmost(a.to):
			if a.from.pos == 0 && a.to.pos == 0 {
				leftRouted = append(leftRouted, i)
				continue
			}
			fallthrough
		default:
			rightRouted = append(rightRouted, i)
		}
	}
	leftLanes, rightLanes := l.lanes(leftRouted), l.lanes(rightRouted)
	leftEdge := right
	for _, n := range l.nodes {
		leftEdge = min(leftEdge, n.x)
	}

	// Labels can reach past the lanes, so the bounds are known once they are drawn.
	lo, hi := leftEdge-float64(len(leftLanes))*laneGap, right+float64(len(rightLanes))*laneGap
	grow := func(l, h float64) { lo, hi = min(lo, l), max(hi, h) }

	g := graph{Name: w.Name}
	if w.Start != nil {
		s := l.byKey[w.Start]
		g.Entry = &entry{X: s.cx(), Y: pad, Top: pad + 5, End: s.y - 1}
	}

	routed := func(i int) bool {
		_, left := leftLanes[i]
		_, onRight := rightLanes[i]
		return left || onRight || l.arcs[i].from == l.arcs[i].to
	}
	fanOut := make(map[*node][]int)
	fanIn := make(map[*node][]int)
	for i, a := range l.arcs {
		if !routed(i) {
			fanOut[a.from] = append(fanOut[a.from], i)
			fanIn[a.to] = append(fanIn[a.to], i)
		}
	}

	path := func(a arc, format string, args ...any) {
		g.Edges = append(g.Edges, edgePath{From: a.from.state.Task.Name, To: a.to.state.Task.Name, D: fmt.Sprintf(format, args...)})
	}
	for i, a := range l.arcs {
		if a.from == a.to {
			// A circle on the top edge near the left corner, clear of the
			// edges that enter at the top centre and the lanes at the sides.
			x, y := a.from.x+loopInset, a.from.y
			top := y - loopLead
			path(a, "M%.1f,%.1f V%.1f A%d,%d 0 0 0 %.1f,%.1f V%.1f", x+loopR, y, top, loopR, loopR, x-loopR, top, y-1)
			grow(g.label(a.edge, x-loopR-6, top-loopR/2, true))
			continue
		}

		lane, left := leftLanes[i]
		rightLane, onRight := rightLanes[i]
		if left || onRight {
			if onRight {
				lane = rightLane
			}
			y1, y2 := a.from.midY()-5, a.to.midY()+5
			if a.back {
				y1, y2 = a.from.midY()+5, a.to.midY()-5
			}
			dir := 1.0
			if y2 < y1 {
				dir = -1
			}
			x, x1, x2, side := right+laneGap*float64(lane+1), a.from.right(), a.to.right()+1, 1.0
			if left {
				x, x1, x2, side = leftEdge-laneGap*float64(lane+1), a.from.x, a.to.x-1, -1
			}
			path(a, "M%.1f,%.1f H%.1f Q%.1f,%.1f %.1f,%.1f V%.1f Q%.1f,%.1f %.1f,%.1f H%.1f",
				x1, y1, x-corner*side, x, y1, x, y1+corner*dir, y2-corner*dir, x, y2, x-corner*side, y2, x2)
			// Beside the lane where it leaves its source, so labels of lanes
			// that run side by side do not meet.
			grow(g.label(a.edge, x+6*side, y1+dir*(nodeH/2+10), left))
			continue
		}

		out := fanOut[a.from]
		sx := a.from.cx() + spread(slices.Index(out, i), len(out), a.from.w)
		in := fanIn[a.to]
		tx := a.to.cx() + spread(slices.Index(in, i), len(in), a.to.w)
		sy, ty := a.from.bottom(), a.to.y-1
		mid := (sy + ty) / 2
		path(a, "M%.1f,%.1f C%.1f,%.1f %.1f,%.1f %.1f,%.1f", sx, sy, sx, mid, tx, mid, tx, ty)
		grow(g.label(a.edge, (sx+tx)/2+7, mid, false))
	}

	for _, n := range l.nodes {
		cls := "node"
		if n.state.IsTerminal() {
			cls += " terminal"
		}
		if n.unreached {
			cls += " unreached"
		}
		if n.state == w.Start {
			cls += " start"
		}
		g.Nodes = append(g.Nodes, box{
			Name: n.state.Task.Name, Class: cls, Terminal: n.state.IsTerminal(),
			X: n.x, Y: n.y, W: n.w, H: nodeH,
			InnerX: n.x + 3.5, InnerY: n.y + 3.5, InnerW: n.w - 7, InnerH: nodeH - 7,
			CX: n.cx(), TextY: n.midY() + 4.5,
		})
	}

	g.MinX = min(0, lo-pad)
	g.Width, g.Height = hi+pad-g.MinX, bottom+pad
	var b strings.Builder
	if err := pages.ExecuteTemplate(&b, "graph", g); err != nil {
		panic(err)
	}
	return template.HTML(b.String())
}

type graph struct {
	MinX, Width, Height float64
	Name                string
	Entry               *entry
	Edges               []edgePath
	Nodes               []box
	Labels              []edgeLabel
}

type entry struct{ X, Y, Top, End float64 }

type edgePath struct{ From, To, D string }

type box struct {
	Name, Class            string
	Terminal               bool
	X, Y, W                float64
	H                      int
	InnerX, InnerY, InnerW float64
	InnerH                 int
	CX, TextY              float64
}

type edgeLabel struct {
	X, Y, DY           float64
	Anchor, On, Policy string
}

func (l *layered) rightmost(n *node) bool {
	return n.pos == len(l.ranks[n.rank])-1
}

// spread offsets the i-th of n edges sharing a node side so they fan out.
func spread(i, n int, width float64) float64 {
	if n < 2 {
		return 0
	}
	step := min(18, (width-24)/float64(n-1))
	return (float64(i) - float64(n-1)/2) * step
}

// label draws an edge's outcome and policy at x, y, ending there when end is
// set, and returns the horizontal bounds of the text.
func (g *graph) label(e awf.Edge, x, y float64, end bool) (float64, float64) {
	var policy []string
	if e.Retries != nil {
		policy = append(policy, fmt.Sprintf("%d retries", *e.Retries))
	}
	if e.Session == awf.Resume {
		policy = append(policy, "resume")
	}
	if e.On == "" && len(policy) == 0 {
		return x, x
	}
	lines := 0
	if e.On != "" {
		lines++
	}
	if len(policy) > 0 {
		lines++
	}
	l := edgeLabel{X: x, Y: y - float64(lines-1)*7 + 4, Anchor: "start", On: string(e.On), Policy: strings.Join(policy, ", ")}
	if end {
		l.Anchor = "end"
	}
	if e.On != "" {
		l.DY = 14
	}
	g.Labels = append(g.Labels, l)
	width := float64(max(len(l.On), len(l.Policy))) * labelChar
	if end {
		return x - width, x
	}
	return x, x + width
}
