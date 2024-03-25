package oviewer

import (
	"context"
	"io"
	"log"
	"strings"

	"golang.org/x/sync/errgroup"
)

func (root *Root) filter(ctx context.Context) {
	searcher := root.setSearcher(root.input.value, root.Config.CaseSensitive)
	root.filterSearch(ctx, searcher)
}

func (root *Root) filterSearch(ctx context.Context, searcher Searcher) {
	if searcher == nil {
		if root.Doc.jumpTargetSection {
			root.Doc.jumpTargetNum = 0
		}
		return
	}
	word := root.searcher.String()
	root.setMessagef("filter:%v (%v)Cancel", word, strings.Join(root.cancelKeys, ","))
	eg, ctx := errgroup.WithContext(ctx)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eg.Go(func() error {
		return root.cancelWait(cancel)
	})

	m := root.Doc
	out := make(chan int)
	r, w := io.Pipe()
	defer r.Close()
	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case lineNum, ok := <-out:
				if !ok {
					return nil
				}
				line := m.LineString(lineNum)
				w.Write([]byte(line))
				w.Write([]byte("\n"))
			}
		}
	})
	eg.Go(func() error {
		nextLN := 0
		for {
			lineNum, err := m.searchLine(ctx, searcher, true, nextLN)
			if err != nil {
				log.Println("searchFilter", err)
				break
			}
			out <- lineNum
			nextLN = lineNum + 1
		}
		close(out)
		root.sendSearchQuit()
		return nil
	})

	filterDoc, err := NewDocument()
	if err != nil {
		log.Println(err)
		return
	}
	if err := filterDoc.ControlReader(r, nil); err != nil {
		log.Println(err)
		return
	}
	root.addDocument(filterDoc)

	if err := eg.Wait(); err != nil {
		root.setMessageLog(err.Error())
		return
	}
	root.setMessagef("search:%v", word)
}
