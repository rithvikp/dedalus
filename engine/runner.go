package engine

import (
	"crypto/sha1"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/rithvikp/dedalus/ast"
)

// TODO: validate for safe negation, auto-persisted relations

func NewRunner(p *ast.Program) (*Runner, error) {
	s, err := New(p)
	if err != nil {
		return nil, err
	}

	return &Runner{State: s}, nil
}

func (r *Runner) Step() {
	var queue []*Rule
	inQueue := map[string]struct{}{}
	queue = append(queue, r.rules...)

	for _, rl := range queue {
		inQueue[rl.id] = struct{}{}
	}

	for len(queue) != 0 {
		rl := queue[0]
		delete(inQueue, rl.id)
		queue = queue[1:]

		time := r.currentTimestamp

		var data [][]string
		for loc := range r.locations {
			ldata := join(rl, loc, time)
			if rl.hasAggregation {
				ldata = aggregate(rl, ldata)
			}

			data = append(data, ldata...)
		}

		modified := false
		for _, d := range data {
			var nextTime int
			switch rl.timeModel {
			case TimeModelSame:
				nextTime = time
			case TimeModelSuccessor:
				nextTime = time + 1
			case TimeModelAsync:
				combined := strings.Join(d, ";")
				b := big.NewInt(0)
				h := sha1.New()
				h.Write([]byte(combined))

				b.SetBytes(h.Sum(nil)[:7]) // h.Sum(nil) has a fixed size (sha1.Size).

				randSrc := rand.NewSource(b.Int64())
				nextTime = time + rand.New(randSrc).Intn(8)
			}

			tuple := d[:len(d)-1]
			nextLoc := d[len(d)-1]
			// Keep track of new locations
			r.locations[nextLoc] = struct{}{}

			fmt.Println(rl.head.id+":", tuple, nextLoc, nextTime)
			if rl.head.push(tuple, nextLoc, nextTime) {
				modified = true
			}
		}

		if modified {
			for _, bodyRule := range rl.head.bodyRules {
				if _, ok := inQueue[bodyRule.id]; !ok {
					queue = append(queue)
				}
			}
		}
	}

	// TODO: Optimize automatic persistence
	for _, rel := range r.relations {
		if !rel.autoPersist {
			continue
		}

		for loc := range r.locations {
			for _, f := range rel.all(loc, r.currentTimestamp) {
				rel.push(f.data, f.location, f.timestamp+1)
			}
		}
	}

	r.currentTimestamp++
}

func (r *Runner) PrintRelation(name string) error {
	rel, ok := r.relations[name]
	if !ok {
		return nil
	}

	tabw := new(tabwriter.Writer)
	tabw.Init(os.Stdout, 4, 8, 1, ' ', 0)

	fmt.Fprint(tabw, "Idx\t")
	for i := range rel.indexes {
		fmt.Fprintf(tabw, "A%d\t", i)
	}
	fmt.Fprintln(tabw, "Loc\tTime")

	facts := rel.allAcrossSpaceTime()
	sort.SliceStable(facts, func(i, j int) bool {
		if facts[i].timestamp == facts[j].timestamp {
			if facts[i].location <= facts[j].location {
				return true
			}
			return false
		} else if facts[i].timestamp < facts[j].timestamp {
			return true
		}
		return false
	})

	for i, f := range facts {
		fmt.Fprintf(tabw, "%d.\t", i+1)
		for _, val := range f.data {
			fmt.Fprintf(tabw, "%s\t", val)
		}
		fmt.Fprintf(tabw, "%s\t%d\n", f.location, f.timestamp)
	}
	return tabw.Flush()
}
