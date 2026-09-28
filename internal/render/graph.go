package render

import (
	"fmt"
	"html"
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

	var b strings.Builder
	b.WriteString(`<defs><marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0,1 L9,5 L0,9 z" class="arrowhead"/></marker></defs>`)

	if w.Start != nil {
		s := l.byKey[w.Start]
		fmt.Fprintf(&b, `<circle class="entry" cx="%.1f" cy="%.1f" r="5"/>`, s.cx(), float64(pad))
		fmt.Fprintf(&b, `<path class="edge" d="M%.1f,%.1f V%.1f" marker-end="url(#arrow)"/>`, s.cx(), float64(pad)+5, s.y-1)
	}

	var labels strings.Builder
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

	for i, a := range l.arcs {
		cls := edgeClass(a)
		from, to := esc(a.from.state.Task.Name), esc(a.to.state.Task.Name)

		if a.from == a.to {
			// The loop sits on whichever side of the node has no neighbour.
			x, y, side := a.from.right(), a.from.y, 1.0
			if a.from.pos == 0 && !l.rightmost(a.from) {
				x, side = a.from.x, -1
			}
			fmt.Fprintf(&b, `<path class="%s" data-from="%s" data-to="%s" d="M%.1f,%.1f C%.1f,%.1f %.1f,%.1f %.1f,%.1f" marker-end="url(#arrow)"/>`,
				cls, from, to, x-22*side, y, x-22*side, y-30, x+30*side, y+nodeH/2, x+side, y+nodeH/2)
			grow(label(&labels, a.edge, x+12*side, y-12, side < 0))
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
			fmt.Fprintf(&b, `<path class="%s" data-from="%s" data-to="%s" d="M%.1f,%.1f H%.1f Q%.1f,%.1f %.1f,%.1f V%.1f Q%.1f,%.1f %.1f,%.1f H%.1f" marker-end="url(#arrow)"/>`,
				cls, from, to, x1, y1, x-corner*side, x, y1, x, y1+corner*dir, y2-corner*dir, x, y2, x-corner*side, y2, x2)
			// Beside the lane where it leaves its source, so labels of lanes
			// that run side by side do not meet.
			grow(label(&labels, a.edge, x+6*side, y1+dir*(nodeH/2+10), left))
			continue
		}

		out := fanOut[a.from]
		sx := a.from.cx() + spread(slices.Index(out, i), len(out), a.from.w)
		in := fanIn[a.to]
		tx := a.to.cx() + spread(slices.Index(in, i), len(in), a.to.w)
		sy, ty := a.from.bottom(), a.to.y-1
		mid := (sy + ty) / 2
		fmt.Fprintf(&b, `<path class="%s" data-from="%s" data-to="%s" d="M%.1f,%.1f C%.1f,%.1f %.1f,%.1f %.1f,%.1f" marker-end="url(#arrow)"/>`,
			cls, from, to, sx, sy, sx, mid, tx, mid, tx, ty)
		grow(label(&labels, a.edge, (sx+tx)/2+7, mid, false))
	}

	for _, n := range l.nodes {
		name := esc(n.state.Task.Name)
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
		fmt.Fprintf(&b, `<a class="%s" href="#task-%s" data-task="%s"><title>%s</title>`, cls, name, name, name)
		fmt.Fprintf(&b, `<rect class="box" x="%.1f" y="%.1f" width="%.1f" height="%d" rx="7"/>`, n.x, n.y, n.w, nodeH)
		if n.state.IsTerminal() {
			fmt.Fprintf(&b, `<rect class="inner" x="%.1f" y="%.1f" width="%.1f" height="%d" rx="4"/>`, n.x+3.5, n.y+3.5, n.w-7, nodeH-7)
		}
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f">%s</text></a>`, n.cx(), n.midY()+4.5, name)
	}

	b.WriteString(labels.String())
	b.WriteString(`</svg>`)

	minX := min(0, lo-pad)
	width, height := hi+pad-minX, bottom+pad
	head := fmt.Sprintf(`<svg class="fsm" viewBox="%.0f 0 %.0f %.0f" width="%.0f" height="%.0f" role="img" aria-label="State machine of %s">`,
		minX, width, height, width, height, esc(w.Name))
	return template.HTML(head + b.String())
}

func (l *layered) rightmost(n *node) bool {
	return n.pos == len(l.ranks[n.rank])-1
}

func edgeClass(a arc) string {
	cls := "edge"
	if a.back {
		cls += " back"
	}
	return cls
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
func label(b *strings.Builder, e awf.Edge, x, y float64, end bool) (float64, float64) {
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
	top := y - float64(lines-1)*7 + 4
	anchor := "start"
	if end {
		anchor = "end"
	}
	fmt.Fprintf(b, `<text class="label" x="%.1f" y="%.1f" text-anchor="%s">`, x, top, anchor)
	dy := 0.0
	if e.On != "" {
		fmt.Fprintf(b, `<tspan class="outcome" x="%.1f">%s</tspan>`, x, esc(string(e.On)))
		dy = 14
	}
	if len(policy) > 0 {
		fmt.Fprintf(b, `<tspan class="policy" x="%.1f" dy="%.0f">%s</tspan>`, x, dy, esc(strings.Join(policy, ", ")))
	}
	b.WriteString(`</text>`)
	width := float64(max(len(e.On), len(strings.Join(policy, ", ")))) * labelChar
	if end {
		return x - width, x
	}
	return x, x + width
}

func esc(s string) string { return html.EscapeString(s) }
